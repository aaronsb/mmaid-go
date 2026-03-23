.PHONY: build test test-visual clean install lint vet

BINARY := mmaid
BUILD_DIR := .

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/mmaid

install:
	go install ./cmd/mmaid

test:
	go test ./... -v

test-short:
	go test ./... -short

test-visual: build
	./test_visual.sh ./$(BINARY)

vet:
	go vet ./...

lint: vet
	@echo "Lint passed (go vet)"

clean:
	rm -f $(BINARY)

all: clean build test
