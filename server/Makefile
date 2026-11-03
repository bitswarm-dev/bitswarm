.PHONY: run build test vet tidy

run:
	air -c .air.toml

build:
	go build -o ./bin/server ./cmd/server

test:
	go test ./... -race

vet:
	go vet ./...

tidy:
	go mod tidy
