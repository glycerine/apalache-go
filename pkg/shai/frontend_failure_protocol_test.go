package shai

import (
	"context"
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
)

func assertFrontendFailure(t *testing.T, data string, typing bool) {
	t.Helper()
	var failure struct {
		Pass string            `json:"pass_name"`
		Code int               `json:"exit_code"`
		Data []json.RawMessage `json:"error_data"`
	}
	if err := json.Unmarshal([]byte(data), &failure); err != nil {
		t.Fatal(err)
	}
	pass, code := "SanyParser", 255
	if typing {
		pass, code = "TypeCheckerSnowcat", 120
	}
	if failure.Pass != pass || failure.Code != code || len(failure.Data) == 0 {
		t.Fatalf("wrong failure envelope: %s", data)
	}
	for _, entry := range failure.Data {
		if typing {
			var pair []string
			if err := json.Unmarshal(entry, &pair); err != nil || len(pair) != 2 || pair[0] == "" || pair[1] == "" {
				t.Fatalf("want [location,message], got %s", entry)
			}
		} else {
			var message string
			if err := json.Unmarshal(entry, &message); err != nil || message == "" {
				t.Fatalf("want parser message, got %s", entry)
			}
		}
	}
}

func TestFrontendFailureProtocolPreservesStageAndShape(t *testing.T) {
	const malformed = "---- MODULE Bad ----\nO == (\n====\n"
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK, cmd.Cmd_CHECK, cmd.Cmd_TLA} {
		t.Run(command.String(), func(t *testing.T) {
			response, err := NewCmdExecutorService().Run(context.Background(), remoteRequest(t, command, malformed, nil))
			if err != nil {
				t.Fatal(err)
			}
			assertFrontendFailure(t, response.GetFailure().GetData(), false)
		})
	}
	t.Run("type_error", func(t *testing.T) {
		response, err := NewCmdExecutorService().Run(context.Background(), remoteRequest(t, cmd.Cmd_TYPECHECK, remoteIllTypedSpec, nil))
		if err != nil {
			t.Fatal(err)
		}
		assertFrontendFailure(t, response.GetFailure().GetData(), true)
	})
	t.Run("load_model", func(t *testing.T) {
		service := NewTransExplorerService()
		connection, err := service.OpenConnection(context.Background(), &trans.ConnectRequest{})
		if err != nil {
			t.Fatal(err)
		}
		response, err := service.LoadModel(context.Background(), &trans.LoadModelRequest{Conn: connection, Spec: malformed})
		if err != nil {
			t.Fatal(err)
		}
		assertFrontendFailure(t, response.GetErr().GetData(), false)
	})
}

func TestRemoteMalformedJSONPreservesParserFailureShape(t *testing.T) {
	data, _ := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "content": "{", "format": "json"}})
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK} {
		response, err := NewCmdExecutorService().Run(context.Background(), &cmd.CmdRequest{Cmd: command, Config: string(data)})
		if err != nil {
			t.Fatal(err)
		}
		var failure struct {
			Pass string `json:"pass_name"`
			Code int    `json:"exit_code"`
			Data string `json:"error_data"`
		}
		if err := json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil {
			t.Fatal(err)
		}
		if failure.Pass != "SanyParser" || failure.Code != 150 || failure.Data == "" {
			t.Fatalf("wrong JSON parse failure: %v", response)
		}
	}
}
