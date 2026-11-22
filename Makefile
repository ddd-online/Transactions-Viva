.PHONY: run run-prod dev test vet build

# Run the application with the development config (~/.transactions.dev.json).
# Text input works here, unlike the `viva dev` reload host.
run:
	go run . -dev

# Run the application with the production config (~/.transactions.json).
run-prod:
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
