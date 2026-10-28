package shai

import (
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc"
)

const MaxInboundMessageSize = 1 << 30

func NewServer(options ...grpc.ServerOption) *grpc.Server {
	options = append([]grpc.ServerOption{grpc.MaxRecvMsgSize(MaxInboundMessageSize)}, options...)
	server := grpc.NewServer(options...)
	cmd.RegisterCmdExecutorServer(server, NewCmdExecutorService())
	trans.RegisterTransExplorerServer(server, NewTransExplorerService())
	return server
}
