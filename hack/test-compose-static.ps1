$ErrorActionPreference = 'Stop'
$root = Resolve-Path "$PSScriptRoot/.."
$composeFile = Join-Path $root 'deploy/docker-compose.yml'
$envFile = Join-Path $root '.env.example'
$rendered = docker compose --env-file $envFile -f $composeFile config --format json
if ($LASTEXITCODE -ne 0) { throw 'docker compose config failed' }
$config = $rendered | ConvertFrom-Json
$expected = @('postgres','redis','prometheus','loki','tempo','backend','frontend')
foreach ($name in $expected) {
    $service = $config.services.$name
    if ($null -eq $service) { throw "Compose service is missing: $name" }
    if ($null -eq $service.healthcheck) { throw "Compose healthcheck is missing: $name" }
}
if ($rendered -match '/docker-entrypoint-initdb.d') { throw 'Goose migrations must not be executed by the PostgreSQL image entrypoint' }
foreach ($volume in @('postgres-data','redis-data','prometheus-data','loki-data','tempo-data')) {
    if ($null -eq $config.volumes.$volume) { throw "persistent volume is missing: $volume" }
}
foreach ($dependency in @('postgres','redis','prometheus','loki','tempo')) {
    if ($config.services.backend.depends_on.$dependency.condition -ne 'service_healthy') { throw "backend does not wait for healthy $dependency" }
}
if ($config.services.frontend.depends_on.backend.condition -ne 'service_healthy') { throw 'frontend does not wait for a healthy backend' }
Write-Host 'Compose static checks passed'
