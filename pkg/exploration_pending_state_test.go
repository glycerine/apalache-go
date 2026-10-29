package apalache

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

const pendingStateSpec = `---- MODULE Pending ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == x=0
Next == x'=x+1
View == x
====`

func TestExplorationAssertionPreservesPendingDestination(t *testing.T) {
	for _, batched := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "batched"}[batched], func(t *testing.T) {
			handler := NewExplorationJSONRPCHandler()
			call := func(method string, params map[string]any) map[string]any {
				t.Helper()
				data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
				if err != nil {
					t.Fatal(err)
				}
				response := postJSONRPC(t, handler, string(data))
				result, ok := response["result"].(map[string]any)
				if !ok {
					t.Fatalf("%s: %v", method, response)
				}
				return result
			}
			loaded := call("loadSpec", map[string]any{"sources": []string{base64.StdEncoding.EncodeToString([]byte(pendingStateSpec))}, "exports": []string{"View"}})
			session := loaded["sessionId"]
			op := func(method string, params map[string]any) map[string]any {
				params["sessionId"] = session
				return call(method, params)
			}
			defer op("disposeSpec", map[string]any{})
			op("assumeTransition", map[string]any{"transitionId": 0, "checkEnabled": true})
			op("nextStep", map[string]any{})
			op("assumeTransition", map[string]any{"transitionId": 0, "checkEnabled": true})
			assertion := map[string]any{"equalities": map[string]any{"x": 0}, "checkEnabled": true}
			if batched {
				op("applyInOrder", map[string]any{"calls": []any{map[string]any{"method": "assumeState", "params": assertion}}})
			} else {
				op("assumeState", assertion)
			}
			check := func(length int) {
				result := op("query", map[string]any{"kinds": []string{"STATE", "TRACE"}})
				state, ok := result["state"].(map[string]any)
				if !ok || state["x"].(map[string]any)["#bigint"] != "1" {
					t.Errorf("pending destination lost: %v", result)
				}
				trace, ok := result["trace"].(map[string]any)
				if !ok {
					t.Fatal(result)
				}
				if states, ok := trace["states"].([]any); !ok || len(states) != length {
					t.Errorf("trace: %v", trace)
				}
			}
			check(2)
			op("compact", map[string]any{"snapshotId": loaded["snapshotId"]})
			check(1)
		})
	}
}
