.PHONY: build test lint run clean tidy

build:
	go build -o reflect ./cmd/reflect

test:
	go test ./...

run: build
	./reflect

tidy:
	go mod tidy

lint:
	go vet ./...

clean:
	rm -f reflect
	rm -rf dist
