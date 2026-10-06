#!/usr/bin/env python3
"""Plans a full mutation run as shards small enough for one GitHub runner each.

A primitive with few enough mutants is one shard, mutated whole. A bigger one is split by
operator, and merge-reports.py judges it once its shards have run every operator between them.
Prints the plan as JSON: every operator flinch has, and the shards.

Usage: plan-shards.py FLINCH [PRIMITIVE ...]
With no primitives it plans every one in contract/.
"""
import json
import os
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor

# About as many mutants as a 4-CPU runner gets through in two hours, which leaves room under
# the job's timeout for a slow one.
LIMIT = 130


def lines(*cmd):
    out = subprocess.run(cmd, check=True, capture_output=True, text=True).stdout
    return [line for line in out.splitlines() if line.strip()]


def main():
    flinch, wanted = sys.argv[1], sys.argv[2:]
    operators = [line.split()[0] for line in lines(flinch, "operators")]
    primitives = wanted or sorted(
        d for d in os.listdir("contract") if os.path.isdir(os.path.join("contract", d))
    )

    def count(primitive, operator=None):
        args = [flinch, "--dry-run", "--only", primitive]
        if operator:
            args += ["--operators", operator]
        return len(lines(*args))

    shards = []
    with ThreadPoolExecutor(max_workers=4) as pool:
        for primitive, n in zip(primitives, pool.map(count, primitives)):
            if n <= LIMIT:
                shards.append({"name": primitive, "only": primitive, "operators": "", "mutants": n})
                continue
            sizes = list(pool.map(lambda op: count(primitive, op), operators))
            # First fit, biggest operator first.
            groups = []
            for size, op in sorted(zip(sizes, operators), reverse=True):
                for g in groups:
                    if g["mutants"] + size <= LIMIT:
                        g["ops"].append(op)
                        g["mutants"] += size
                        break
                else:
                    groups.append({"ops": [op], "mutants": size})
            for i, g in enumerate(groups, 1):
                ops = sorted(g["ops"], key=operators.index)
                shards.append({
                    "name": f"{primitive}-{i}", "only": primitive,
                    "operators": ",".join(ops), "mutants": g["mutants"],
                })

    print(json.dumps({"operators": operators, "shards": shards}))


if __name__ == "__main__":
    main()
