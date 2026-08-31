param(
    [string]$ContainerName = "krip-migration-test",
    [int]$Port = 15432
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
$image = "postgres:15.19"
$password = "migration-test-only"
$dsn = "postgres://postgres:$password@127.0.0.1:$Port/postgres?sslmode=disable"

if ($ContainerName -ne "krip-migration-test") {
    throw "Refusing unexpected container name"
}

try {
    docker run --detach --name $ContainerName --publish "${Port}:5432" --env "POSTGRES_PASSWORD=$password" $image | Out-Null
    $deadline = (Get-Date).AddMinutes(2)
    do {
        docker exec $ContainerName pg_isready -U postgres 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { break }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)
    if ($LASTEXITCODE -ne 0) { throw "PostgreSQL 15 did not become ready" }

    $env:POSTGRES_DSN = $dsn
    go run ./cmd/migrate -command up
    go run ./cmd/migrate -command up
    go run ./cmd/migrate -command status

    docker exec $ContainerName createdb -U postgres legacy
    docker exec $ContainerName psql -U postgres -d legacy -v ON_ERROR_STOP=1 -c @"
CREATE TABLE incidents (
  id SERIAL PRIMARY KEY, tenant_id TEXT NOT NULL, namespace TEXT NOT NULL,
  policy_name TEXT NOT NULL, status TEXT NOT NULL,
  started_at TIMESTAMP DEFAULT now(), updated_at TIMESTAMP DEFAULT now(),
  last_event_at TIMESTAMP DEFAULT now()
);
INSERT INTO incidents (tenant_id, namespace, policy_name, status)
VALUES ('legacy', 'default', 'legacy-policy', 'open');
"@
    $env:POSTGRES_DSN = "postgres://postgres:$password@127.0.0.1:$Port/legacy?sslmode=disable"
    go run ./cmd/migrate -command up
    $count = docker exec $ContainerName psql -U postgres -d legacy -Atc "SELECT count(*) FROM incidents"
    if ($count.Trim() -ne "1") { throw "Legacy incident row count changed" }

    $env:POSTGRES_DSN = "postgres://postgres:$password@127.0.0.1:1/unreachable?sslmode=disable"
    $migrationFailed = $false
    try {
        go run ./cmd/migrate -command up 2>$null
    } catch {
        $migrationFailed = $true
    }
    if (-not $migrationFailed) { throw "Migration unexpectedly succeeded against an unavailable database" }
} finally {
    Remove-Item Env:POSTGRES_DSN -ErrorAction SilentlyContinue
    try { docker rm --force $ContainerName 2>$null | Out-Null } catch {}
}
