param([string]$ClusterName = 'krip-ci-e2e', [switch]$KeepCluster)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/..").Path
$context = "kind-$ClusterName"

try {
    & (Join-Path $PSScriptRoot 'demo.ps1') -ClusterName $ClusterName -SkipFaults
    & (Join-Path $PSScriptRoot 'e2e-image-pull.ps1') -Context $context
    & (Join-Path $PSScriptRoot 'e2e-crash-loop.ps1') -Context $context
    & (Join-Path $PSScriptRoot 'e2e-failed-scheduling.ps1') -Context $context
} catch {
    New-Item -ItemType Directory -Force (Join-Path $root 'deploy/tmp/e2e/cluster') | Out-Null
    kubectl --context $context get all,events -A -o wide *> (Join-Path $root 'deploy/tmp/e2e/cluster/resources.txt')
    kubectl --context $context -n runtime-system logs -l app.kubernetes.io/component=backend --all-containers --tail=1000 *> (Join-Path $root 'deploy/tmp/e2e/cluster/backend.log')
    throw
} finally {
    if (-not $KeepCluster -and (kind get clusters | Where-Object { $_ -eq $ClusterName })) {
        kind delete cluster --name $ClusterName
    }
}
