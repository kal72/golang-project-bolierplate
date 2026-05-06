.PHONY: proto proto-check wire run-api run-grpc lint

PROTO_DIR    := proto
GEN_DIR      := gen
PROTOC_FLAGS  = --go_out=$(GEN_DIR) \
                --go_opt=paths=source_relative \
                --go-grpc_out=$(GEN_DIR) \
                --go-grpc_opt=paths=source_relative \
                --proto_path=$(PROTO_DIR)

# ── proto ──────────────────────────────────────────────────────────────────

proto: proto-check gen-dir
	@echo "generating protobuf..."
	@find $(PROTO_DIR) -name "*.proto" | while read f; do \
		rel=$$(echo $$f | sed 's|^$(PROTO_DIR)/||'); \
		echo "  → $$f"; \
		protoc $(PROTOC_FLAGS) $$rel; \
	done
	@echo "done."

gen-dir:
	@mkdir -p $(GEN_DIR)

proto-check:
	@which protoc > /dev/null 2>&1 || \
		(echo "ERROR: protoc not found. Install: brew install protobuf" && exit 1)
	@which protoc-gen-go > /dev/null 2>&1 || \
		(echo "ERROR: protoc-gen-go not found. Run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest" && exit 1)
	@which protoc-gen-go-grpc > /dev/null 2>&1 || \
		(echo "ERROR: protoc-gen-go-grpc not found. Run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" && exit 1)

proto-clean:
	@echo "cleaning gen/..."
	@rm -rf $(GEN_DIR)
	@echo "done."

# ── wire ───────────────────────────────────────────────────────────────────

wire:
	@cd di && wire

# ── lint ───────────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

# ── install tools ──────────────────────────────────────────────────────────

install-tools:
	@echo "installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/google/wire/cmd/wire@latest
	@echo "done. make sure $(shell go env GOPATH)/bin is in your PATH"