package apalache

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestExplorationOperatorEvaluationFailureIsAnError(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	loaded := postJSONRPC(t, handler, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"loadSpec","params":{"sources":[%q],"exports":["View"]}}`, base64.StdEncoding.EncodeToString([]byte(pendingStateSpec))))
	result, ok := loaded["result"].(map[string]any)
	if !ok {
		t.Fatal(loaded)
	}
	session := result["sessionId"].(string)
	op := func(method, params string) map[string]any {
		return postJSONRPC(t, handler, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"sessionId":%q%s}}`, method, session, params))
	}
	defer op("disposeSpec", "")
	for _, response := range []map[string]any{op("assumeTransition", `,"transitionId":0,"checkEnabled":true`), op("nextStep", "")} {
		if response["error"] != nil {
			t.Fatal(response)
		}
	}
	exhausted := op("nextModel", `,"operator":"View"`)
	if result, ok := exhausted["result"].(map[string]any); !ok || result["hasNext"] != "FALSE" {
		t.Fatal(exhausted)
	}
	for _, operator := range []string{"View", "Missing"} {
		response := op("query", fmt.Sprintf(`,"kinds":["OPERATOR"],"operator":%q`, operator))
		failure, ok := response["error"].(map[string]any)
		if !ok || failure["code"] != float64(JSONRPCInternalError) {
			t.Errorf("%s: expected internal evaluation error, got %v", operator, response)
		}
	}
}
