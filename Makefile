GO ?= go
PROTOC ?= protoc
BIN_DIR ?= bin
PROTOC_GEN_GO_VERSION ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.2

.PHONY: test build tools generate clean

test:
	$(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/deployctl .
	$(GO) build -o $(BIN_DIR)/deployctld ./cmd/deployctld

tools:
	mkdir -p $(BIN_DIR)
	GOBIN=$(CURDIR)/$(BIN_DIR) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(CURDIR)/$(BIN_DIR) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

generate: tools
	PATH="$(CURDIR)/$(BIN_DIR):$$PATH" $(PROTOC) \
		--go_out=. --go_opt=module=deployctl \
		--go-grpc_out=. --go-grpc_opt=module=deployctl \
		api/deployctl/v1/deployctl.proto

clean:
	rm -rf $(BIN_DIR)
