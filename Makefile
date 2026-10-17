dev-up:
	docker-compose -f deploy/docker-compose.yml up -d

dev-down:
	docker-compose -f deploy/docker-compose.yml down

run:
	go run cmd/server/main.go

test:
	go test ./... -cover

ENVTEST_K8S_VERSION ?= 1.36.x
ENVTEST_CACHE ?= .cache/envtest
SETUP_ENVTEST ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.25.2

.PHONY: envtest-assets test-envtest
envtest-assets:
	$(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_CACHE) -p path

test-envtest:
	KUBEBUILDER_ASSETS="$$($(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_CACHE) -p path)" go test ./internal/operator -run Envtest -count=1

migrate-up:
	go run ./cmd/migrate -command up

migrate-status:
	go run ./cmd/migrate -command status

test-migrations:
	pwsh -NoProfile -File hack/test-migrations.ps1

.PHONY: demo demo-clean demo-faults demo-verify
demo:
	pwsh -NoProfile -File hack/demo.ps1

demo-verify:
	pwsh -NoProfile -File hack/demo.ps1 -SkipBuild

demo-faults:
	pwsh -NoProfile -File hack/demo-faults.ps1

demo-clean:
	pwsh -NoProfile -File hack/demo-clean.ps1
