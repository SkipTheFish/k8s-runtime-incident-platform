dev-up:
	docker-compose -f deploy/docker-compose.yml up -d

dev-down:
	docker-compose -f deploy/docker-compose.yml down

run:
	go run cmd/server/main.go

test:
	go test ./... -cover