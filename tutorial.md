# Build a Go API with protobuf, gRPC, and REST

This tutorial explains this repository from its API definition to a running server. You should know basic Go functions, structs, and interfaces. You do not need previous gRPC experience.

Run terminal commands from the repository root, where `go.mod` and `Makefile` live.

## 1. Understand the project

The application has one operation: give it a name and it returns a greeting. Clients can call it through gRPC or through an HTTP endpoint that accepts JSON.

```text
proto/hello.proto          API contract you edit
        |
        | protoc + three plugins (make proto)
        v
grpc/hello.pb.go           Message types and protobuf support
grpc/hello_grpc.pb.go      gRPC client and server plumbing
grpc/hello.pb.gw.go        HTTP/JSON gateway handlers
        |
        | main.go supplies the behavior and starts servers
        v
gRPC client -> :44444 -> gRPC handler -> Server.SayHello
HTTP client -> :8080  -> gateway      -> Server.SayHello
```

The REST path in this project calls the Go service implementation directly. It does not make a network request to port 44444.

Other important files:

| File | Purpose |
| --- | --- |
| `go.mod` | Module name, Go version requirement, and dependency versions |
| `go.sum` | Checksums used to verify downloaded Go modules |
| `Makefile` | Repeatable commands for installing plugins, generating code, and running the server |
| `.gitignore` | Keeps generated code out of new Git commits |

## 2. Read the protobuf file

Protobuf is both a schema language and a serialization system. The schema describes messages; serialization converts message values into bytes and back. gRPC uses the schema's service declarations to describe remote operations.

Here is `proto/hello.proto`:

```proto
syntax = "proto3";

package proto;

option go_package = "grpc-server-go/grpc;proto";

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloResponse);
}

message HelloRequest {
  string inp_name = 1;
}

message HelloResponse {
  string out_message = 1;
}
```

### `syntax`: choose the schema language version

`syntax = "proto3";` tells the compiler to use proto3 rules. It is unrelated to the Go version or gRPC version.

### `package`: give protobuf symbols a namespace

`package proto;` makes the service's full protobuf name `proto.Greeter`. The gRPC method name is `/proto.Greeter/SayHello`.

This protobuf namespace is separate from a Go import path. Changing it changes service identity and, with this project's default HTTP mapping, the REST URL too.

### `go_package`: choose the generated Go package

```proto
option go_package = "grpc-server-go/grpc;proto";
```

There are two parts:

- `grpc-server-go/grpc`: the Go import path, matching the module `grpc-server-go` and its `grpc` directory.
- `proto`: the Go package name written inside the generated files.

In `main.go`, we import that package with an alias:

```go
pb "grpc-server-go/grpc"
```

`pb` is a local nickname. It allows us to write `pb.HelloRequest` even though the generated package is named `proto`. It also avoids confusing the generated package with the gRPC library imported as `grpc`.

The output directory is controlled by the generation command. With `paths=source_relative`, `go_package` does not automatically place files into the `grpc` directory; `--go_out=grpc` does that.

### `message`: define request and response data

```proto
message HelloRequest {
  string inp_name = 1;
}
```

`HelloRequest` is a message type. `inp_name` is a string field, and `1` is its field number. The field number identifies the field in the binary wire format; it is not a default value or an array position.

Numbers are unique within each message, so both the request and response can have a field numbered `1`.

When evolving an API, preserve the numbers of existing fields. If you remove a field, reserve its number and name rather than assigning them to a different field:

```proto
// Example of a future schema AFTER removing inp_name:
message HelloRequest {
  reserved 1;
  reserved "inp_name";
  string display_name = 2;
}
```

For this proto3 string field, a missing value reads as the empty string. The current handler does not validate the name, so an empty request produces `"Hello "`.

### `service` and `rpc`: define the operation

```proto
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloResponse);
}
```

This declares a unary RPC: one request and one response. It defines the method signature, but does not define how to build the greeting. Your Go method supplies that behavior.

The schema contains no HTTP annotation. The Makefile enables a default gateway mapping, which gives this operation the HTTP route `POST /proto.Greeter/SayHello`. See the [gateway mapping documentation](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/grpc_api_configuration/) for the default mapping rules.

## 3. Install the compiler and plugins

### Compiler versus plugin versus runtime dependency

These serve different purposes:

| Component | When it runs | Purpose |
| --- | --- | --- |
| `protoc` | During generation | Reads and checks `.proto` files and invokes generators |
| Generator plugins | During generation | Produce Go source files from the schema |
| Go runtime libraries | During build and execution | Support protobuf messages, gRPC networking, and HTTP gateway behavior |

Installing a Go plugin does not install `protoc`. Installing a plugin also does not automatically add its runtime dependencies to your application's `go.mod`.

### Install prerequisites

Install Go compatible with the version declared in this project's `go.mod` (currently `1.27.1`). On macOS with Homebrew, install the protobuf compiler and Make if needed:

```bash
brew install protobuf
# macOS developer tools commonly already provide make.
make --version
go version
protoc --version
```

For other operating systems, follow the official [protobuf compiler installation instructions](https://grpc.io/docs/protoc-installation/).

### Use the existing `setup` Make target

The repository already has the installation target:

```makefile
setup:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
```

Run it once, and again whenever you intentionally change the pinned plugin versions:

```bash
make setup
```

The three recipe lines run the same commands you could enter individually in your terminal. Exact version pins keep the selected plugin versions stable between installations.

The gateway generator is pinned to `v2.29.0`, while this project's gateway runtime in `go.mod` is currently `v2.31.0`. They are separate components with separate versions. When upgrading them, regenerate and verify the application together. The `protoc` version is not pinned by this Makefile.

Go places installed executables in `GOBIN` if set, otherwise in `GOPATH/bin`. Inspect these values with:

```bash
go env GOBIN GOPATH
```

Your shell profile may already put that directory on `PATH`. The Makefile also exports it for the commands Make runs. Make does not source your Bash profile itself; it inherits the environment from the shell that launched it.

See the official [gRPC Go setup guide](https://grpc.io/docs/languages/go/quickstart/) for the Go plugin installation workflow.

## 4. Generate code and understand every output

Run:

```bash
make proto
```

This executes:

```bash
mkdir -p grpc
protoc -I proto --go_out=grpc --go_opt=paths=source_relative \
  --go-grpc_out=grpc --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=grpc --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=true \
  hello.proto
```

`-I proto` tells the compiler where to search for schemas, so it finds `hello.proto` inside `proto/`. Each `--..._out` selects a plugin and an output directory. Each `--..._opt` passes settings to that plugin. `paths=source_relative` preserves the schema's relative path beneath that plugin's output directory.

`protoc` discovers the plugins by executable name on `PATH`:

| Command flag | Plugin executable | Generated file |
| --- | --- | --- |
| `--go_out` | `protoc-gen-go` | `grpc/hello.pb.go` |
| `--go-grpc_out` | `protoc-gen-go-grpc` | `grpc/hello_grpc.pb.go` |
| `--grpc-gateway_out` | `protoc-gen-grpc-gateway` | `grpc/hello.pb.gw.go` |

### `hello.pb.go`: the message model

This file gives Go code representations of `HelloRequest` and `HelloResponse`. In this project, those include exported fields and getters:

```go
request := &pb.HelloRequest{InpName: "Soumen"}
name := request.GetInpName()
response := &pb.HelloResponse{OutMessage: "Hello " + name}
```

It also supplies protobuf descriptors and runtime integration, allowing protobuf libraries to inspect and serialize those messages. Descriptors are metadata about the schema, including message names and fields. They are not your greeting business logic.

Why required: the application and other generated files need the concrete request and response types. Handwritten structs with similar fields would not automatically provide equivalent protobuf behavior. See the [Go generated code guide](https://protobuf.dev/reference/go/go-generated/) for the generated message API.

### `hello_grpc.pb.go`: the gRPC contract and transport glue

Important symbols in this file include:

- `GreeterClient` and `NewGreeterClient`: let a Go client invoke the remote service through a gRPC connection.
- `GreeterServer`: the interface your service implementation must satisfy.
- `UnimplementedGreeterServer`: supplies default methods returning an unimplemented error and supports forward compatibility as methods are added.
- `RegisterGreeterServer`: connects your implementation to a gRPC server.
- Internal method handlers and the service descriptor: let gRPC dispatch an incoming method call to the right Go method.

Why required: message types alone do not tell the gRPC server which operation to run. This file connects the service definition to gRPC's dispatch machinery.

Generating it does not start a server or fill in your application behavior. `main.go` still needs to register a service and call `Serve`.

### `hello.pb.gw.go`: the HTTP/JSON adapter

This file registers the HTTP route, decodes its JSON body into a protobuf request, calls the service, and writes the response using the gateway runtime's JSON and error handling.

For this project:

```text
POST /proto.Greeter/SayHello
{"inpName":"Soumen"}
        |
        v
HelloRequest -> SayHello -> HelloResponse
        |
        v
{"outMessage":"Hello Soumen"}
```

Why required: the gRPC server on port 44444 does not accept an ordinary JSON POST from `curl`. The gateway provides that HTTP interface.

It offers several ways to connect the HTTP handler to your service:

| Registration function | Calls the service through |
| --- | --- |
| `RegisterGreeterHandlerServer` | An existing Go server implementation directly; used here |
| `RegisterGreeterHandlerFromEndpoint` | A gRPC connection created for an endpoint address |
| `RegisterGreeterHandler` | An existing gRPC client connection |
| `RegisterGreeterHandlerClient` | An existing generated client implementation |

The gateway file is required for this REST setup, but not for a gRPC-only application. Likewise, a program that only stores protobuf messages may need message code without any service code.

`generate_unbound_methods=true` tells the gateway generator to generate HTTP handlers even though our RPC has no HTTP annotation. Without that option or an explicit mapping, you would not get this route.

### Treat generated files as build outputs

Edit `proto/hello.proto`, then run `make proto`. Do not manually edit generated files: regeneration overwrites your edits.

This repository keeps them locally and ignores them in Git:

```gitignore
/grpc/*.pb.go
/grpc/*.pb.gw.go
```

A fresh clone therefore needs generation before it can build. `make run` does not automatically invoke `make proto`.

## 5. Understand the Makefile

A Makefile is a collection of named targets. The basic shape is:

```makefile
target: prerequisites
	command
```

Recipe lines begin with a real tab, not spaces. Prerequisites name things that must be handled before the target. Our current targets do not declare dependencies on each other, so you choose their order explicitly.

### Variables and environment

```makefile
GO_BIN_DIR := $(shell go env GOBIN)
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif
export PATH := $(GO_BIN_DIR):$(PATH)
```

`$(shell ...)` captures the output of a shell command. `:=` assigns the value immediately. `ifeq` checks whether `GOBIN` is empty and selects the fallback. `export` makes the resulting `PATH` available to recipe commands.

### Phony targets

```makefile
.PHONY: setup proto run clean gofmt
```

These targets represent actions rather than files. Marking them phony ensures they run even if a file or directory with the same name exists. This matters especially for `proto`, because a `proto/` directory already exists.

### What each target does

| Command | Action | When to use it |
| --- | --- | --- |
| `make setup` | Installs all three Go generator plugins | First setup or intentional tool upgrades |
| `make proto` | Generates all three Go files | First setup and after schema changes |
| `make run` | Runs `go run .` | Start the application |
| `make gofmt` | Formats root and generated Go files | After editing Go code; generated files must exist |
| `make clean` | Removes generated files locally | When you want to regenerate from scratch |

Backslashes in the `protoc` recipe continue one shell command across multiple lines. The two separate `go install` lines run in sequence; if one fails, Make stops instead of continuing to the next recipe line.

There is no `test` target currently. Use `go test ./...` directly.

## 6. Understand every section of `main.go`

### Package and imports

`package main` declares an executable program. Go starts execution in its `main()` function.

| Import | Used for |
| --- | --- |
| `context` | Passing request context and a context for gateway registration |
| `pb "grpc-server-go/grpc"` | Generated messages and service registration functions |
| `log` | Startup messages and fatal errors |
| `net` | Opening the gRPC TCP listener |
| `net/http` | Running the HTTP server |
| `github.com/grpc-ecosystem/grpc-gateway/v2/runtime` | The gateway HTTP router and runtime |
| `google.golang.org/grpc` | Creating and serving the gRPC server |

### The service implementation

```go
type Server struct {
    pb.UnimplementedGreeterServer
}
```

Embedding `UnimplementedGreeterServer` provides the generated defaults and satisfies the embedding requirement of the generated server interface. Your explicit `SayHello` method supplies the real implementation for that operation.

### The business method

```go
func (s *Server) SayHello(
    ctx context.Context,
    req *pb.HelloRequest,
) (*pb.HelloResponse, error) {
    response := &pb.HelloResponse{
        OutMessage: "Hello " + req.GetInpName(),
    }
    return response, nil
}
```

- `(s *Server)` is the receiver: this function is a method on `Server`.
- `ctx` can carry cancellation, deadlines, and request information. This simple method does not use it yet.
- `req` contains the decoded request.
- `GetInpName()` reads the name using the generated getter.
- `OutMessage` is the generated Go field corresponding to `out_message`.
- Returning `nil` for the error means the operation succeeded.

This method is shared by gRPC and REST. There is no need to implement the greeting twice.

### Create and register the gRPC service

```go
server := &Server{}

lis, err := net.Listen("tcp", ":44444")
if err != nil {
    log.Fatal(err)
}

grpcSrv := grpc.NewServer()
pb.RegisterGreeterServer(grpcSrv, server)
```

`net.Listen` opens a TCP listener. `grpc.NewServer` creates the gRPC server object. `RegisterGreeterServer` tells that server which implementation handles Greeter requests.

Creating and registering the server does not yet start its request-serving loop.

### Create and register the HTTP gateway

```go
mux := runtime.NewServeMux()
err = pb.RegisterGreeterHandlerServer(
    context.Background(),
    mux,
    server,
)
if err != nil {
    log.Fatal(err)
}
```

The mux is an HTTP router: it matches incoming HTTP methods and paths to handlers. The generated registration function installs this service's route into that router.

`context.Background()` supplies a root context for registration. Incoming HTTP requests have their own request contexts when the handler runs.

Because we pass `server` directly, REST calls bypass the gRPC network transport and its interceptors. If you later add authentication or logging only in a gRPC interceptor, it will not automatically cover this REST path.

### Run both servers

```go
go func() {
    log.Println("gRPC server listening on :44444")
    if err := grpcSrv.Serve(lis); err != nil {
        log.Fatal(err)
    }
}()

log.Println("REST server listening on :8080")
if err := http.ListenAndServe(":8080", mux); err != nil {
    log.Fatal(err)
}
```

Both serving functions block while they serve requests. The `go` keyword starts gRPC's serving loop in a goroutine, allowing `main` to continue to the HTTP server.

The final `()` immediately invokes the anonymous function. `http.ListenAndServe` uses `mux` to handle HTTP requests and keeps the main goroutine running.

The addresses `:44444` and `:8080` listen on available interfaces, not just localhost. This learning example uses plaintext connections and exits immediately on fatal errors; it does not yet implement graceful shutdown, TLS, or server timeout configuration.

## 7. Build and test step by step

### First run after cloning

Once Go, Make, and `protoc` are installed:

```bash
make setup
make proto
go mod download
go test ./...
make run
```

The Go build can download missing dependencies automatically; `go mod download` makes that step explicit. Keep the server terminal open while running the requests below in another terminal.

### Compile checks

```bash
go test ./...
```

This compiles and tests every Go package under the module. With no test files, it prints `[no test files]`: that confirms compilation, but it does not prove that HTTP or gRPC requests succeed.

### Test REST with curl

```bash
curl -i -X POST http://localhost:8080/proto.Greeter/SayHello \
  -H 'Content-Type: application/json' \
  -d '{"inpName":"Soumen"}'
```

Expect an HTTP 200 response and this JSON body:

```json
{"outMessage":"Hello Soumen"}
```

The protobuf JSON mapping uses camel case by default: `inp_name` becomes `inpName`, and `out_message` becomes `outMessage`. JSON formatting and whitespace may differ without changing the response's meaning.

Try another name to confirm that your Go method is using the request value:

```bash
curl -sS -X POST http://localhost:8080/proto.Greeter/SayHello \
  -H 'Content-Type: application/json' \
  -d '{"inpName":"Alex"}'
```

Expected greeting: `Hello Alex`.

Then send an empty object:

```bash
curl -sS -X POST http://localhost:8080/proto.Greeter/SayHello \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Expected greeting: `Hello `, including the trailing space. This demonstrates the string's default value and the absence of application validation.

To check decoding errors, send malformed JSON:

```bash
curl -i -X POST http://localhost:8080/proto.Greeter/SayHello \
  -H 'Content-Type: application/json' \
  -d '{'
```

Expect HTTP 400 with a gateway error body rather than a greeting.

### Test gRPC independently

`curl` testing the REST endpoint does not verify the gRPC listener. Use `grpcurl`, a command-line gRPC client. On macOS:

```bash
brew install grpcurl
```

Then call the service using the local schema:

```bash
grpcurl -plaintext \
  -import-path proto \
  -proto hello.proto \
  -d '{"inpName":"Soumen"}' \
  localhost:44444 proto.Greeter/SayHello
```

Expect a response whose `outMessage` is `Hello Soumen`.

`-plaintext` matches this server's lack of TLS. Supplying the `.proto` file lets the client learn the API without server reflection, which this application has not enabled. See the [grpcurl documentation](https://github.com/fullstorydev/grpcurl) for its schema and invocation options.

### Exercise the behavior with a Go unit test

For an optional next step, create `main_test.go` beside `main.go`:

```go
package main

import (
    "context"
    pb "grpc-server-go/grpc"
    "testing"
)

func TestSayHello(t *testing.T) {
    server := &Server{}
    response, err := server.SayHello(
        context.Background(),
        &pb.HelloRequest{InpName: "Soumen"},
    )
    if err != nil {
        t.Fatal(err)
    }
    if response == nil {
        t.Fatal("expected a response")
    }
    if got := response.GetOutMessage(); got != "Hello Soumen" {
        t.Fatalf("got %q, want %q", got, "Hello Soumen")
    }
}
```

Run `go test ./...` again. This checks the method's behavior without starting network servers. The curl and grpcurl checks cover the two request paths separately.

Stop the running server with Ctrl+C when finished.

## 8. Common problems

| Symptom | Likely cause and fix |
| --- | --- |
| `protoc: command not found` | Install the protobuf compiler and make its executable available on `PATH` |
| `protoc-gen-...: program not found or is not executable` | Run `make setup`; check `GOBIN`, `GOPATH`, and the plugin directory |
| Generated Go package cannot be imported | Run `make proto` before building; check that `go_package` matches the module and output directory |
| REST route missing after a schema edit | Regenerate with `make proto`, stop the old server, and restart `make run` |
| HTTP 404 | Check the exact path and use `POST`; `/hello` is not the current route |
| Connection refused | Start the server and check the port: REST uses 8080, gRPC uses 44444 |
| `address already in use` | Stop the other process using that port or change the listener address |
| `grpcurl` reports missing reflection support | Include `-import-path proto -proto hello.proto` as shown above |
| `make: ... missing separator` | Replace leading spaces on recipe lines with a real tab |

For everyday work: edit the schema and regenerate when the API changes; edit `main.go` when behavior changes; then run compile checks and exercise both endpoints.
