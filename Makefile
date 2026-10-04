GO_BIN_DIR := $(shell go env GOBIN)
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif
export PATH := $(GO_BIN_DIR):$(PATH)

.PHONY: setup proto run clean gofmt

setup:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0

proto:
	mkdir -p grpc
	protoc -I proto --go_out=grpc --go_opt=paths=source_relative \
		--go-grpc_out=grpc --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=grpc --grpc-gateway_opt=paths=source_relative \
		--grpc-gateway_opt=generate_unbound_methods=true \
		hello.proto

run:
	go run .

gofmt:
	gofmt -w *.go grpc/*.go

clean:
	rm -f grpc/*.pb.go grpc/*.pb.gw.go
