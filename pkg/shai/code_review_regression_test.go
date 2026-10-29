package shai

import (
	"context"
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestReviewRemoteCheckUnsatisfiableInit(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0 /\ x > 0
Next == x' = x
====`
	response, err := NewCmdExecutorService().Run(context.Background(), remoteRequest(t, cmd.Cmd_CHECK, source,
		map[string]any{"init": "Init", "next": "Next", "length": 1, "no-deadlock": false}))
	if err != nil {
		t.Fatal(err)
	}
	if response.GetFailure() == nil {
		t.Fatalf("unsatisfiable Init returned success: %+v", response)
	}
	var data struct {
		ErrorData struct {
			Result          string            `json:"checking_result"`
			Counterexamples []json.RawMessage `json:"counterexamples"`
		} `json:"error_data"`
	}
	if err := json.Unmarshal([]byte(response.GetFailure().GetData()), &data); err != nil {
		t.Fatal(err)
	}
	if data.ErrorData.Result != "Deadlock" || data.ErrorData.Counterexamples == nil || len(data.ErrorData.Counterexamples) != 0 {
		t.Fatalf("expected Deadlock without a fabricated trace: %s", response.GetFailure().GetData())
	}
}
