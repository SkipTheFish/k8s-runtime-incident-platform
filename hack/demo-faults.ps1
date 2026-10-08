param([string]$ClusterName = 'krip-demo')
$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$context = "kind-$ClusterName"
if (-not (kind get clusters | Where-Object { $_ -eq $ClusterName })) { throw "demo cluster not found: $ClusterName; run make demo first" }
kubectl --context $context create namespace incident-demo --dry-run=client -o yaml | kubectl --context $context apply -f -
kubectl --context $context apply -f (Join-Path $root 'deploy/demo/correlation-policies.yaml')
kubectl --context $context apply -f (Join-Path $root 'deploy/demo/faults.yaml')
Write-Host @"
Fault scenarios applied: ImagePull failure, CrashLoopBackOff, FailedScheduling.
Console:    kubectl --context $context -n runtime-system port-forward svc/runtime-incident-platform-frontend 8081:80
API:        kubectl --context $context -n runtime-system port-forward svc/runtime-incident-platform-backend 8080:8080
Incidents:  http://127.0.0.1:8080/api/v1/incidents
Receiver:   kubectl --context $context -n runtime-system logs -f deployment/webhook-receiver
Resources:  kubectl --context $context -n incident-demo get pods,events
"@
