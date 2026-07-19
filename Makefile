.PHONY: build test lint run clean tidy

build:
	go build -o knowthyself ./cmd/knowthyself

test:
	go test ./...

run: build
	./knowthyself

tidy:
	go mod tidy

lint:
	go vet ./...

clean:
	rm -f knowthyself
	rm -rf dist
