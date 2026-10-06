$ErrorActionPreference = "Stop"
$dockerfile = Get-Content -Raw (Join-Path $PSScriptRoot "..\Dockerfile")
if ($dockerfile -notmatch 'USER 10001:10001') { throw "backend image must run as non-root UID 10001" }
if ($dockerfile -notmatch 'HEALTHCHECK.+/livez') { throw "backend image must define a /livez healthcheck" }
if ($dockerfile -notmatch 'CGO_ENABLED=0') { throw "backend binary must be statically built" }
if ($dockerfile -match 'COPY \. \.' ) { throw "Dockerfile must not copy the entire repository into the build context" }
Write-Host "Backend Dockerfile static checks passed."
