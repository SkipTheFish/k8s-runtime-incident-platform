param([string]$ClusterName = 'krip-helm-test', [switch]$KeepCluster)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$artifacts = Join-Path $root 'deploy/tmp/kind-helm'
$chart = Join-Path $root 'deploy/helm/runtime-incident-platform'
$namespace = 'runtime-system'
$created = $false

function Require-Tool([string]$name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) { throw "required tool is missing: $name" }
}
function Collect-Diagnostics {
    New-Item -ItemType Directory -Force $artifacts | Out-Null
    kubectl --context "kind-$ClusterName" -n $namespace get all -o wide *> (Join-Path $artifacts 'resources.txt')
    kubectl --context "kind-$ClusterName" -n $namespace get events --sort-by=.lastTimestamp *> (Join-Path $artifacts 'events.txt')
    kubectl --context "kind-$ClusterName" -n $namespace logs -l app.kubernetes.io/component=backend --all-containers --tail=500 *> (Join-Path $artifacts 'backend.log')
}

try {
    'docker','kind','kubectl','helm' | ForEach-Object { Require-Tool $_ }
    if (-not (kind get clusters | Select-String -SimpleMatch $ClusterName -Quiet)) {
        kind create cluster --name $ClusterName
        $created = $true
    }
    docker build -t runtime-incident-platform-backend:local $root
    docker build -t runtime-incident-platform-frontend:local (Join-Path $root 'runtime-console')
    kind load docker-image --name $ClusterName runtime-incident-platform-backend:local runtime-incident-platform-frontend:local
    kubectl --context "kind-$ClusterName" create namespace $namespace --dry-run=client -o yaml | kubectl --context "kind-$ClusterName" apply -f -
    kubectl --context "kind-$ClusterName" -n $namespace create secret generic runtime-incident-platform-database --from-literal=dsn='postgres://runtime:runtime@postgres:5432/runtime_incidents?sslmode=disable' --dry-run=client -o yaml | kubectl --context "kind-$ClusterName" apply -f -
    kubectl --context "kind-$ClusterName" -n $namespace apply -f (Join-Path $root 'deploy/test/phase-9/dependencies.yaml')
    kubectl --context "kind-$ClusterName" -n $namespace rollout status deployment/postgres --timeout=180s
    helm upgrade --install krip $chart --kube-context "kind-$ClusterName" --namespace $namespace --set config.kubernetesEnabled=false --wait --timeout 5m
    helm upgrade krip $chart --kube-context "kind-$ClusterName" --namespace $namespace --set config.kubernetesEnabled=false --set backend.resources.requests.cpu=110m --wait --timeout 5m
    kubectl --context "kind-$ClusterName" -n $namespace wait --for=condition=Available deployment --all --timeout=180s
    helm uninstall krip --kube-context "kind-$ClusterName" --namespace $namespace --wait
    if (-not (kubectl --context "kind-$ClusterName" get crd alertpolicies.observability.io --ignore-not-found)) { throw 'CRD was not retained after uninstall' }
} catch {
    Collect-Diagnostics
    throw
} finally {
    if ($created -and -not $KeepCluster) { kind delete cluster --name $ClusterName }
}
