# Performance harness for the Thunky G-machine. PowerShell counterpart of
# bench/run.sh.
#
# For every bench/cases/*.þ this times the engine and prints a table. Thunky
# now has a single execution engine, so there is no cross-backend comparison; the
# headline number is wall-clock per case. If a frozen oracle binary
# (thunky.oracle.exe) is present it is timed alongside as a baseline and its
# output is checked to still match -- a perf number is meaningless if the engine
# disagrees with the oracle.
#
# These are deliberately SLOW and are NOT part of the regression suite in tests/,
# nor the stdlib unit tests in examples/core_tests.þ. Run them by hand when
# evaluating performance.
#
# Usage:
#   pwsh bench/run.ps1 [name]          run all cases (or only <name>)
#   pwsh bench/run.ps1 -Reps N [name]  run each case N times, keep the best (default 1)

param(
    [int]$Reps = 1,
    [string]$Name = ''
)

$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
Set-Location $root
$bin = Join-Path $root 'thunky.bench.exe'
$oracle = Join-Path $root 'thunky.oracle.exe'
$haveOracle = Test-Path $oracle

Write-Host "building $bin ..."
& go build -o $bin .
if ($LASTEXITCODE -ne 0) { Write-Host 'build failed'; exit 1 }

# Run-Bin runs the binary $Reps times, keeping the fastest wall-clock, and returns
# the best seconds plus a hash of stdout (so we can assert the engine agrees with
# the oracle without holding the whole output).
function Run-Bin([string]$exe, [string]$relPath) {
    $best = [double]::PositiveInfinity
    $out = ''
    for ($i = 0; $i -lt $Reps; $i++) {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $out = & $exe $relPath
        $sw.Stop()
        $dur = $sw.Elapsed.TotalSeconds
        if ($dur -lt $best) { $best = $dur }
    }
    $text = ($out -join "`n")
    $md5 = [System.Security.Cryptography.MD5]::Create()
    $hash = [BitConverter]::ToString($md5.ComputeHash([Text.Encoding]::UTF8.GetBytes($text)))
    return @{ Seconds = $best; Hash = $hash }
}

Write-Host ''
if ($haveOracle) {
    Write-Host ("{0,-16} {1,11} {2,11} {3,9}   {4}" -f 'case', 'engine(s)', 'oracle(s)', 'speedup', 'match')
    Write-Host ('-' * 67)
} else {
    Write-Host ("{0,-16} {1,11}" -f 'case', 'engine(s)')
    Write-Host ('-' * 28)
}

$totalE = 0.0
$totalO = 0.0
$fail = 0

Get-ChildItem -Path 'bench/cases' -Filter '*.þ' | Sort-Object Name | ForEach-Object {
    $caseName = $_.BaseName
    if ($Name -ne '' -and $caseName -ne $Name) { return }
    $rel = "bench/cases/$caseName.þ"

    $e = Run-Bin $bin $rel
    $script:totalE += $e.Seconds

    if ($haveOracle) {
        $o = Run-Bin $oracle $rel
        $script:totalO += $o.Seconds
        if ($e.Hash -eq $o.Hash) { $match = 'ok' } else { $match = 'DIFFER'; $script:fail++ }
        if ($e.Seconds -gt 0) { $speedup = ('{0:N2}x' -f ($o.Seconds / $e.Seconds)) } else { $speedup = 'n/a' }
        Write-Host ("{0,-16} {1,11:N3} {2,11:N3} {3,9}   {4}" -f $caseName, $e.Seconds, $o.Seconds, $speedup, $match)
    } else {
        Write-Host ("{0,-16} {1,11:N3}" -f $caseName, $e.Seconds)
    }
}

if ($haveOracle) {
    Write-Host ('-' * 67)
    if ($totalE -gt 0) { $speedup = ('{0:N2}x' -f ($totalO / $totalE)) } else { $speedup = 'n/a' }
    Write-Host ("{0,-16} {1,11:N3} {2,11:N3} {3,9}" -f 'TOTAL', $totalE, $totalO, $speedup)
    if ($fail -ne 0) {
        Write-Host ''
        Write-Host "$fail case(s) disagree with the oracle -- investigate before trusting timings."
        exit 1
    }
} else {
    Write-Host ('-' * 28)
    Write-Host ("{0,-16} {1,11:N3}" -f 'TOTAL', $totalE)
}
