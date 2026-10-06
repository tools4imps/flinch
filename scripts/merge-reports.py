#!/usr/bin/env python3
"""Merges the JSON reports of a sharded full run into one verdict.

Each shard mutated one primitive, or some of its operators, so on its own it can't tell whether an
obligation or a test holds anything: a test in one primitive can kill a mutant in another. This
reads every shard's report, credits each kill again across all of them, and judges every primitive
the shards mutated whole between them. A primitive split by operator never had a shard that
mutated it whole, so its declarations are checked for staleness here.

Usage: merge-reports.py PLAN SHARDS_DIR OUTPUT
PLAN is plan-shards.py's JSON in a file, or "-" to read it from $PLAN. Each shard's report is at
SHARDS_DIR/shard-NAME/report.json. The merged report goes to OUTPUT and a summary to stdout. Exits
like flinch: 0 when the Contract holds, 1 when it doesn't, 2 when the merge couldn't decide.
"""
import json
import os
import sys
from collections import defaultdict

UNHELD = {"lived", "unreached", "erased"}


def primitive_of(ref):
    return ref.rsplit("/", 1)[0]


def declaration_lines(path):
    """Returns the line numbers of the declarations in a mutants.md."""
    found, fence = [], None
    with open(path) as f:
        for n, line in enumerate(f, 1):
            text = line.strip()
            if text.startswith("```"):
                fence = text[3:].strip() if fence is None else None
            elif fence in ("equivalent", "unpromised") and text:
                found.append(n)
    return found


def main():
    plan_arg, shards_dir, output = sys.argv[1:4]
    plan = json.loads(os.environ["PLAN"] if plan_arg == "-" else open(plan_arg).read())
    every_op = set(plan["operators"])

    reports, undecided = [], []
    for s in plan["shards"]:
        path = os.path.join(shards_dir, "shard-" + s["name"], "report.json")
        try:
            with open(path) as f:
                reports.append((s, json.load(f)))
        except (OSError, ValueError) as e:
            undecided.append(f"shard {s['name']} left no report ({e.__class__.__name__})")

    ops_run, split = defaultdict(set), set()
    for s, _ in reports:
        if s["operators"]:
            ops_run[s["only"]].update(s["operators"].split(","))
            split.add(s["only"])
        else:
            ops_run[s["only"]].update(every_op)
    whole = {p for p, ops in ops_run.items() if ops >= every_op}

    contract_errors, broken, mutants, ran = {}, {}, {}, defaultdict(int)
    obligations, tests = [], []
    for s, r in reports:
        if r["undecided"]:
            undecided.append(f"shard {s['name']}: {r['undecided']}")
        for p in r["contract_errors"]:
            contract_errors[(p["path"], p["line"], p["message"])] = p
        for p in r["broken_declarations"]:
            broken[(p["path"], p["line"], p["message"])] = p
        for m in r["mutants"]:
            mutants.setdefault(m["hash"], m)
        for t in r["tests"]:
            ran[t["name"]] += t["ran"]
        if not obligations:
            obligations, tests = r["obligations"], r["tests"]

    # The declarations of a split primitive that no mutant matched.
    matched = {
        (os.path.normpath(m["declaration"]["path"]), m["declaration"]["line"])
        for m in mutants.values() if m["declaration"]
    }
    for p in sorted(split & whole):
        path = os.path.join("contract", p, "mutants.md")
        if not os.path.exists(path):
            continue
        for n in declaration_lines(path):
            if (os.path.normpath(path), n) not in matched:
                msg = "no mutant of the split run matched this declaration, so it is stale; delete the line"
                broken[(path, n, msg)] = {"path": path, "line": n, "message": msg}

    holders, holds, solid, kills = defaultdict(set), defaultdict(set), set(), defaultdict(set)
    for m in mutants.values():
        for k in m["killed_by"]:
            kills[k["test"]].add(m["hash"])
            if m["status"] != "killed":
                continue
            for o in k["obligations"]:
                holders[m["hash"]].add(o)
                holds[o].add(m["hash"])
                if k["kind"] not in ("panic", "timeout"):
                    solid.add(o)

    merged_obligations = []
    for o in obligations:
        h = sorted(holds[o["id"]])
        judged = primitive_of(o["id"]) in whole
        merged_obligations.append({
            "id": o["id"], "tests": o["tests"], "holds": h,
            "sole_holds": [x for x in h if len(holders[x]) == 1],
            "judged": judged, "hollow": judged and not h,
            "crash_only": bool(h) and o["id"] not in solid,
        })
    merged_tests = []
    for t in tests:
        k = sorted(kills[t["name"]])
        judged = primitive_of(t["name"]) in whole
        n = ran[t["name"]]
        merged_tests.append({**t, "kills": k, "ran": n, "judged": judged, "blind": judged and n > 0 and not k})

    ms = sorted(mutants.values(), key=lambda m: (m["file"], m["line"], m["col"], m["id"]))
    counts = defaultdict(int)
    for m in ms:
        counts[m["status"]] += 1
    unheld = [m for m in ms if m["status"] in UNHELD]
    no_verdict = [m for m in ms if m["status"] == "no verdict"]
    hollow = [o for o in merged_obligations if o["hollow"]]
    blind = [t for t in merged_tests if t["blind"]]
    crash_only = [o for o in merged_obligations if o["crash_only"]]
    outside_only = [m for m in ms if m["outside_only"]]
    broken_list = sorted(broken.values(), key=lambda p: (p["path"], p["line"]))
    errors = sorted(contract_errors.values(), key=lambda p: (p["path"], p["line"]))
    unjudged = sorted({primitive_of(o["id"]) for o in obligations} - whole)
    if no_verdict:
        undecided.append(f"{len(no_verdict)} mutants have no verdict")

    exit_code = 0
    if errors or unheld or broken_list or hollow or blind:
        exit_code = 1
    if undecided:
        exit_code = 2

    with open(output, "w") as f:
        json.dump({
            "plan": plan, "whole": sorted(whole), "not_judged": unjudged, "undecided": undecided,
            "counts": dict(counts), "contract_errors": errors, "broken_declarations": broken_list,
            "mutants": ms, "obligations": merged_obligations, "tests": merged_tests, "exit": exit_code,
        }, f, indent=2, ensure_ascii=False)

    print(f"Merged {len(reports)} of {len(plan['shards'])} shards: {len(ms)} mutants")
    print("  " + ", ".join(f"{n} {s}" for s, n in sorted(counts.items())))

    def section(title, items, show):
        if items:
            print(f"\n{title} ({len(items)}):")
            for item in items:
                print("  " + show(item))

    section("Couldn't decide", undecided, str)
    section("Contract errors", errors, lambda p: f"{p['path']}:{p['line']}: {p['message']}")
    section("Unheld mutants", unheld, lambda m: f"{m['file']}:{m['line']}  {m['status']}  {m['id']}")
    section("Broken declarations", broken_list, lambda p: f"{p['path']}:{p['line']}: {p['message']}")
    section("Hollow obligations", hollow, lambda o: o["id"])
    section("Blind tests", blind, lambda t: f"{t['name']} (ran against {t['ran']} mutants)")
    section("Obligations held only by crashes", crash_only, lambda o: o["id"])
    section("Mutants killed only from outside their primitive", outside_only,
            lambda m: f"{m['file']}:{m['line']}  {m['id']}")
    if unjudged:
        print("\nNot judged, since no shards mutated them whole: " + ", ".join(unjudged))
    print("\n" + ["The Contract holds.", "The Contract doesn't hold.", "The merge couldn't decide."][exit_code])
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
