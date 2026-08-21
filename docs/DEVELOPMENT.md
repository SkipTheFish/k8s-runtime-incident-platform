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
