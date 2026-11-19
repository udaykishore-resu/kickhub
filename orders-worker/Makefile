.PHONY: tidy build test vet up down send logs

go.sum: go.mod
	go mod tidy

tidy:
	go mod tidy

build: go.sum
	go build ./...

test: go.sum
	go test ./...

vet: go.sum
	go vet ./...

MSG ?= hello

up: go.sum
	docker compose up --build -d

down:
	docker compose down -v

send:
	docker compose run --rm send "$(MSG)"

logs:
	docker compose logs -f worker
