# Performance comparison harness: interpreter (--mode=interp) vs compiled
# bytecode (--mode=compiled). PowerShell counterpart of bench/run.sh.
#
# For every bench/cases/*.mf this runs the program in BOTH modes, checks the two
# modes produce identical output (a correctness guard — a perf number is
# meaningless if the backends disagree), times each, and prints a table with the
# speedup ratio.
#
# These are deliberately SLOW (each case ~10-30s per mode) and are NOT part of
# the regression suite in tests/, nor the stdlib unit tests in
# examples/core_tests.mf. Run them by hand when evaluating backend performance.
#
# Usage:
#   pwsh bench/run.ps1 [name]          run all cases (or only <name>)
#   pwsh bench/run.ps1 -Reps N [name]  run each mode N times, keep the best (default 1)

param(
    [int]$Reps = 1,
    [string]$Name = ''
)

$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
Set-Location $root
$bin = Join-Path $root 'microfun.bench.exe'

Write-Host "building $bin ..."
& go build -o $bin .
if ($LASTEXITCODE -ne 0) { Write-Host 'build failed'; exit 1 }

# Run-Mode runs the binary $Reps times, keeping the fastest wall-clock, and
# returns the best seconds plus a hash of stdout (so we can assert both modes
# agree without holding the whole output).
function Run-Mode([string]$mode, [string]$relPath) {
    $best = [double]::PositiveInfinity
    $out = ''
    for ($i = 0; $i -lt $Reps; $i++) {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $out = & $bin "--mode=$mode" $relPath
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
Write-Host ("{0,-18} {1,10} {2,10} {3,8}   {4}" -f 'case', 'interp(s)', 'compiled(s)', 'speedup', 'match')
Write-Host ('-' * 67)

$totalI = 0.0
$totalC = 0.0
$fail = 0

Get-ChildItem -Path 'bench/cases' -Filter '*.mf' | Sort-Object Name | ForEach-Object {
    $caseName = $_.BaseName
    if ($Name -ne '' -and $caseName -ne $Name) { return }
    $rel = "bench/cases/$caseName.mf"

    $i = Run-Mode 'interp' $rel
    $c = Run-Mode 'compiled' $rel

    if ($i.Hash -eq $c.Hash) { $match = 'ok' } else { $match = 'DIFFER'; $script:fail++ }

    if ($c.Seconds -gt 0) { $speedup = ('{0:N2}x' -f ($i.Seconds / $c.Seconds)) } else { $speedup = 'n/a' }
    Write-Host ("{0,-18} {1,10:N3} {2,10:N3} {3,8}   {4}" -f $caseName, $i.Seconds, $c.Seconds, $speedup, $match)

    $script:totalI += $i.Seconds
    $script:totalC += $c.Seconds
}

Write-Host ('-' * 67)
if ($totalC -gt 0) { $speedup = ('{0:N2}x' -f ($totalI / $totalC)) } else { $speedup = 'n/a' }
Write-Host ("{0,-18} {1,10:N3} {2,10:N3} {3,8}" -f 'TOTAL', $totalI, $totalC, $speedup)

if ($fail -ne 0) {
    Write-Host ''
    Write-Host "$fail case(s) produced DIFFERENT output between modes — investigate before trusting timings."
    exit 1
}
