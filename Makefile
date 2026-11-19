.PHONY: run dev test vet build

# Run the application.
run:
	go run .

# Run with hot reload (the entry is root main.go).
dev:
	viva dev .

test:
	go test ./... -count=1

vet:
	go vet ./...
	viva vet

build:
	viva build
