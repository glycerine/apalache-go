# Shai Go protocol

This directory contains the Go implementation of the version-one Shai gRPC
services. The protocol definitions remain authoritative in
`../../shai/src/main/protobuf`; generated Go files are checked in so normal
builds do not require `protoc`.

The current generated files use `protoc` 3.19.2, `protoc-gen-go` 1.31.0, and
`protoc-gen-go-grpc` 1.5.1. From the repository root, regenerate them with:

```sh
protoc \
  --proto_path=shai/src/main/protobuf \
  --go_out=pkg/shai/cmdexecutor --go_opt=paths=source_relative \
  --go_opt=McmdExecutor.proto=github.com/glycerine/apalache-go/shai/cmdexecutor \
  --go-grpc_out=pkg/shai/cmdexecutor --go-grpc_opt=paths=source_relative \
  --go-grpc_opt=McmdExecutor.proto=github.com/glycerine/apalache-go/shai/cmdexecutor \
  shai/src/main/protobuf/cmdExecutor.proto

protoc \
  --proto_path=shai/src/main/protobuf \
  --go_out=pkg/shai/transexplorer --go_opt=paths=source_relative \
  --go_opt=MtransExplorer.proto=github.com/glycerine/apalache-go/shai/transexplorer \
  --go-grpc_out=pkg/shai/transexplorer --go-grpc_opt=paths=source_relative \
  --go-grpc_opt=MtransExplorer.proto=github.com/glycerine/apalache-go/shai/transexplorer \
  shai/src/main/protobuf/transExplorer.proto
```

Separate Go packages are required because both protobuf files define
`PingRequest` and `PongResponse` in distinct protobuf namespaces.
