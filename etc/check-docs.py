#!/usr/bin/env python3
"""Run every runnable code block in the documentation.

The docs site (web/site.js) turns an untagged fenced block into a snippet with
a Run button, wrapping it in `show ( ... )` when it mentions no output builtin.
This reproduces that exactly and reports any block that does not compile and
run, so a broken example cannot reach the site unnoticed.

    python etc/check-docs.py [--verbose]

Exit status is non-zero if any runnable block fails.
"""

import glob
import io
import os
import re
import subprocess
import sys
import tempfile

BINARY = os.path.join(os.path.dirname(__file__), "..", "thunky.exe")
if not os.path.exists(BINARY):
    BINARY = os.path.join(os.path.dirname(__file__), "..", "thunky")

# Info strings that mean "not a runnable Thunky program"; must match the
# STATIC_LANGS set and the thunky-static check in web/site.js.
STATIC = {"text", "txt", "output", "ebnf", "bnf", "sh", "bash", "console",
          "go", "yaml", "json", "none", "thunky-static"}

OUTPUT_BUILTINS = re.compile(r"\b(show|peek|write|bwrite)\b")


def auto_show(src):
    """Mirror site.js autoShow: wrap a bare expression so it prints."""
    stripped = re.sub(r"--[^\n]*", "", re.sub(r"('[^']*'|\"[^\"]*\")", "''", src))
    if OUTPUT_BUILTINS.search(stripped) or re.search(r"\bmodule\b", stripped):
        return src
    m = re.match(r"^\s*import[\w\s,]*?\bin\b", src)
    if m:
        return m.group(0) + "\nshow (\n" + src[m.end():] + "\n)"
    return "show (\n" + src + "\n)"


def blocks(path):
    """Yield (line_number, info_string, body) for each fenced block."""
    lines = io.open(path, encoding="utf-8").read().split("\n")
    inside, info, start, buf = False, None, 0, []
    for i, line in enumerate(lines, 1):
        if line.startswith("```"):
            if not inside:
                inside, info, start, buf = True, line[3:].strip(), i, []
            else:
                inside = False
                yield start, info, "\n".join(buf)
        elif inside:
            buf.append(line)


def main():
    verbose = "--verbose" in sys.argv
    root = os.path.join(os.path.dirname(__file__), "..")
    paths = [os.path.join(root, "README.md"),
             os.path.join(root, "docs", "LANGUAGE.md")]
    paths += sorted(glob.glob(os.path.join(root, "docs", "tutorial", "*.md")))

    failures, ran, skipped = [], 0, 0
    for path in paths:
        rel = os.path.relpath(path, root).replace("\\", "/")
        for line, info, body in blocks(path):
            if info in STATIC:
                skipped += 1
                continue
            if not body.strip():
                continue
            ran += 1
            with tempfile.NamedTemporaryFile("w", suffix=".th", delete=False,
                                             encoding="utf-8") as fh:
                fh.write(auto_show(body))
                tmp = fh.name
            try:
                proc = subprocess.run([BINARY, tmp], capture_output=True,
                                      text=True, timeout=20, encoding="utf-8",
                                      errors="replace")
                out = (proc.stdout or "") + (proc.stderr or "")
                ok = proc.returncode == 0
            except subprocess.TimeoutExpired:
                out, ok = "(timed out after 20s)", False
            finally:
                os.unlink(tmp)

            if not ok:
                # A failing block often prints some output first; the useful
                # part is the diagnostic, which carries a source location.
                diag = [l for l in out.strip().split("\n") if re.search(r":\d+:\d+:|error", l)]
                failures.append((rel, line, (diag or out.strip().split("\n"))[0]))
            if verbose:
                print("%-4s %s:%d" % ("ok" if ok else "FAIL", rel, line))

    print("\n%d runnable blocks, %d static, %d failed" % (ran, skipped, len(failures)))
    for rel, line, msg in failures:
        print("  FAIL %s:%d  %s" % (rel, line, msg[:100]))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
