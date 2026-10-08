dev-up:
	docker-compose -f deploy/docker-compose.yml up -d

dev-down:
	docker-compose -f deploy/docker-compose.yml down

run:
	go run cmd/server/main.go

test:
	go test ./... -cover

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
