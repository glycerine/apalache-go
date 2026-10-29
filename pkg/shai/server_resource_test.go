//go:build resource

package shai

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestShaiGRPCRepeatedConcurrentLifecyclesReleaseResources(t *testing.T) {
	testShaiGRPCLifecyclesReleaseResources(t, 8, func(lifecycle int) (net.Listener, string, []grpc.DialOption, error) {
		listener := bufconn.Listen(1 << 20)
		options := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		}
		return listener, fmt.Sprintf("passthrough:///shai-resource-%d", lifecycle), options, nil
	})
}

func TestShaiGRPCRepeatedRealSocketLifecyclesReleaseResources(t *testing.T) {
	testShaiGRPCLifecyclesReleaseResources(t, 16, func(int) (net.Listener, string, []grpc.DialOption, error) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, "", nil, err
		}
		return listener, listener.Addr().String(), []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, nil
	})
}

type shaiLifecycleTransport func(int) (net.Listener, string, []grpc.DialOption, error)

func testShaiGRPCLifecyclesReleaseResources(t *testing.T, lifecycles int, transport shaiLifecycleTransport) {
	t.Helper()
	baselineFDs := shaiFileDescriptorCount(t)
	baselineGoroutines := runtime.NumGoroutine()
	const workers = 8
	for lifecycle := 0; lifecycle < lifecycles; lifecycle++ {
		listener, target, options, err := transport(lifecycle)
		if err != nil {
			t.Fatalf("lifecycle %d listen: %v", lifecycle, err)
		}
		server := NewServer()
		serveErrors := make(chan error, 1)
		go func() { serveErrors <- server.Serve(listener) }()
		connection, err := grpc.NewClient(target, options...)
		if err != nil {
			server.Stop()
			_ = listener.Close()
			t.Fatalf("lifecycle %d connect: %v", lifecycle, err)
		}
		commands := cmd.NewCmdExecutorClient(connection)
		transitions := trans.NewTransExplorerClient(connection)
		errorsFound := make(chan error, workers*2)
		var wait sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wait.Add(1)
			go func(worker int) {
				defer wait.Done()
				if _, callErr := commands.Ping(context.Background(), &cmd.PingRequest{}); callErr != nil {
					errorsFound <- fmt.Errorf("lifecycle %d worker %d command ping: %w", lifecycle, worker, callErr)
				}
				if _, callErr := transitions.Ping(context.Background(), &trans.PingRequest{}); callErr != nil {
					errorsFound <- fmt.Errorf("lifecycle %d worker %d explorer ping: %w", lifecycle, worker, callErr)
				}
			}(worker)
		}
		wait.Wait()
		close(errorsFound)
		for callErr := range errorsFound {
			t.Error(callErr)
		}
		if err = connection.Close(); err != nil {
			t.Errorf("lifecycle %d close connection: %v", lifecycle, err)
		}
		server.GracefulStop()
		_ = listener.Close()
		if err = <-serveErrors; err != nil && err != grpc.ErrServerStopped {
			t.Errorf("lifecycle %d serve: %v", lifecycle, err)
		}
	}
	if t.Failed() {
		return
	}
	runtime.GC()
	debug.FreeOSMemory()
	if got := shaiFileDescriptorCount(t); got != baselineFDs {
		t.Fatalf("file descriptors after Shai lifecycles = %d, want baseline %d", got, baselineFDs)
	}
	if got := runtime.NumGoroutine(); got > baselineGoroutines+4 {
		t.Fatalf("goroutines after Shai lifecycles = %d, baseline %d", got, baselineGoroutines)
	}
}

func shaiFileDescriptorCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("file-descriptor accounting requires /proc/self/fd: %v", err)
	}
	return len(entries)
}
