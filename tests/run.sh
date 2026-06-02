#!/usr/bin/env bash
#
# Regression harness for the microfun front-end and the G-machine backend.
#
# microfun now has a single execution engine, so this is a GOLDEN harness: for
# every tests/cases/<category>/<name>.mf it runs the engine and checks that the
# combined stdout+stderr and the exit code match the recorded expectation in
# <name>.expected (and <name>.exit, when the exit code is non-zero). The golden
# files were produced by the pre-rewrite tree-walking interpreter (the frozen
# oracle) and are the authority the engine must reproduce byte-for-byte.
#
# A sibling <name>.in, if present, is fed as standard input (raw bytes).
#
# These are distinct from the in-language standard-library unit tests in
# examples/core_tests.mf.
#
# Usage:
#   tests/run.sh [category]      run all cases (or only one category)
#   tests/run.sh --bless [category]
#                                (re)generate <name>.expected / <name>.exit from
#                                the current engine — review the diff before keeping
#
# Paths are passed to the binary as forward-slash, repo-root-relative strings so
# the source path embedded in diagnostics is identical on every platform and the
# golden files stay portable.

set -u
cd "$(dirname "$0")/.." # repo root

bless=0
filter=""
for arg in "$@"; do
	case "$arg" in
	--bless) bless=1 ;;
	*) filter="$arg" ;;
	esac
done

BIN=./microfun.test.exe
echo "building $BIN ..."
if ! go build -o "$BIN" .; then
	echo "build failed"
	exit 1
fi

tmp=$(mktemp)
trap 'rm -f "$tmp" "$BIN"' EXIT

pass=0
fail=0
current_cat=""

while IFS= read -r mf; do
	cat_dir=$(basename "$(dirname "$mf")")
	if [ -n "$filter" ] && [ "$cat_dir" != "$filter" ]; then
		continue
	fi
	name=$(basename "$mf" .mf)
	base="${mf%.mf}"

	if [ "$cat_dir" != "$current_cat" ]; then
		current_cat="$cat_dir"
		printf '\n[%s]\n' "$current_cat"
	fi

	in="$base.in"
	[ -f "$in" ] || in=/dev/null

	"$BIN" "$mf" <"$in" >"$tmp" 2>&1
	code=$?

	if [ "$bless" -eq 1 ]; then
		cp "$tmp" "$base.expected"
		if [ "$code" -ne 0 ]; then echo "$code" >"$base.exit"; else rm -f "$base.exit"; fi
		printf '  BLESS %s (exit %d)\n' "$name" "$code"
		continue
	fi

	expected_exit=0
	[ -f "$base.exit" ] && expected_exit=$(cat "$base.exit")

	reasons=""
	if [ ! -f "$base.expected" ]; then
		reasons="$reasons golden(no .expected — run --bless)"
	elif ! cmp -s "$tmp" "$base.expected" || [ "$code" != "$expected_exit" ]; then
		reasons="$reasons golden(exit $code != expected $expected_exit or output differs)"
	fi

	if [ -z "$reasons" ]; then
		pass=$((pass + 1))
		printf '  PASS  %s\n' "$name"
	else
		fail=$((fail + 1))
		printf '  FAIL  %s --%s\n' "$name" "$reasons"
		diff "$base.expected" "$tmp" 2>/dev/null | sed 's/^/        | /' | head -20
	fi
done < <(find tests/cases -name '*.mf' | sort)

if [ "$bless" -eq 1 ]; then
	printf '\n=== blessed expectations ===\n'
	exit 0
fi

printf '\n=== %d passed, %d failed ===\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
