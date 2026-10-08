$ErrorActionPreference = 'Stop'
$script = Get-Content "$PSScriptRoot/demo-clean.ps1" -Raw
foreach ($required in @("if (`$ClusterName -ne 'krip-demo')", 'kind delete cluster --name $ClusterName', 'Remove-Item -LiteralPath $resolved')) {
    if (-not $script.Contains($required)) { throw "cleanup safety guard missing: $required" }
}
if ($script -match 'Remove-Item\s+[^\r\n]*(\*|\$root\b)') { throw 'broad cleanup target detected' }
& "$PSScriptRoot/demo-clean.ps1" -DryRun
Write-Host 'demo cleanup safety checks passed'
