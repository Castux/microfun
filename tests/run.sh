#!/usr/bin/env bash
#
# Regression harness for the microfun front-end and compiler backend.
#
# For every tests/cases/<category>/<name>.mf this performs two checks:
#
#   * DIFFERENTIAL — run under --mode=interp (the AST tree-walker, the oracle)
#     and --mode=compiled (the bytecode VM); their combined output and exit code
#     must be byte-identical. Any divergence is a compiler bug.
#
#   * GOLDEN — the interpreter's output and exit code must match the recorded
#     expectation in <name>.expected (and <name>.exit, when non-zero). This
#     catches regressions in the lexer, parser, analyzer, and interpreter that a
#     purely differential check would miss (both backends could regress together,
#     and lexing/parsing errors happen identically in both modes).
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
#                                the interpreter — review the diff before keeping
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

tmp_i=$(mktemp)
tmp_c=$(mktemp)
trap 'rm -f "$tmp_i" "$tmp_c" "$BIN"' EXIT

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

	"$BIN" --mode=interp "$mf" <"$in" >"$tmp_i" 2>&1
	i_code=$?

	if [ "$bless" -eq 1 ]; then
		cp "$tmp_i" "$base.expected"
		if [ "$i_code" -ne 0 ]; then echo "$i_code" >"$base.exit"; else rm -f "$base.exit"; fi
		printf '  BLESS %s (exit %d)\n' "$name" "$i_code"
		continue
	fi

	"$BIN" --mode=compiled "$mf" <"$in" >"$tmp_c" 2>&1
	c_code=$?

	expected_exit=0
	[ -f "$base.exit" ] && expected_exit=$(cat "$base.exit")

	reasons=""
	if ! cmp -s "$tmp_i" "$tmp_c" || [ "$i_code" != "$c_code" ]; then
		reasons="$reasons differential(interp exit $i_code != compiled exit $c_code or output differs)"
	fi
	if [ ! -f "$base.expected" ]; then
		reasons="$reasons golden(no .expected — run --bless)"
	elif ! cmp -s "$tmp_i" "$base.expected" || [ "$i_code" != "$expected_exit" ]; then
		reasons="$reasons golden(exit $i_code != expected $expected_exit or output differs)"
	fi

	if [ -z "$reasons" ]; then
		pass=$((pass + 1))
		printf '  PASS  %s\n' "$name"
	else
		fail=$((fail + 1))
		printf '  FAIL  %s --%s\n' "$name" "$reasons"
		diff "$base.expected" "$tmp_i" 2>/dev/null | sed 's/^/        | /' | head -20
	fi
done < <(find tests/cases -name '*.mf' | sort)

if [ "$bless" -eq 1 ]; then
	printf '\n=== blessed expectations ===\n'
	exit 0
fi

printf '\n=== %d passed, %d failed ===\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
