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
