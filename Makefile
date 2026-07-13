.PHONY: build test vet clean run

BINARY=hookrelay
CMD_DIR=cmd/hookrelay

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) hookrelay.json

docker-build:
	docker build -t hookrelay .
