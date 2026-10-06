// Package cli is flinch's command line. cmd/flinch is a thin wrapper around Main, so the cli
// primitive's Contract tests can run it in-process.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"

	"github.com/tools4imps/flinch/internal/engine"
	"github.com/tools4imps/flinch/internal/report"
	"github.com/tools4imps/flinch/internal/version"
)

const usage = `flinch holds Go code to its Contract by mutation.

Usage:
  flinch [run] [flags]   build every mutant of covered code and run the Contract suite against it
  flinch contract        check the Contract's own files, without building anything
  flinch operators       list the mutation operators
  flinch version         print the version

Flags for run and contract:
  --contract DIR             where the Contract lives (default "contract")
  --tags LIST                build tags for every go command flinch runs
  --format text|json         report format (default text)
  --output FILE              write the report to FILE instead of stdout

Flags for run:
  --since REF                mutate only the lines changed since the merge base with REF
  --only PRIMITIVE           mutate one primitive's covered code; repeatable
  --operators LIST           run only these operators
  --jobs N                   parallel workers (default: the number of CPUs)
  --timeout-coefficient N    multiplies the clean run time in each time budget (default 10)
  --dry-run                  list the mutants a run would build, and build none

Exit status: 0 when the Contract holds, 1 when it doesn't, 2 when flinch couldn't decide.
`

// operators describes each default operator for "flinch operators".
var operators = [][2]string{
	{"erase", "the function returns zero values from its first line; runs before the function's other mutants"},
	{"arithmetic", "+ and - swap, * and / swap, % becomes *; numeric operands only"},
	{"boundary", "< and <= swap, > and >= swap"},
	{"equality", "== and != swap"},
	{"logical", "&& and || swap"},
	{"negation", "a ! is dropped"},
	{"bool", "true and false swap"},
	{"step", "++ and -- swap, += and -= swap"},
	{"drop-error", "in a return, an expression of type error becomes nil"},
	{"drop-call", "a call whose results are unused is removed; calls into log and log/slog are left alone"},
}

// listFlag collects a flag that may repeat and may hold a comma-separated list.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// Main runs flinch with the given arguments and returns the process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "version":
		fmt.Fprintf(stdout, "flinch %s\n", version.Version)
		return 0
	case "operators":
		for _, op := range operators {
			fmt.Fprintf(stdout, "%-11s %s\n", op[0], op[1])
		}
		return 0
	case "run", "contract":
	case "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "flinch: %s is not a command\n\n%s", cmd, usage)
		return 2
	}

	fs := flag.NewFlagSet("flinch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		contractDir = fs.String("contract", "contract", "")
		format      = fs.String("format", "text", "")
		output      = fs.String("output", "", "")
		sinceRef    = fs.String("since", "", "")
		jobs        = fs.Int("jobs", runtime.NumCPU(), "")
		coefficient = fs.Float64("timeout-coefficient", 10, "")
		dryRun      = fs.Bool("dry-run", false, "")
		showVersion = fs.Bool("version", false, "")
		tags, only  listFlag
		ops         listFlag
	)
	fs.Var(&tags, "tags", "")
	fs.Var(&only, "only", "")
	fs.Var(&ops, "operators", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return bad(stderr, err.Error())
	}
	if *showVersion {
		fmt.Fprintf(stdout, "flinch %s\n", version.Version)
		return 0
	}
	switch {
	case fs.NArg() > 0:
		return bad(stderr, fmt.Sprintf("flinch %s takes no arguments, and got %q", cmd, fs.Arg(0)))
	case *format != "text" && *format != "json":
		return bad(stderr, fmt.Sprintf("--format must be text or json, not %q", *format))
	case *jobs < 1:
		return bad(stderr, "--jobs must be at least 1")
	case *coefficient <= 0:
		return bad(stderr, "--timeout-coefficient must be more than 0")
	}
	if cmd == "contract" {
		var runOnly []string
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "since", "only", "operators", "jobs", "timeout-coefficient", "dry-run":
				runOnly = append(runOnly, "--"+f.Name)
			}
		})
		if len(runOnly) > 0 {
			return bad(stderr, fmt.Sprintf("%s only applies to flinch run", strings.Join(runOnly, ", ")))
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return fail(stderr, err)
	}
	base := engine.Options{Dir: wd, Contract: *contractDir, Tags: tags}
	out := stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return fail(stderr, err)
		}
		defer f.Close()
		out = f
	}

	if cmd == "contract" {
		st, err := engine.Check(base)
		if err != nil {
			return fail(stderr, err)
		}
		rep := &report.Report{Version: version.Version, Tags: tags, Problems: st.Problems, Scoped: true, Contract: *contractDir}
		if st.Ownership != nil {
			rep.Outside = st.Ownership.Outside()
		}
		if len(st.Problems) > 0 {
			rep.Exit = 1
		}
		if *format == "json" {
			return write(out, stderr, *format, rep)
		}
		return contractText(out, st, rep)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ro := engine.RunOptions{
		Options: base, Since: *sinceRef, Only: only, Operators: ops,
		Jobs: *jobs, Coefficient: *coefficient, Progress: stderr,
	}
	if *dryRun {
		plan, err := engine.Prepare(ctx, ro)
		if err != nil {
			return fail(stderr, err)
		}
		if len(plan.Static.Problems) > 0 {
			rep := &report.Report{Version: version.Version, Tags: tags, Problems: plan.Static.Problems, Exit: 1, Contract: *contractDir}
			return write(out, stderr, *format, rep)
		}
		for _, m := range plan.Mutants {
			fmt.Fprintf(out, "%s:%d  %s  %s\n", m.File, m.Line, m.Hash, m.ID)
		}
		fmt.Fprintf(out, "%d mutants, none built\n", len(plan.Mutants))
		return 0
	}
	rep, err := engine.Run(ctx, ro)
	if err != nil {
		return fail(stderr, err)
	}
	return write(out, stderr, *format, rep)
}

func write(out, stderr io.Writer, format string, rep *report.Report) int {
	var err error
	if format == "json" {
		err = report.JSON(out, rep)
	} else {
		err = report.Text(out, rep)
	}
	if err != nil {
		return fail(stderr, err)
	}
	return rep.Exit
}

// contractText is the text report for "flinch contract": how big the Contract is, its errors, and
// the packages it doesn't cover. A full run's report has much more to say, so this one stands alone.
func contractText(out io.Writer, st *engine.Static, rep *report.Report) int {
	obligations := 0
	for _, prim := range st.Contract.Primitives {
		obligations += len(prim.Obligations)
	}
	fmt.Fprintf(out, "flinch contract: %s, %s, %s\n", count(len(st.Contract.Primitives), "primitive"),
		count(obligations, "obligation"), count(len(st.Tests), "Contract test"))
	if len(rep.Problems) == 0 {
		fmt.Fprintln(out, "no Contract errors")
	} else {
		fmt.Fprintf(out, "\nContract errors (%d)\n", len(rep.Problems))
		for _, p := range rep.Problems {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
	if len(rep.Outside) > 0 {
		fmt.Fprintf(out, "\nOutside the Contract (%d), never failing the build; name one in a covers block to bring it in\n", len(rep.Outside))
		for _, dir := range rep.Outside {
			fmt.Fprintf(out, "  %s\n", dir)
		}
	}
	fmt.Fprintf(out, "\nexit %d\n", rep.Exit)
	return rep.Exit
}

// count writes n with a singular or plural noun.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// bad reports a mistake on the command line. It never reaches a verdict.
func bad(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "flinch: %s\n\n%s", msg, usage)
	return 2
}

// fail reports that flinch couldn't run at all.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "flinch: %v\n", err)
	return 2
}
