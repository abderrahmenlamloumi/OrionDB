.PHONY: build test run-agent run-ingester run-consumer

build:
	docker compose -f deploy/docker-compose.yml build

test:
	go test ./...
	python -m compileall mlops

run-agent:
	go run ./agent

run-ingester:
	go run ./ingester

run-consumer:
	go run ./consumer

clean:
	rm -f bin/*

bench:
	go test -bench=. -benchmem ./internal/ingest