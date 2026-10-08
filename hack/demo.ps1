param([string]$ClusterName = 'krip-demo', [switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$namespace = 'runtime-system'
$context = "kind-$ClusterName"
$chart = Join-Path $root 'deploy/helm/runtime-incident-platform'
$work = Join-Path $root 'deploy/tmp/demo'

foreach ($tool in @('docker','kind','kubectl','helm','openssl')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "required tool is missing: $tool" }
}
docker info *> $null
if (-not (kind get clusters | Where-Object { $_ -eq $ClusterName })) { kind create cluster --name $ClusterName }
kubectl --context $context create namespace $namespace --dry-run=client -o yaml | kubectl --context $context apply -f -
kubectl --context $context -n $namespace apply -f (Join-Path $root 'deploy/test/phase-9/dependencies.yaml')
kubectl --context $context -n $namespace rollout status deployment/postgres --timeout=180s
kubectl --context $context -n $namespace create secret generic runtime-incident-platform-database --from-literal=dsn='postgres://runtime:runtime@postgres:5432/runtime_incidents?sslmode=disable' --dry-run=client -o yaml | kubectl --context $context apply -f -

New-Item -ItemType Directory -Force $work | Out-Null
$key = Join-Path $work 'tls.key'; $cert = Join-Path $work 'tls.crt'
openssl req -x509 -newkey rsa:2048 -nodes -days 7 -subj '/CN=runtime-incident-platform-backend.runtime-system.svc' -addext 'subjectAltName=DNS:runtime-incident-platform-backend.runtime-system.svc' -keyout $key -out $cert 2>$null
kubectl --context $context -n $namespace create secret tls runtime-incident-platform-webhook-tls --key=$key --cert=$cert --dry-run=client -o yaml | kubectl --context $context apply -f -
kubectl --context $context -n $namespace create secret generic runtime-incident-platform-webhook --from-literal=secret='demo-only-not-for-production' --dry-run=client -o yaml | kubectl --context $context apply -f -
$caBundle = [Convert]::ToBase64String([IO.File]::ReadAllBytes($cert))

if (-not $SkipBuild) {
    docker build -t runtime-incident-platform-backend:local $root
    docker build -t runtime-incident-platform-frontend:local (Join-Path $root 'runtime-console')
}
kind load docker-image --name $ClusterName runtime-incident-platform-backend:local runtime-incident-platform-frontend:local
helm upgrade --install krip $chart --kube-context $context --namespace $namespace --set webhook.enabled=true --set webhook.existingSecret=runtime-incident-platform-webhook --set webhook.tlsSecret=runtime-incident-platform-webhook-tls --set-string webhook.caBundle=$caBundle --wait --timeout 5m
kubectl --context $context -n $namespace wait --for=condition=Available deployment --all --timeout=180s
Write-Host "Demo cluster is ready. Run 'make demo' again safely or continue with phase 9.8 fault injection."
