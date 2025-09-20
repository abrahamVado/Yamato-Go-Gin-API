    run:
	go run ./cmd/server

    tidy:
	go mod tidy

    build:
	go build -o ./bin/auth ./cmd/server

