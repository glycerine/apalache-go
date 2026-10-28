package shai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTransExplorerConnectionAndModelLoading(t *testing.T) {
	service := NewTransExplorerService()
	ctx := context.Background()
	if _, err := service.Ping(ctx, &trans.PingRequest{}); err != nil {
		t.Fatal(err)
	}
	connection, err := service.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil || !shaiUUID.MatchString(connection.GetId()) {
		t.Fatalf("connection = %#v, %v", connection, err)
	}
	second, err := service.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil || second.GetId() == connection.GetId() {
		t.Fatalf("second connection = %#v, %v", second, err)
	}
	response, err := service.LoadModel(ctx, &trans.LoadModelRequest{Conn: connection, Spec: `---- MODULE Root ----
EXTENDS Aux
Value == AuxValue
====`, Aux: []string{`---- MODULE Aux ----
AuxValue == 1
====`}})
	if err != nil || response.GetSpec() == "" {
		t.Fatalf("LoadModel = %#v, %v", response, err)
	}
	var document map[string]any
	if err = json.Unmarshal([]byte(response.GetSpec()), &document); err != nil || document["name"] != "ApalacheIR" {
		t.Fatalf("loaded IR = %q, %v", response.GetSpec(), err)
	}
	service.mu.RLock()
	loaded := service.connections[connection.GetId()]
	service.mu.RUnlock()
	if loaded != response.GetSpec() {
		t.Fatalf("connection model = %q, want %q", loaded, response.GetSpec())
	}
}

func TestTransExplorerValidatesConnectionsAndReportsParseFailures(t *testing.T) {
	service := NewTransExplorerService()
	ctx := context.Background()
	if _, err := service.LoadModel(ctx, &trans.LoadModelRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing connection = %v", err)
	}
	if _, err := service.LoadModel(ctx, &trans.LoadModelRequest{Conn: &trans.Connection{Id: "bad"}}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid connection = %v", err)
	}
	unknown := &trans.Connection{Id: "00000000-0000-4000-8000-000000000000"}
	if _, err := service.LoadModel(ctx, &trans.LoadModelRequest{Conn: unknown}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unknown connection = %v", err)
	}
	uppercase := &trans.Connection{Id: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}
	if _, err := service.LoadModel(ctx, &trans.LoadModelRequest{Conn: uppercase}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("uppercase UUID = %v", err)
	}
	connection, err := service.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.LoadModel(ctx, &trans.LoadModelRequest{Conn: connection, Spec: "not a module"})
	if err != nil || response.GetErr() == nil || response.GetErr().GetErrorType() != trans.TransExplorerErrorType_PASS_FAILURE {
		t.Fatalf("invalid source = %#v, %v", response, err)
	}
	response, err = service.LoadModel(ctx, &trans.LoadModelRequest{Conn: connection, Spec: "---- MODULE Bad ----\nValue == (1 + )\n===="})
	if err != nil || response.GetErr() == nil || response.GetErr().GetErrorType() != trans.TransExplorerErrorType_PASS_FAILURE {
		t.Fatalf("parse failure = %#v, %v", response, err)
	}
}

func TestTransExplorerLoadsModelsSequentiallyAndConcurrently(t *testing.T) {
	service := NewTransExplorerService()
	ctx := context.Background()
	connection, err := service.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"First", "Second"} {
		request := &trans.LoadModelRequest{Conn: connection, Spec: "---- MODULE " + name + " ----\nValue == TRUE\n===="}
		if response, loadErr := service.LoadModel(ctx, request); loadErr != nil || response.GetSpec() == "" {
			t.Fatalf("sequential %s = %#v, %v", name, response, loadErr)
		}
	}

	connections := make([]*trans.Connection, 2)
	for index := range connections {
		connections[index], err = service.OpenConnection(ctx, &trans.ConnectRequest{})
		if err != nil {
			t.Fatal(err)
		}
	}
	errors := make(chan error, len(connections))
	for index, conn := range connections {
		go func(index int, conn *trans.Connection) {
			name := fmt.Sprintf("Parallel%d", index)
			response, loadErr := service.LoadModel(ctx, &trans.LoadModelRequest{
				Conn: conn,
				Spec: "---- MODULE " + name + " ----\nValue == TRUE\n====",
			})
			if loadErr == nil && response.GetSpec() == "" {
				loadErr = fmt.Errorf("empty model response")
			}
			errors <- loadErr
		}(index, conn)
	}
	for range connections {
		if err = <-errors; err != nil {
			t.Fatal(err)
		}
	}
}
