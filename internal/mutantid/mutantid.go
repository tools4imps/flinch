// Package mutantid names mutants in a form people can read and type. An id leaves out line numbers, so
// edits elsewhere in a file leave it alone.
//
//	internal/gitdiff.Hunks: i >= 0 -> i > 0
//	internal/check.Keep: return nil, err -> return nil, nil #2
package mutantid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// An ID names one mutant.
type ID struct {
	Dir         string // the package directory, slash-separated and relative to the module root; "." for the root
	Unit        string // see package units for how units are named
	Original    string // the source the mutant replaces, whitespace collapsed
	Replacement string // what replaces it, whitespace collapsed
	N           int    // 1 for the first identical change in the unit, 2 for the second, and so on
}

func (id ID) String() string {
	s := Prefix(id.Dir, id.Unit) + id.Original + " -> " + id.Replacement
	if id.N >= 2 {
		s += " #" + strconv.Itoa(id.N)
	}
	return s
}

// Hash is the first 12 hex digits of the id's SHA-256. It's what people type in a shell, where the
// id's ">", "#" and "*" all mean something.
func (id ID) Hash() string { return Hash(id.String()) }

// Hash is the first 12 hex digits of the SHA-256 of s.
func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

// Prefix is the part of an id that names the unit, colon and space included.
func Prefix(dir, unit string) string { return dir + "." + unit + ": " }

// Collapse turns each run of whitespace into one space and trims both ends.
func Collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// A Pattern is one line of a declaration block: an id, or a whole unit's wildcard.
type Pattern struct {
	Dir      string
	Unit     string
	Wildcard bool // "dir.unit: *"
	ID       ID   // set when Wildcard is false
}

var trailingN = regexp.MustCompile(` #([0-9]+)$`)

// Parse reads a declaration line. Whitespace in the source parts is collapsed the way ids collapse it.
func Parse(line string) (Pattern, error) {
	line = strings.TrimSpace(line)
	colon := strings.Index(line, ": ")
	if colon < 0 {
		return Pattern{}, errors.New(`a mutant id looks like "dir.Unit: original -> replacement"`)
	}
	dir, unit, err := splitUnit(line[:colon])
	if err != nil {
		return Pattern{}, err
	}
	rest := strings.TrimSpace(line[colon+2:])
	if rest == "*" {
		return Pattern{Dir: dir, Unit: unit, Wildcard: true}, nil
	}
	arrow := strings.Index(rest, " -> ")
	if arrow < 0 {
		return Pattern{}, errors.New(`a mutant id needs " -> " between the original and the replacement`)
	}
	id := ID{Dir: dir, Unit: unit, Original: Collapse(rest[:arrow]), N: 1}
	repl := rest[arrow+4:]
	if m := trailingN.FindStringSubmatch(repl); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n < 2 {
			return Pattern{}, errors.New("an occurrence number starts at #2; the first one has none")
		}
		id.N = n
		repl = repl[:len(repl)-len(m[0])]
	}
	id.Replacement = Collapse(repl)
	if id.Original == "" {
		return Pattern{}, errors.New("a mutant id needs the original source before \" -> \"")
	}
	return Pattern{Dir: dir, Unit: unit, ID: id}, nil
}

// splitUnit splits "internal/check.judge.settled" into "internal/check" and "judge.settled". The unit
// starts at the first dot after the last slash, so a directory name can't hold a dot. The module's
// root package is written with an empty directory: "..Name".
func splitUnit(s string) (dir, unit string, err error) {
	if strings.HasPrefix(s, "..") {
		dir, unit = ".", s[2:]
	} else {
		slash := strings.LastIndex(s, "/")
		dot := strings.Index(s[slash+1:], ".")
		if dot < 0 {
			return "", "", errors.New(`a mutant id names its unit after the package directory, as in "internal/skip.Match"`)
		}
		dir, unit = s[:slash+1+dot], s[slash+1+dot+1:]
	}
	if dir == "" || unit == "" {
		return "", "", errors.New(`a mutant id names its unit after the package directory, as in "internal/skip.Match"`)
	}
	return dir, unit, nil
}
