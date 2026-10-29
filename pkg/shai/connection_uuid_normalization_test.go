package shai

import (
	"context"
	"strings"
	"testing"

	trans "github.com/glycerine/apalache-go/shai/transexplorer"
)

func TestConnectionUUIDLookupIsCaseInsensitive(t *testing.T) {
	service := NewTransExplorerService()
	connection, err := service.OpenConnection(context.Background(), &trans.ConnectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{strings.ToUpper(connection.GetId()), connection.GetId()} {
		response, err := service.LoadModel(context.Background(), &trans.LoadModelRequest{Conn: &trans.Connection{Id: id}, Spec: remoteTrivialSpec})
		if err != nil || response.GetSpec() == "" {
			t.Fatalf("load %s: %v %v", id, response, err)
		}
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	if len(service.connections) != 1 || service.connections[connection.GetId()] == "" {
		t.Fatalf("connection was duplicated or not updated: %v", service.connections)
	}
}
