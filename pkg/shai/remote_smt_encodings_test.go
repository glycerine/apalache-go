package shai

import (
	"context"
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestRemoteCheckSupportsAllSMTEncodings(t *testing.T) {
	for _, encoding := range []string{"oopsla19", "arrays", "funArrays"} {
		t.Run(encoding, func(t *testing.T) {
			for _, violation := range []bool{false, true} {
				source := "---- MODULE M ----\nInit == TRUE\nNext == TRUE\nInv == TRUE\n===="
				if violation {
					source = remoteMultiErrorSpec
				}
				response, err := NewCmdExecutorService().Run(context.Background(), remoteRequest(t, cmd.Cmd_CHECK, source, map[string]any{"length": 0, "smt-encoding": encoding, "inv": []string{"Inv"}}))
				if err != nil {
					t.Fatal(err)
				}
				if !violation {
					if response.GetSuccess() == "" {
						t.Fatalf("expected success: %v", response)
					}
					continue
				}
				var failure struct {
					Pass string `json:"pass_name"`
					Code int    `json:"exit_code"`
				}
				if err := json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil {
					t.Fatal(err)
				}
				if failure.Code != 12 || failure.Pass != "BoundedChecker" {
					t.Fatalf("expected counterexample: %v", response)
				}
			}
		})
	}
}
