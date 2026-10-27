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
    param(
        [string]$Context,
        [string]$SystemNamespace,
        [object]$Incident,
        [int]$MinimumDecisions = 1,
        [string]$TimelineType = 'incident.created',
        [int]$TimeoutSeconds = 30
    )
    if ([string]::IsNullOrWhiteSpace($Incident.fingerprint)) { throw 'Incident fingerprint is empty' }
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $incidentContext = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path "/api/v1/incidents/$($Incident.id)/context") | ConvertFrom-Json
        $signals = @($incidentContext.signals)
        $decisions = @($incidentContext.correlationDecisions)
        $timelineTypes = @($incidentContext.timeline | ForEach-Object { $_.type })
        if ($incidentContext.schema -eq 'incident-context/v1' -and
            $signals.Count -ge $MinimumDecisions -and
            $decisions.Count -ge $MinimumDecisions -and
            $TimelineType -in $timelineTypes) {
            return $incidentContext
        }
        Start-Sleep -Seconds 2
    } while ([DateTime]::UtcNow -lt $deadline)

    $actualSchema = if ($null -eq $incidentContext) { '<missing>' } else { [string]$incidentContext.schema }
    $actualSignals = if ($null -eq $incidentContext) { 0 } else { @($incidentContext.signals).Count }
    $actualDecisions = if ($null -eq $incidentContext) { 0 } else { @($incidentContext.correlationDecisions).Count }
    $actualTimeline = if ($null -eq $incidentContext) { '<missing>' } else { (@($incidentContext.timeline | ForEach-Object { $_.type }) -join ',') }
    throw "Incident $($Incident.id) evidence incomplete after ${TimeoutSeconds}s: schema=$actualSchema signals=$actualSignals decisions=$actualDecisions timeline=[$actualTimeline], expected decisions>=$MinimumDecisions and timeline=$TimelineType"
}

function Assert-KripSingleIncident {
    param([string]$Context, [string]$SystemNamespace, [string]$Namespace, [string]$Category)
    $page = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/api/v1/incidents?pageSize=200') | ConvertFrom-Json
    $matches = @($page.items | Where-Object { $_.namespace -eq $Namespace -and $_.category -eq $Category })
    if ($matches.Count -ne 1) { throw "expected one deduplicated $Category Incident, found $($matches.Count)" }
}

function Assert-KripWebhookDelivery {
    param([string]$Context, [string]$SystemNamespace, [long]$IncidentID)
    $metrics = @(Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/metrics') -join "`n"
    if ($metrics -notmatch 'webhook_delivery_total\{[^}]*outcome="success"[^}]*\}\s+[1-9]') { throw 'no successful Webhook delivery was observed' }
    $logs = @(kubectl --context $Context -n $SystemNamespace logs deployment/webhook-receiver --tail=500) -join "`n"
    foreach ($header in @('x-krip-signature', 'x-krip-timestamp', 'x-krip-event-id')) {
        if ($logs -notmatch "(?i)$header") { throw "Webhook receiver did not observe $header" }
    }
    if ($logs -notmatch "(?i)[`"']?incidentID[`"']?\s*[:=]\s*$IncidentID(?:\D|$)") {
        throw "Webhook receiver did not observe a payload for Incident $IncidentID"
    }
}

function Save-KripE2EDiagnostics {
    param([string]$Context, [string]$SystemNamespace, [string]$Scenario, [string]$Root, [object]$ErrorRecord)
    $directory = Join-Path $Root "deploy/tmp/e2e/$Scenario"
    New-Item -ItemType Directory -Force $directory | Out-Null
    kubectl --context $Context get pods,events -A -o wide *> (Join-Path $directory 'cluster.txt')
    kubectl --context $Context -n $SystemNamespace logs -l app.kubernetes.io/component=backend --all-containers --tail=1000 *> (Join-Path $directory 'backend.log')
    kubectl --context $Context -n $SystemNamespace logs deployment/webhook-receiver --tail=1000 *> (Join-Path $directory 'webhook-receiver.log')
    try { (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/metrics') *> (Join-Path $directory 'metrics.txt') } catch {}
    try {
        $incidents = (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path '/api/v1/incidents?pageSize=200')
        $incidents *> (Join-Path $directory 'incidents.json')
        $page = $incidents | ConvertFrom-Json
        foreach ($item in @($page.items | Where-Object { $_.namespace -eq "e2e-$Scenario" })) {
            (Invoke-KripBackend -Context $Context -SystemNamespace $SystemNamespace -Path "/api/v1/incidents/$($item.id)/context") *> (Join-Path $directory "incident-$($item.id)-context.json")
        }
    } catch {}
    if ($null -ne $ErrorRecord) {
        ($ErrorRecord | Format-List * -Force | Out-String) *> (Join-Path $directory 'error.txt')
    }
}
