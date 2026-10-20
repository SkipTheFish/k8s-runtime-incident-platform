$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$chart = Join-Path $root 'deploy/helm/runtime-incident-platform'
helm lint $chart
$rendered = (helm template krip $chart --namespace runtime-system) -join "`n"
if ($LASTEXITCODE -ne 0) { throw 'helm template failed' }
if ($rendered -match 'verbs:\s*\[[^\]]*"?\*') { throw 'wildcard RBAC verb is forbidden' }
if ($rendered -match 'resources:\s*\[[^\]]*"?\*') { throw 'wildcard RBAC resource is forbidden' }
if ($rendered -notmatch 'resources: \[alertpolicies, correlationpolicies, observabilitytargets, observetenants\][\s\S]*?verbs: \[get, list, watch, patch, update\]') {
    throw 'CRD main-resource update permissions required by finalizers are missing'
}
if ($rendered -notmatch 'resources: \[namespaces\][\s\S]*?verbs: \[create\]') { throw 'ObserveTenant namespace create permission is missing' }
if ($rendered -notmatch 'resources: \[roles\][\s\S]*?verbs: \[get, list, watch, create, patch, update\]') { throw 'ObserveTenant Role permissions are missing' }
@('AlertPolicy', 'CorrelationPolicy', 'ObservabilityTarget', 'ObserveTenant') | ForEach-Object {
    if (-not (Select-String -Path "$chart/crds/*.yaml" -Pattern "kind: $_" -Quiet)) { throw "missing CRD: $_" }
}
Write-Host 'Helm static checks passed'
