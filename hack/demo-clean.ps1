param([string]$ClusterName = 'krip-demo', [switch]$DryRun)
$ErrorActionPreference = 'Stop'
if ($ClusterName -ne 'krip-demo') { throw "refusing to clean non-demo cluster: $ClusterName" }
$root = (Resolve-Path "$PSScriptRoot/..").Path
$temp = Join-Path $root 'deploy/tmp/demo'
$allowedParent = (Join-Path $root 'deploy/tmp') + [IO.Path]::DirectorySeparatorChar

if (Get-Command kind -ErrorAction SilentlyContinue) {
    $exists = kind get clusters 2>$null | Where-Object { $_ -eq $ClusterName }
    if ($exists) {
        if ($DryRun) { Write-Host "Would delete kind cluster: $ClusterName" }
        else { kind delete cluster --name $ClusterName }
    }
} elseif (-not $DryRun) {
    Write-Warning 'kind is not installed; no cluster deletion was attempted'
}

if (Test-Path -LiteralPath $temp) {
    $resolved = (Resolve-Path -LiteralPath $temp).Path
    if (-not $resolved.StartsWith($allowedParent, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolved -Leaf) -ne 'demo') {
        throw "refusing to remove unexpected path: $resolved"
    }
    if ($DryRun) { Write-Host "Would remove generated demo files: $resolved" }
    else { Remove-Item -LiteralPath $resolved -Recurse -Force }
}
Write-Host 'Demo cleanup complete; source manifests and unrelated clusters were not changed.'
