.PHONY: build test lint run tidy

build:
	go build -o weatherdash ./cmd/weatherdash

test:
	go test ./... -race

lint:
	go vet ./...
	gofmt -l .

run: build
	./weatherdash

tidy:
	go mod tidy
