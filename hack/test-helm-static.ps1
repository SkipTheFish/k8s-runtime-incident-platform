$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$chart = Join-Path $root 'deploy/helm/runtime-incident-platform'
helm lint $chart
$rendered = helm template krip $chart --namespace runtime-system
if ($LASTEXITCODE -ne 0) { throw 'helm template failed' }
if ($rendered -match 'verbs:\s*\[[^\]]*"?\*') { throw 'wildcard RBAC verb is forbidden' }
if ($rendered -match 'resources:\s*\[[^\]]*"?\*') { throw 'wildcard RBAC resource is forbidden' }
@('AlertPolicy', 'CorrelationPolicy', 'ObservabilityTarget', 'ObserveTenant') | ForEach-Object {
    if (-not (Select-String -Path "$chart/crds/*.yaml" -Pattern "kind: $_" -Quiet)) { throw "missing CRD: $_" }
}
Write-Host 'Helm static checks passed'
