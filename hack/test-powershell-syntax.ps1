param([string[]]$Path = @("$PSScriptRoot/*.ps1"))
$ErrorActionPreference = 'Stop'
$files = Get-ChildItem -Path $Path -File
foreach ($file in $files) {
    $tokens = $null
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) { throw "$($file.Name): $($errors -join '; ')" }
}
Write-Host "PowerShell syntax checks passed for $($files.Count) files"
