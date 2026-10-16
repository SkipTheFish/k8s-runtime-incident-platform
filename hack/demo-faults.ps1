param(
    [string]$ClusterName = 'krip-demo',
    [string]$Context = '',
    [string]$SystemNamespace = 'runtime-system',
    [int]$TimeoutSeconds = 300
)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
if ([string]::IsNullOrWhiteSpace($Context)) { $Context = "kind-$ClusterName" }
$demoNamespace = 'incident-demo'

function Invoke-Backend([string]$Path) {
    $pod = kubectl --context $Context -n $SystemNamespace get pod -l app.kubernetes.io/component=backend -o jsonpath='{.items[0].metadata.name}'
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($pod)) { throw 'backend Pod is unavailable' }
    $body = kubectl --context $Context -n $SystemNamespace exec $pod -- wget -qO- "http://127.0.0.1:8080$Path"
    if ($LASTEXITCODE -ne 0) { throw "backend request failed: $Path" }
    return $body
}

kubectl --context $Context create namespace $demoNamespace --dry-run=client -o yaml | kubectl --context $Context apply -f -
kubectl --context $Context apply -f (Join-Path $root 'deploy/demo/correlation-policies.yaml')
kubectl --context $Context apply -f (Join-Path $root 'deploy/demo/faults.yaml')

$expected = @('image-pull-backoff', 'crash-loop-backoff', 'failed-scheduling')
$deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$incidents = @()
$missing = $expected
do {
    try {
        $page = (Invoke-Backend '/api/v1/incidents?pageSize=200') | ConvertFrom-Json
        $incidents = @($page.items | Where-Object { $_.namespace -eq $demoNamespace -and $_.status -in @('open','acknowledged','reopened') })
        $missing = @($expected | Where-Object { $_ -notin @($incidents.category) })
        if ($missing.Count -eq 0) { break }
    } catch {
        $missing = $expected
    }
    Start-Sleep -Seconds 5
} while ([DateTime]::UtcNow -lt $deadline)

if ($missing.Count -gt 0) { throw "timed out waiting for Incident categories: $($missing -join ', ')" }
foreach ($category in $expected) {
    $matches = @($incidents | Where-Object category -eq $category)
    if ($matches.Count -ne 1) { throw "expected exactly one active $category Incident, found $($matches.Count)" }
    $incident = $matches[0]
    if ([string]::IsNullOrWhiteSpace($incident.fingerprint)) { throw "$category Incident has no fingerprint" }
    $incidentContext = (Invoke-Backend "/api/v1/incidents/$($incident.id)/context") | ConvertFrom-Json
    if ($incidentContext.schema -ne 'incident-context/v1' -or @($incidentContext.signals).Count -lt 1 -or @($incidentContext.timeline).Count -lt 1 -or @($incidentContext.correlationDecisions).Count -lt 1) {
        throw "$category Incident context is incomplete"
    }
}

$metrics = Invoke-Backend '/metrics'
if ($metrics -notmatch 'webhook_delivery_total\{[^}]*outcome="success"[^}]*\}\s+[1-9]') { throw 'no successful Webhook delivery was observed' }
$receiverLogs = kubectl --context $Context -n $SystemNamespace logs deployment/webhook-receiver --tail=500
foreach ($header in @('x-krip-signature','x-krip-timestamp','x-krip-event-id')) {
    if ($receiverLogs -notmatch "(?i)$header") { throw "Webhook receiver did not observe header: $header" }
}

Write-Host @"
Verified: one active Incident for each expected category, complete Context data, and signed Webhook delivery.
Console:    kubectl --context $Context -n $SystemNamespace port-forward svc/runtime-incident-platform-frontend 8081:80
API:        kubectl --context $Context -n $SystemNamespace port-forward svc/runtime-incident-platform-backend 8080:8080
Incidents:  http://127.0.0.1:8080/api/v1/incidents
Receiver:   kubectl --context $Context -n $SystemNamespace logs -f deployment/webhook-receiver
Resources:  kubectl --context $Context -n $demoNamespace get pods,events
"@
