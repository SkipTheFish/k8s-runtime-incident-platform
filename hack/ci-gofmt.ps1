param([string]$BaseRef = '')
$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($BaseRef)) {
    $files = @(git diff --name-only --diff-filter=ACMR HEAD^ HEAD -- '*.go')
} else {
    git fetch --no-tags --depth=1 origin $BaseRef
    $files = @(git diff --name-only --diff-filter=ACMR "origin/$BaseRef...HEAD" -- '*.go')
}
$files = @($files | Where-Object { $_ -and (Test-Path -LiteralPath $_) })
if ($files.Count -eq 0) { Write-Host 'No changed Go files'; exit 0 }
$unformatted = @(& gofmt -l $files)
if ($unformatted.Count -gt 0) { throw "gofmt required:`n$($unformatted -join "`n")" }
Write-Host "gofmt check passed for $($files.Count) changed Go files"
