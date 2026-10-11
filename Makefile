BINARY     := opencode-extractor
BUILD_DIR  := build
SOURCE_DIR := source
OUTPUT     := $(CURDIR)/$(BUILD_DIR)/$(BINARY)

.PHONY: all build test clean

all: build

build:
	mkdir -p $(BUILD_DIR)
	cd $(SOURCE_DIR) && go build -trimpath -o $(OUTPUT) ./cmd/$(BINARY)

test:
	cd $(SOURCE_DIR) && go test ./...

clean:
	rm -f $(OUTPUT)
