param(
    [Parameter(Mandatory = $true)][string]$Context,
    [string]$Namespace = 'runtime-system',
    [string]$Release = 'krip',
    [int]$TimeoutSeconds = 300
)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."

foreach ($tool in @('kubectl','helm')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "required tool is missing: $tool" }
}
kubectl --context $Context cluster-info *> $null
if ($LASTEXITCODE -ne 0) { throw "Kubernetes context is unavailable: $Context" }
foreach ($crd in @('prometheusrules.monitoring.coreos.com','servicemonitors.monitoring.coreos.com')) {
    kubectl --context $Context get crd $crd *> $null
    if ($LASTEXITCODE -ne 0) { throw "required Prometheus Operator CRD is missing: $crd" }
}

$values = helm get values $Release --kube-context $Context --namespace $Namespace -o json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw "Helm release was not found: $Namespace/$Release" }
if ($values.config.kubernetesEnabled -ne $true) { throw 'k3s acceptance requires config.kubernetesEnabled=true' }
if ($values.webhook.enabled -ne $true) { throw 'k3s acceptance requires webhook.enabled=true' }

kubectl --context $Context -n $Namespace wait --for=condition=Available deployment --all --timeout="$($TimeoutSeconds)s"
$deployment = kubectl --context $Context -n $Namespace get deployment -l app.kubernetes.io/component=backend -o json | ConvertFrom-Json
$serviceAccount = $deployment.items[0].spec.template.spec.serviceAccountName
if ([string]::IsNullOrWhiteSpace($serviceAccount)) { throw 'backend ServiceAccount was not found' }
$identity = "system:serviceaccount:$Namespace`:$serviceAccount"
foreach ($permission in @(
    @('update','alertpolicies.observability.io'),
    @('patch','alertpolicies.observability.io'),
    @('update','correlationpolicies/status')
)) {
    $allowed = kubectl --context $Context auth can-i $permission[0] $permission[1] --as $identity -n incident-demo
    if ($allowed -ne 'yes') { throw "RBAC denied $($permission -join ' ') for $identity" }
}

$pod = kubectl --context $Context -n $Namespace get pod -l app.kubernetes.io/component=backend -o jsonpath='{.items[0].metadata.name}'
foreach ($endpoint in @('/livez','/readyz')) {
    kubectl --context $Context -n $Namespace exec $pod -- wget -qO- "http://127.0.0.1:8080$endpoint" *> $null
    if ($LASTEXITCODE -ne 0) { throw "backend endpoint failed: $endpoint" }
}

& (Join-Path $PSScriptRoot 'demo-faults.ps1') -Context $Context -SystemNamespace $Namespace -TimeoutSeconds $TimeoutSeconds
Write-Host "k3s acceptance passed for $Context ($Namespace/$Release)"
