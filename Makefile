.PHONY: build test vet clean run

BINARY=hookrelay
CMD_DIR=cmd/hookrelay

build:
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./... -count=1 -race

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) hookrelay.json

docker-build:
	docker build -t hookrelay .
