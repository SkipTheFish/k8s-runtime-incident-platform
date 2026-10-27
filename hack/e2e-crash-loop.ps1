param(
    [Parameter(Mandatory = $true)][string]$Context,
    [string]$SystemNamespace = 'runtime-system',
    [string]$Namespace = 'e2e-crash-loop',
    [int]$TimeoutSeconds = 300
)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/..").Path
. (Join-Path $PSScriptRoot 'e2e-common.ps1')

try {
    kubectl --context $Context create namespace $Namespace --dry-run=client -o yaml | kubectl --context $Context apply -f -
    @"
apiVersion: apps/v1
kind: Deployment
metadata: {name: crash-loop, namespace: $Namespace}
spec:
  replicas: 1
  selector: {matchLabels: {app: crash-loop}}
  template:
    metadata: {labels: {app: crash-loop}}
    spec:
      containers: [{name: app, image: busybox:1.37.0, command: [sh, -c, 'echo deterministic-e2e-failure; exit 23']}]
"@ | kubectl --context $Context apply -f -
    $incident = Wait-KripIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'crash-loop-backoff' -Status 'open' -TimeoutSeconds $TimeoutSeconds
    Assert-KripIncidentEvidence -Context $Context -SystemNamespace $SystemNamespace -Incident $incident | Out-Null
    Start-Sleep -Seconds 10
    Assert-KripSingleIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'crash-loop-backoff'
    @"
apiVersion: apps/v1
kind: Deployment
metadata: {name: crash-loop, namespace: $Namespace}
spec:
  replicas: 1
  selector: {matchLabels: {app: crash-loop}}
  template:
    metadata: {labels: {app: crash-loop}}
    spec:
      containers: [{name: app, image: busybox:1.37.0, command: [sh, -c, 'sleep 3600']}]
"@ | kubectl --context $Context apply -f -
    kubectl --context $Context -n $Namespace rollout status deployment/crash-loop --timeout="$($TimeoutSeconds)s"
    $resolved = Wait-KripIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'crash-loop-backoff' -Status 'resolved' -IncidentID $incident.id -TimeoutSeconds $TimeoutSeconds
    Assert-KripIncidentEvidence -Context $Context -SystemNamespace $SystemNamespace -Incident $resolved -MinimumDecisions 2 -TimelineType 'incident.resolved' | Out-Null
    Assert-KripWebhookDelivery -Context $Context -SystemNamespace $SystemNamespace -IncidentID $incident.id
    Write-Host "CrashLoopBackOff E2E passed: Incident #$($incident.id)"
} catch {
    Save-KripE2EDiagnostics -Context $Context -SystemNamespace $SystemNamespace -Scenario 'crash-loop' -Root $root -ErrorRecord $_
    throw
}
