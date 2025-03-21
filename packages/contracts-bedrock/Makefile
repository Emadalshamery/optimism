# Define the output binaries
GO_FFI_TARGETS := go-ffi go-ffi-cannon64

# Find all Go source files in the directory
GO_SOURCES := $(wildcard ./scripts/go-ffi/*.go)

# Default target to build both
go-ffi: ./scripts/go-ffi/go-ffi ./scripts/go-ffi/go-ffi-cannon64

# Rule for standard build
./scripts/go-ffi/go-ffi: $(GO_SOURCES)
	cd ./scripts/go-ffi && go build -o ./go-ffi

# Rule for cannon64 build
./scripts/go-ffi/go-ffi-cannon64: $(GO_SOURCES)
	cd ./scripts/go-ffi && go build -tags=cannon64 -o ./go-ffi-cannon64

# Clean target
clean:
	rm -f ./scripts/go-ffi/go-ffi ./scripts/go-ffi/go-ffi-cannon64

.PHONY: all clean
