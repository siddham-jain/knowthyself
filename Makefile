.PHONY: build test lint run clean tidy

build:
	go build -o synch ./cmd/synch

test:
	go test ./...

run: build
	./synch

tidy:
	go mod tidy

lint:
	go vet ./...

clean:
	rm -f synch
	rm -rf dist
