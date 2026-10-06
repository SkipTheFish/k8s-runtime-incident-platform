$ErrorActionPreference = 'Stop'
$dockerfile = Get-Content "$PSScriptRoot/../runtime-console/Dockerfile" -Raw
$nginx = Get-Content "$PSScriptRoot/../runtime-console/nginx.conf" -Raw
foreach ($required in @('npm ci', 'npm run build', 'nginx-unprivileged:', 'USER 101', 'HEALTHCHECK')) {
    if (-not $dockerfile.Contains($required)) { throw "frontend Dockerfile missing: $required" }
}
if (-not $nginx.Contains('try_files $uri $uri/ /index.html')) { throw 'SPA fallback is missing' }
if (-not $nginx.Contains('proxy_pass http://runtime-incident-platform-backend:8080')) { throw 'API proxy is missing' }
Write-Host 'frontend image definition checks passed'
