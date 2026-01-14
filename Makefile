BINARY_NAME := service-provider-manager

build:
	go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

run:
	go run ./cmd/$(BINARY_NAME)

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

.PHONY: build run clean fmt vet test tidy
