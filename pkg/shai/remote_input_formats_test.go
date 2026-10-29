package shai

import (
	"context"
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestRemoteJSONInputRoundTrip(t *testing.T) {
	service := NewCmdExecutorService()
	parsed, err := service.Run(context.Background(), remoteRequest(t, cmd.Cmd_PARSE, "---- MODULE M ----\nInit == TRUE\nNext == TRUE\n====", nil))
	if err != nil || parsed.GetSuccess() == "" {
		t.Fatalf("parse: %v %v", parsed, err)
	}
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK, cmd.Cmd_CHECK} {
		t.Run(command.String(), func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "content": parsed.GetSuccess(), "format": "json"}, "checker": map[string]any{"length": 0}})
			response, err := service.Run(context.Background(), &cmd.CmdRequest{Cmd: command, Config: string(data)})
			if err != nil || response.GetSuccess() == "" {
				t.Fatalf("round trip: %v %v", response, err)
			}
		})
	}
}

func TestRemoteTypedQuintInput(t *testing.T) {
	const quint = `{"stage":"compiling","modules":[{"id":1,"name":"M","declarations":[{"id":2,"kind":"def","name":"Foo","qualifier":"def","expr":{"id":3,"kind":"bool","value":true}}]}],"types":{"2":{"type":{"kind":"bool"}},"3":{"type":{"kind":"bool"}}},"table":{}}`
	data, _ := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "content": quint, "format": "qnt"}})
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK} {
		response, err := NewCmdExecutorService().Run(context.Background(), &cmd.CmdRequest{Cmd: command, Config: string(data)})
		if err != nil || response.GetSuccess() == "" {
			t.Fatalf("%s: %v %v", command, response, err)
		}
	}
}
