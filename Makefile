BINARY := openvm
ifeq ($(OS),Windows_NT)
EXT := .exe
else
EXT :=
endif

ARGS ?=

build:
	@go build -o bin/$(BINARY)$(EXT) cmd/main.go

run:build
	@./bin/$(BINARY)$(EXT) $(ARGS)

test:
	@go test ./...

dev:run
