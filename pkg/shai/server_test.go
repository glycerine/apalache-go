package shai

import (
	"context"
	"net"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestServerRegistersBothShaiServices(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := NewServer()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-serveErrors; err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Serve = %v", err)
		}
	})

	ctx := context.Background()
	connection, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	if _, err = cmd.NewCmdExecutorClient(connection).Ping(ctx, &cmd.PingRequest{}); err != nil {
		t.Fatalf("CmdExecutor.Ping = %v", err)
	}
	transitions := trans.NewTransExplorerClient(connection)
	if _, err = transitions.Ping(ctx, &trans.PingRequest{}); err != nil {
		t.Fatalf("TransExplorer.Ping = %v", err)
	}
	opened, err := transitions.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil || opened.GetId() == "" {
		t.Fatalf("TransExplorer.OpenConnection = %#v, %v", opened, err)
	}
}
