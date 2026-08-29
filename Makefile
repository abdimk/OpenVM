BINARY := openvm

ARGS ?=

build:
	@go build -o bin/$(BINARY) cmd/main.go

run:build
	@./bin/$(BINARY) $(ARGS)
	
dev:run
	
