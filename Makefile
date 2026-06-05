BINARY := project-mcp-sync

.PHONY: build test run clean

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

# 사용 예: make run ARGS="sync --help"
run:
	go run . $(ARGS)

clean:
	rm -rf bin
