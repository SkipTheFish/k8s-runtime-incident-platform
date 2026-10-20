param([switch]$RequireComplete)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/..").Path
$acceptance = Join-Path $root 'docs/ACCEPTANCE.md'
$manifest = Join-Path $root 'docs/RELEASE_MANIFEST.md'
$chart = Join-Path $root 'deploy/helm/runtime-incident-platform/Chart.yaml'

foreach ($path in @($acceptance, $manifest, $chart, (Join-Path $root 'docs/openapi.yaml'), (Join-Path $root 'docs/performance/FIRST_CAPACITY_BASELINE.md'))) {
    if (-not (Test-Path -LiteralPath $path)) { throw "release evidence is missing: $path" }
}
$chartText = Get-Content -Raw -LiteralPath $chart
if ($chartText -notmatch '(?m)^version:\s+0\.1\.0\s*$' -or $chartText -notmatch '(?m)^appVersion:\s+"0\.1\.0"\s*$') {
    throw 'Chart version and appVersion must match the v0.1.0 release candidate'
}
$latestMigration = Get-ChildItem (Join-Path $root 'migrations') -Filter '*.sql' | Sort-Object Name | Select-Object -Last 1 -ExpandProperty Name
if ($latestMigration -ne '00011_outbox_replay_audit.sql') { throw "unexpected latest migration: $latestMigration" }
$acceptanceText = Get-Content -Raw -LiteralPath $acceptance
$pending = ([regex]::Matches($acceptanceText, '(?m)^- \[ \] ')).Count
if ($RequireComplete -and $pending -gt 0) { throw "$pending release acceptance item(s) are still pending" }
if ($RequireComplete -and (Get-Content -Raw -LiteralPath $manifest) -match '<pending>|待发布流水线确定|打 tag 前记录') {
    throw 'release manifest still contains unresolved image or commit identity'
}
Write-Host "Release evidence structure passed; pending acceptance items: $pending"
