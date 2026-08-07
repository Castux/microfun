#!/usr/bin/env bash
# Assembles the static site (docs + playground) into a deployable directory.
#
#   web/build.sh [outdir]     # default outdir: _site
#
# Requires Go. The result is self-contained: serve it with any static file
# server, e.g. `python -m http.server -d _site`.

set -euo pipefail
cd "$(dirname "$0")/.."

out="${1:-_site}"
rm -rf "$out"
mkdir -p "$out/docs" "$out/examples"

# The site is Thunky-branded; the wasm binary keeps the module name.

# The wasm build of the compiler+runtime, and the Go-version-matched JS shim.
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$out/thunky.wasm" .
goroot="$(go env GOROOT)"
if [ -f "$goroot/lib/wasm/wasm_exec.js" ]; then
    cp "$goroot/lib/wasm/wasm_exec.js" "$out/"
else
    cp "$goroot/misc/wasm/wasm_exec.js" "$out/"   # Go < 1.24 layout
fi

# Static assets.
cp web/index.html web/playground.html web/style.css web/favicon.svg \
   web/site.js web/playground.js web/runner.js web/worker.js web/thunky-mode.js \
   "$out/"

# The markdown the site renders, and the example programs the playground loads.
# docs/implementation/ is intentionally excluded — see the note in web/site.js.
mkdir -p "$out/docs/tutorial"
cp README.md LICENSE.md "$out/"
cp docs/*.md "$out/docs/"
cp docs/tutorial/*.md "$out/docs/tutorial/"
cp examples/*.þ "$out/examples/"

echo "site assembled in $out/"
