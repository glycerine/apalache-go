package shai

import (
	"context"
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestRemoteRecordModeDefaultsAndAnnotations(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		features     []string
		success      bool
	}{
		{"default_rows", `---- MODULE M ----
R == {[a|->1],[b|->2]}
====`, nil, false},
		{"explicit_legacy", `---- MODULE M ----
R == {[a|->1],[b|->2]}
====`, []string{"no-rows"}, true},
		{"legacy_rejects_row_annotation", `---- MODULE M ----
\* @type: {a: Int};
R == [a|->1]
====`, []string{"no-rows"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "content": tc.source, "format": "tla"}, "features": tc.features})
			response, err := NewCmdExecutorService().Run(context.Background(), &cmd.CmdRequest{Cmd: cmd.Cmd_TYPECHECK, Config: string(data)})
			if err != nil {
				t.Fatal(err)
			}
			if (response.GetSuccess() != "") != tc.success {
				t.Fatalf("response=%v", response)
			}
		})
	}
}
