#!/usr/bin/env bash
#
# Performance comparison harness: interpreter (--mode=interp) vs builder bytecode
# VM (--mode=compiled) vs spineless tagless G-machine (--mode=stg).
#
# For every bench/cases/*.mf this runs the program in ALL THREE modes, checks
# that they produce identical output (a correctness guard — a perf number is
# meaningless if the backends disagree), times each, and prints a table with the
# speedup of compiled and stg over the interpreter. The stg-vs-i column is the
# headline number for IMPROVEMENTS.md §1: the STG eliminates the per-activation
# application-spine allocation the builder VM still pays.
#
# These are deliberately SLOW (each case ~10-30s per mode) and are NOT part of
# the regression suite in tests/. They are run by hand when evaluating backend
# performance. They are also distinct from the in-language stdlib unit tests in
# examples/core_tests.mf.
#
# Usage:
#   bench/run.sh [name]        run all cases (or only bench/cases/<name>.mf)
#   bench/run.sh --reps N ...  run each mode N times and keep the best (default 1)
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

# run_mode MODE FILE -> echoes "<seconds> <output-hash>"; runs $reps times, keeps
# the fastest wall-clock. Output is captured and hashed so we can assert both
# modes agree without holding multi-MB strings.
run_mode() {
	local mode="$1" file="$2"
	local best="" out h
	for _ in $(seq 1 "$reps"); do
		local start end dur
		start=$(date +%s.%N)
		out=$("$BIN" --mode="$mode" "$file")
		end=$(date +%s.%N)
		dur=$(awk -v a="$start" -v b="$end" 'BEGIN { printf "%.3f", b - a }')
		if [ -z "$best" ] || awk -v d="$dur" -v b="$best" 'BEGIN { exit !(d < b) }'; then
			best="$dur"
		fi
	done
	h=$(printf '%s' "$out" | cksum | cut -d' ' -f1)
	echo "$best $h"
}

printf '\n%-16s %9s %11s %7s %11s %7s   %s\n' \
	"case" "interp(s)" "compiled(s)" "c-vs-i" "stg(s)" "stg-vs-i" "match"
printf '%s\n' "------------------------------------------------------------------------------------"

total_i=0
total_c=0
total_s=0
fail=0

while IFS= read -r mf; do
	name=$(basename "$mf" .mf)
	rel="bench/cases/$name.mf"
	if [ -n "$filter" ] && [ "$name" != "$filter" ]; then
		continue
	fi

	read -r i_time i_hash < <(run_mode interp "$rel")
	read -r c_time c_hash < <(run_mode compiled "$rel")
	read -r s_time s_hash < <(run_mode stg "$rel")

	if [ "$i_hash" = "$c_hash" ] && [ "$i_hash" = "$s_hash" ]; then
		match="ok"
	else
		match="DIFFER"
		fail=$((fail + 1))
	fi

	c_speedup=$(awk -v i="$i_time" -v c="$c_time" 'BEGIN { if (c > 0) printf "%.2fx", i / c; else printf "n/a" }')
	s_speedup=$(awk -v i="$i_time" -v s="$s_time" 'BEGIN { if (s > 0) printf "%.2fx", i / s; else printf "n/a" }')
	printf '%-16s %9s %11s %7s %11s %7s   %s\n' "$name" "$i_time" "$c_time" "$c_speedup" "$s_time" "$s_speedup" "$match"

	total_i=$(awk -v t="$total_i" -v x="$i_time" 'BEGIN { printf "%.3f", t + x }')
	total_c=$(awk -v t="$total_c" -v x="$c_time" 'BEGIN { printf "%.3f", t + x }')
	total_s=$(awk -v t="$total_s" -v x="$s_time" 'BEGIN { printf "%.3f", t + x }')
done < <(find bench/cases -name '*.mf' | sort)

printf '%s\n' "------------------------------------------------------------------------------------"
c_speedup=$(awk -v i="$total_i" -v c="$total_c" 'BEGIN { if (c > 0) printf "%.2fx", i / c; else printf "n/a" }')
s_speedup=$(awk -v i="$total_i" -v s="$total_s" 'BEGIN { if (s > 0) printf "%.2fx", i / s; else printf "n/a" }')
printf '%-16s %9s %11s %7s %11s %7s\n' "TOTAL" "$total_i" "$total_c" "$c_speedup" "$total_s" "$s_speedup"

if [ "$fail" -ne 0 ]; then
	printf '\n%d case(s) produced DIFFERENT output between modes — investigate before trusting timings.\n' "$fail"
	exit 1
fi
