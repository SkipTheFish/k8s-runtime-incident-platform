# Development

## Go toolchain and dependency baseline

The project supports Go 1.26.x and pins Go 1.26.9 as the preferred toolchain in
`go.mod`. The Kubernetes libraries are kept on the mutually compatible v0.36
line, with controller-runtime v0.24. This matches the controller-runtime
compatibility matrix and avoids an unrelated dependency upgrade.

Before submitting a backend change, run:

```sh
go mod tidy
go build ./...
go vet ./...
go test ./...
```

## Configuration

Copy `.env.example` to `.env` for local development and replace the example
PostgreSQL password. The `.env` file is ignored by Git. With
`KUBERNETES_ENABLED=false`, no kubeconfig or in-cluster service account is
required; PostgreSQL remains required. URLs must be absolute HTTP(S) URLs,
durations use Go duration syntax such as `10s`, and `WEBHOOK_WORKERS` must be
between 1 and 64.

For Docker Compose, pass the same variable names through an `env_file` or the
Compose service `environment` section. Use Compose service DNS names (for
example `postgres:5432` and `redis:6379`) rather than loopback addresses.

In Kubernetes, put non-sensitive values in a ConfigMap and inject them with
`envFrom` or individual `env` entries. Inject `POSTGRES_DSN` and any future
tokens, passwords, webhook keys, or other credentials from a Kubernetes Secret.
Do not store real credentials in `.env.example`, ConfigMaps, image layers, or
committed manifests. Set `KUBERNETES_ENABLED=true` in-cluster; the application
then uses the pod's service account configuration.

The supported variables are:

| Variable | Required | Default |
| --- | --- | --- |
| `APP_NAME` | No | `runtime-incident-platform` |
| `APP_PORT` | No | `8080` |
| `SHUTDOWN_TIMEOUT` | No | `10s` |
| `POSTGRES_DSN` | Yes | None |
| `POSTGRES_TIMEOUT` | No | `5s` |
| `REDIS_ADDR` | No | `127.0.0.1:6379` |
| `PROMETHEUS_URL` | No | `http://127.0.0.1:9090` |
| `LOKI_URL` | No | `http://127.0.0.1:3100` |
| `TEMPO_URL` | No | `http://127.0.0.1:3200` |
| `CLUSTER_ID` | Yes | None |
| `KUBERNETES_ENABLED` | No | `true` |
| `WEBHOOK_WORKERS` | No | `2` |
| `SIGNAL_WORKERS` | No | `2` |
| `SIGNAL_MAX_RETRIES` | No | `5` |
| `SIGNAL_RETRY_BASE` | No | `100ms` |
| `SIGNAL_RETRY_MAX` | No | `30s` |
