#!/usr/bin/env bash
#
# Performance harness for the microfun G-machine.
#
# For every bench/cases/*.mf this times the engine and prints a table. microfun
# now has a single execution engine, so there is no cross-backend comparison; the
# headline number is wall-clock per case. If a frozen oracle binary
# (microfun.oracle.exe) is present it is timed alongside as a baseline and its
# output is checked to still match — a perf number is meaningless if the engine
# disagrees with the oracle.
#
# These are deliberately SLOW and are NOT part of the regression suite in tests/;
# run them by hand when evaluating performance. They are also distinct from the
# in-language stdlib unit tests in examples/core_tests.mf.
#
# Usage:
#   bench/run.sh [name]        run all cases (or only bench/cases/<name>.mf)
#   bench/run.sh --reps N ...  run each case N times and keep the best (default 1)
#
# Timing uses wall-clock; close other load for stable numbers. Paths are passed
# repo-root-relative with forward slashes so diagnostics stay portable.

set -u
cd "$(dirname "$0")/.." # repo root

reps=1
filter=""
while [ $# -gt 0 ]; do
	case "$1" in
	--reps) reps="$2"; shift 2 ;;
	*) filter="$1"; shift ;;
	esac
done

BIN=./microfun.bench.exe
echo "building $BIN ..."
if ! go build -o "$BIN" .; then
	echo "build failed"
	exit 1
fi
trap 'rm -f "$BIN"' EXIT

ORACLE=./microfun.oracle.exe
have_oracle=0
[ -x "$ORACLE" ] && have_oracle=1

# time_run BIN FILE -> echoes "<seconds> <output-hash>"; runs $reps times, keeps
# the fastest wall-clock. Output is captured and hashed so we can assert the
# engine agrees with the oracle without holding multi-MB strings.
time_run() {
	local bin="$1" file="$2"
	local best="" out h
	for _ in $(seq 1 "$reps"); do
		local start end dur
		start=$(date +%s.%N)
		out=$("$bin" "$file")
		end=$(date +%s.%N)
		dur=$(awk -v a="$start" -v b="$end" 'BEGIN { printf "%.3f", b - a }')
		if [ -z "$best" ] || awk -v d="$dur" -v b="$best" 'BEGIN { exit !(d < b) }'; then
			best="$dur"
		fi
	done
	h=$(printf '%s' "$out" | cksum | cut -d' ' -f1)
	echo "$best $h"
}

if [ "$have_oracle" -eq 1 ]; then
	printf '\n%-16s %11s %11s %9s   %s\n' "case" "engine(s)" "oracle(s)" "speedup" "match"
	printf '%s\n' "----------------------------------------------------------------------"
else
	printf '\n%-16s %11s\n' "case" "engine(s)"
	printf '%s\n' "----------------------------"
fi

total_e=0
total_o=0
fail=0

while IFS= read -r mf; do
	name=$(basename "$mf" .mf)
	rel="bench/cases/$name.mf"
	if [ -n "$filter" ] && [ "$name" != "$filter" ]; then
		continue
	fi

	read -r e_time e_hash < <(time_run "$BIN" "$rel")
	total_e=$(awk -v t="$total_e" -v x="$e_time" 'BEGIN { printf "%.3f", t + x }')

	if [ "$have_oracle" -eq 1 ]; then
		read -r o_time o_hash < <(time_run "$ORACLE" "$rel")
		total_o=$(awk -v t="$total_o" -v x="$o_time" 'BEGIN { printf "%.3f", t + x }')
		if [ "$e_hash" = "$o_hash" ]; then match="ok"; else match="DIFFER"; fail=$((fail + 1)); fi
		speedup=$(awk -v o="$o_time" -v e="$e_time" 'BEGIN { if (e > 0) printf "%.2fx", o / e; else printf "n/a" }')
		printf '%-16s %11s %11s %9s   %s\n' "$name" "$e_time" "$o_time" "$speedup" "$match"
	else
		printf '%-16s %11s\n' "$name" "$e_time"
	fi
done < <(find bench/cases -name '*.mf' | sort)

if [ "$have_oracle" -eq 1 ]; then
	printf '%s\n' "----------------------------------------------------------------------"
	speedup=$(awk -v o="$total_o" -v e="$total_e" 'BEGIN { if (e > 0) printf "%.2fx", o / e; else printf "n/a" }')
	printf '%-16s %11s %11s %9s\n' "TOTAL" "$total_e" "$total_o" "$speedup"
	if [ "$fail" -ne 0 ]; then
		printf '\n%d case(s) disagree with the oracle — investigate before trusting timings.\n' "$fail"
		exit 1
	fi
else
	printf '%s\n' "----------------------------"
	printf '%-16s %11s\n' "TOTAL" "$total_e"
fi
