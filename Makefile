.PHONY: build clean run

BINARY_NAME=orchestrator

build:
	go build -o $(BINARY_NAME) cmd/orchestrator/main.go

clean:
	rm -f $(BINARY_NAME)

run: build
	./$(BINARY_NAME)
