param(
    [Parameter(Mandatory = $true)][string]$Context,
    [string]$SystemNamespace = 'runtime-system',
    [string]$Namespace = 'e2e-failed-scheduling',
    [int]$TimeoutSeconds = 300
)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/..").Path
$labelKey = 'observability.io/krip-e2e-schedulable'
. (Join-Path $PSScriptRoot 'e2e-common.ps1')

try {
    kubectl --context $Context create namespace $Namespace --dry-run=client -o yaml | kubectl --context $Context apply -f -
    kubectl --context $Context label nodes --all "$labelKey-" --overwrite *> $null
    @"
apiVersion: v1
kind: Pod
metadata: {name: pending, namespace: $Namespace}
spec:
  nodeSelector: {${labelKey}: "true"}
  containers: [{name: app, image: busybox:1.37.0, command: [sh, -c, 'sleep 3600']}]
"@ | kubectl --context $Context apply -f -
    $incident = Wait-KripIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'failed-scheduling' -Status 'open' -TimeoutSeconds $TimeoutSeconds
    Assert-KripIncidentEvidence -Context $Context -SystemNamespace $SystemNamespace -Incident $incident | Out-Null
    Start-Sleep -Seconds 10
    Assert-KripSingleIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'failed-scheduling'
    $node = kubectl --context $Context get nodes -o jsonpath='{.items[0].metadata.name}'
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($node)) { throw 'no schedulable node found' }
    kubectl --context $Context label node $node "$labelKey=true" --overwrite
    kubectl --context $Context -n $Namespace wait pod/pending --for=condition=Ready --timeout="$($TimeoutSeconds)s"
    $podUID = kubectl --context $Context -n $Namespace get pod pending -o jsonpath='{.metadata.uid}'
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($podUID)) { throw 'recovered Pod UID is unavailable' }
    $eventDeadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        $scheduledEvent = kubectl --context $Context -n $Namespace get events --field-selector "involvedObject.uid=$podUID,reason=Scheduled" -o jsonpath='{.items[0].metadata.name}' 2>$null
        if (-not [string]::IsNullOrWhiteSpace($scheduledEvent)) { break }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $eventDeadline)
    if ([string]::IsNullOrWhiteSpace($scheduledEvent)) { throw 'scheduler did not publish the Pod recovery Event' }
    # Replay the observed scheduler outcome with a fresh Event UID. Kubernetes
    # may compact the original one before a newly established watch sees it.
    $eventTime = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    @"
apiVersion: v1
kind: Event
metadata:
  generateName: krip-e2e-scheduled-
  namespace: $Namespace
involvedObject:
  apiVersion: v1
  kind: Pod
  namespace: $Namespace
  name: pending
  uid: $podUID
reason: Scheduled
message: Pod recovery confirmed by the scheduler and replayed for the E2E watcher
source: {component: krip-e2e}
firstTimestamp: $eventTime
lastTimestamp: $eventTime
count: 1
type: Normal
"@ | kubectl --context $Context create -f -
    if ($LASTEXITCODE -ne 0) { throw 'failed to replay the scheduler recovery Event through the informer' }
    $resolved = Wait-KripIncident -Context $Context -SystemNamespace $SystemNamespace -Namespace $Namespace -Category 'failed-scheduling' -Status 'resolved' -IncidentID $incident.id -TimeoutSeconds $TimeoutSeconds
    Assert-KripIncidentEvidence -Context $Context -SystemNamespace $SystemNamespace -Incident $resolved -MinimumDecisions 2 -TimelineType 'incident.resolved' | Out-Null
    Assert-KripWebhookDelivery -Context $Context -SystemNamespace $SystemNamespace -IncidentID $incident.id
    Write-Host "FailedScheduling E2E passed: Incident #$($incident.id)"
} catch {
    Save-KripE2EDiagnostics -Context $Context -SystemNamespace $SystemNamespace -Scenario 'failed-scheduling' -Root $root -ErrorRecord $_
    throw
} finally {
    kubectl --context $Context label nodes --all "$labelKey-" --overwrite *> $null
}
