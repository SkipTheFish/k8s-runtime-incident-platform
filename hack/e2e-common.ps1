Set-StrictMode -Version Latest

function Invoke-KripBackend {
    param([string]$Context, [string]$SystemNamespace, [string]$Path)
    $pod = kubectl --context $Context -n $SystemNamespace get pod -l app.kubernetes.io/component=backend -o jsonpath='{.items[0].metadata.name}'
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($pod)) { throw 'backend Pod is unavailable' }
    $body = kubectl --context $Context -n $SystemNamespace exec $pod -- wget -qO- "http://127.0.0.1:8080$Path"
    if ($LASTEXITCODE -ne 0) { throw "backend request failed: $Path" }
    return $body
}

function Wait-KripIncident {
    param([string]$Context, [string]$SystemNamespace, [string]$Namespace, [string]$Category, [string]$Status, [int]$TimeoutSeconds, [long]$IncidentID = 0)
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $page = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/api/v1/incidents?pageSize=200') | ConvertFrom-Json
        $matches = @($page.items | Where-Object {
            $_.namespace -eq $Namespace -and $_.category -eq $Category -and $_.status -eq $Status -and ($IncidentID -eq 0 -or $_.id -eq $IncidentID)
        })
        if ($matches.Count -eq 1) { return $matches[0] }
        Start-Sleep -Seconds 3
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "timed out waiting for one $Status $Category Incident in $Namespace"
}

function Assert-KripIncidentEvidence {
    param([string]$Context, [string]$SystemNamespace, [object]$Incident, [int]$MinimumDecisions = 1, [string]$TimelineType = 'incident.created')
    if ([string]::IsNullOrWhiteSpace($Incident.fingerprint)) { throw 'Incident fingerprint is empty' }
    $context = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path "/api/v1/incidents/$($Incident.id)/context") | ConvertFrom-Json
    if ($context.schema -ne 'incident-context/v1' -or @($context.signals).Count -lt $MinimumDecisions -or @($context.correlationDecisions).Count -lt $MinimumDecisions) {
        throw "Incident $($Incident.id) context is incomplete"
    }
    if ($TimelineType -notin @($context.timeline.type)) { throw "Incident $($Incident.id) timeline lacks $TimelineType" }
    return $context
}

function Assert-KripSingleIncident {
    param([string]$Context, [string]$SystemNamespace, [string]$Namespace, [string]$Category)
    $page = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/api/v1/incidents?pageSize=200') | ConvertFrom-Json
    $matches = @($page.items | Where-Object { $_.namespace -eq $Namespace -and $_.category -eq $Category })
    if ($matches.Count -ne 1) { throw "expected one deduplicated $Category Incident, found $($matches.Count)" }
}

function Assert-KripWebhookDelivery {
    param([string]$Context, [string]$SystemNamespace)
    $metrics = Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/metrics'
    if ($metrics -notmatch 'webhook_delivery_total\{[^}]*outcome="success"[^}]*\}\s+[1-9]') { throw 'no successful Webhook delivery was observed' }
    $logs = kubectl --context $Context -n $SystemNamespace logs deployment/webhook-receiver --tail=500
    foreach ($header in @('x-krip-signature', 'x-krip-timestamp', 'x-krip-event-id')) {
        if ($logs -notmatch "(?i)$header") { throw "Webhook receiver did not observe $header" }
    }
}

function Save-KripE2EDiagnostics {
    param([string]$Context, [string]$SystemNamespace, [string]$Scenario, [string]$Root)
    $directory = Join-Path $Root "deploy/tmp/e2e/$Scenario"
    New-Item -ItemType Directory -Force $directory | Out-Null
    kubectl --context $Context get pods,events -A -o wide *> (Join-Path $directory 'cluster.txt')
    kubectl --context $Context -n $SystemNamespace logs -l app.kubernetes.io/component=backend --all-containers --tail=1000 *> (Join-Path $directory 'backend.log')
    try { (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/api/v1/incidents?pageSize=200') *> (Join-Path $directory 'incidents.json') } catch {}
}
