.PHONY: build test race vet fmt cover benchmark fuzz docker docker-run docker-smoke

build:
	mkdir -p bin
	go build -o bin/forgedb ./cmd/forgedb

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

benchmark:
	go test -bench=. -benchmem ./benchmarks

fuzz:
	go test ./internal/sql/parser -run=^$$ -fuzz=FuzzParserDoesNotPanic -fuzztime=10s

docker:
	docker build -t forgedb:local .

docker-run:
	docker run --rm -it -v "$$PWD/data:/data" forgedb:local shell /data/app.db

docker-smoke:
	bash tests/docker-smoke.sh
