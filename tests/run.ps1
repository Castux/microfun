# Regression harness for the microfun front-end and compiler backend (PowerShell).
#
# For every tests/cases/<category>/<name>.mf this performs two checks:
#
#   * DIFFERENTIAL — run under --mode=interp (the oracle) and --mode=compiled;
#     their combined output and exit code must be byte-identical.
#   * GOLDEN — the interpreter's output and exit code must match the recorded
#     <name>.expected (and <name>.exit, when non-zero).
#
# A sibling <name>.in, if present, is fed as standard input (raw bytes, so the
# invalid-UTF-8 case works). Output is captured as raw bytes (via the process
# BaseStream) so no console encoding mangles it. Paths are passed forward-slash,
# repo-root-relative, matching the portable golden files.
#
# Usage:
#   pwsh tests/run.ps1 [category]
#   pwsh tests/run.ps1 -Bless [category]   # (re)generate .expected / .exit
#
# These are distinct from the stdlib unit tests in examples/core_tests.mf.

param(
    [switch]$Bless,
    [string]$Category = ''
)

$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
Set-Location $root
$bin = Join-Path $root 'microfun.test.exe'

Write-Host "building $bin ..."
& go build -o $bin .
if ($LASTEXITCODE -ne 0) { Write-Host 'build failed'; exit 1 }

# Invoke-MF runs the binary in the given mode, feeding inFile as raw stdin, and
# returns the combined stdout+stderr as a byte array plus the exit code. Reading
# the raw BaseStream avoids any console-encoding transformation.
function Invoke-MF([string]$mode, [string]$relPath, [string]$inFile) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $bin
    $psi.Arguments = "--mode=$mode `"$relPath`""
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $p = [System.Diagnostics.Process]::Start($psi)
    if ($inFile -and (Test-Path $inFile)) {
        $bytes = [System.IO.File]::ReadAllBytes($inFile)
        $p.StandardInput.BaseStream.Write($bytes, 0, $bytes.Length)
    }
    $p.StandardInput.Close()
    $ms = New-Object System.IO.MemoryStream
    $p.StandardOutput.BaseStream.CopyTo($ms)
    $p.StandardError.BaseStream.CopyTo($ms)
    $p.WaitForExit()
    return @{ Bytes = $ms.ToArray(); Code = $p.ExitCode }
}

function Bytes-Equal($a, $b) {
    if ($a.Length -ne $b.Length) { return $false }
    return [System.Linq.Enumerable]::SequenceEqual([byte[]]$a, [byte[]]$b)
}

$pass = 0
$fail = 0
$cur = ''

Get-ChildItem -Path 'tests/cases' -Recurse -Filter '*.mf' | Sort-Object FullName | ForEach-Object {
    $catd = Split-Path (Split-Path $_.FullName -Parent) -Leaf
    if ($Category -ne '' -and $catd -ne $Category) { return }
    if ($catd -ne $cur) { $cur = $catd; Write-Host ''; Write-Host "[$cur]" }

    $rel = $_.FullName.Substring($root.Path.Length + 1).Replace('\', '/')
    $base = [System.IO.Path]::ChangeExtension($_.FullName, $null).TrimEnd('.')
    $inFile = "$base.in"
    $expectedFile = "$base.expected"
    $exitFile = "$base.exit"

    $i = Invoke-MF 'interp' $rel $inFile

    if ($Bless) {
        [System.IO.File]::WriteAllBytes($expectedFile, $i.Bytes)
        if ($i.Code -ne 0) { Set-Content -Path $exitFile -Value $i.Code -NoNewline }
        elseif (Test-Path $exitFile) { Remove-Item $exitFile }
        Write-Host ("  BLESS " + $_.BaseName + " (exit $($i.Code))")
        return
    }

    $c = Invoke-MF 'compiled' $rel $inFile

    $expectedExit = 0
    if (Test-Path $exitFile) { $expectedExit = [int](Get-Content -Raw $exitFile) }

    $reasons = @()
    if (-not (Bytes-Equal $i.Bytes $c.Bytes) -or ($i.Code -ne $c.Code)) {
        $reasons += "differential(interp exit $($i.Code), compiled exit $($c.Code))"
    }
    if (-not (Test-Path $expectedFile)) {
        $reasons += "golden(no .expected -- run -Bless)"
    } else {
        $expected = [System.IO.File]::ReadAllBytes($expectedFile)
        if (-not (Bytes-Equal $i.Bytes $expected) -or ($i.Code -ne $expectedExit)) {
            $reasons += "golden(exit $($i.Code), expected $expectedExit)"
        }
    }

    if ($reasons.Count -eq 0) {
        $pass++
        Write-Host ("  PASS  " + $_.BaseName)
    } else {
        $fail++
        Write-Host ("  FAIL  " + $_.BaseName + " -- " + ($reasons -join '; '))
    }
}

Remove-Item $bin -ErrorAction SilentlyContinue

if ($Bless) { Write-Host ''; Write-Host '=== blessed expectations ==='; exit 0 }

Write-Host ''
Write-Host "=== $pass passed, $fail failed ==="
if ($fail -ne 0) { exit 1 }
