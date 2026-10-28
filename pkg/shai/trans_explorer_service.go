package shai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	pal "github.com/glycerine/apalache-go"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var shaiModuleHeader = regexp.MustCompile(`(?m)-+\s*MODULE\s+([A-Za-z_][A-Za-z0-9_]*)\s*-+`)
var shaiUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type TransExplorerService struct {
	trans.UnimplementedTransExplorerServer
	mu          sync.RWMutex
	connections map[string]string
}

func NewTransExplorerService() *TransExplorerService {
	return &TransExplorerService{connections: make(map[string]string)}
}

func (service *TransExplorerService) Ping(context.Context, *trans.PingRequest) (*trans.PongResponse, error) {
	return &trans.PongResponse{}, nil
}

func (service *TransExplorerService) OpenConnection(context.Context, *trans.ConnectRequest) (*trans.Connection, error) {
	id, err := newShaiUUID()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	service.mu.Lock()
	service.connections[id] = ""
	service.mu.Unlock()
	return &trans.Connection{Id: id}, nil
}

func (service *TransExplorerService) LoadModel(ctx context.Context, request *trans.LoadModelRequest) (*trans.LoadModelResponse, error) {
	if request.GetConn() == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid RPC request: no Connection provided")
	}
	id := request.GetConn().GetId()
	if !shaiUUID.MatchString(id) {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid RPC request: Connection id %s is not a valid UUID", id)
	}
	service.mu.RLock()
	_, found := service.connections[id]
	service.mu.RUnlock()
	if !found {
		return nil, status.Errorf(codes.FailedPrecondition, "Invalid Connection: no connection is registered for id %s", id)
	}
	frontend, err := parseShaiSources(ctx, request.GetSpec(), request.GetAux())
	if err != nil {
		return &trans.LoadModelResponse{Result: &trans.LoadModelResponse_Err{Err: &trans.TransExplorerError{
			ErrorType: trans.TransExplorerErrorType_UNEXPECTED, Data: jsonError(err),
		}}}, nil
	}
	if !frontend.OK() {
		data, _ := json.Marshal(frontend.Diagnostics)
		return &trans.LoadModelResponse{Result: &trans.LoadModelResponse_Err{Err: &trans.TransExplorerError{
			ErrorType: trans.TransExplorerErrorType_PASS_FAILURE, Data: string(data),
		}}}, nil
	}
	spec := string(frontend.IR)
	service.mu.Lock()
	service.connections[id] = spec
	service.mu.Unlock()
	return &trans.LoadModelResponse{Result: &trans.LoadModelResponse_Spec{Spec: spec}}, nil
}

func parseShaiSources(ctx context.Context, root string, auxiliary []string) (pal.FrontendResult, error) {
	directory, err := os.MkdirTemp("", "apalache-shai-")
	if err != nil {
		return pal.FrontendResult{}, err
	}
	defer os.RemoveAll(directory)
	sources := append([]string{root}, auxiliary...)
	rootPath := ""
	for index, source := range sources {
		match := shaiModuleHeader.FindStringSubmatch(source)
		name := fmt.Sprintf("__ShaiSource%d", index)
		if len(match) == 2 {
			name = match[1]
		}
		path := filepath.Join(directory, name+".tla")
		if err = os.WriteFile(path, []byte(source), 0o600); err != nil {
			return pal.FrontendResult{}, err
		}
		if index == 0 {
			rootPath = path
		}
	}
	return pal.ParseFile(ctx, rootPath, pal.FrontendOptions{}), nil
}

func newShaiUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func jsonError(err error) string {
	encoded, _ := json.Marshal(map[string]string{"msg": err.Error()})
	return string(encoded)
}
