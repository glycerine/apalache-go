package apalache

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExplorationJSONRPCHealthAndErrors(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		wantCode int
		wantID   any
		status   string
	}{
		{name: "health", method: http.MethodPost, path: "/rpc", body: `{"jsonrpc":"2.0","method":"health","params":{},"id":7}`, wantID: float64(7), status: "OK"},
		{name: "string id", method: http.MethodPost, path: "/rpc", body: `{"jsonrpc":"2.0","method":"health","id":"request"}`, wantID: "request", status: "OK"},
		{name: "unknown", method: http.MethodPost, path: "/rpc", body: `{"jsonrpc":"2.0","method":"missing","id":8}`, wantCode: JSONRPCMethodNotFound, wantID: float64(8)},
		{name: "parse", method: http.MethodPost, path: "/rpc", body: `{`, wantCode: JSONRPCParseError},
		{name: "version", method: http.MethodPost, path: "/rpc", body: `{"jsonrpc":"1.0","method":"health","id":9}`, wantCode: JSONRPCInvalidRequest},
		{name: "method", method: http.MethodGet, path: "/rpc", body: ``, wantCode: JSONRPCInvalidRequest},
		{name: "path", method: http.MethodPost, path: "/other", body: `{"jsonrpc":"2.0","method":"health","id":10}`, wantCode: JSONRPCInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("HTTP response = %d, %q", response.Code, response.Header())
			}
			var document map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			if document["jsonrpc"] != "2.0" || document["id"] != test.wantID {
				t.Fatalf("response envelope = %#v", document)
			}
			if test.wantCode != 0 {
				errorObject, ok := document["error"].(map[string]any)
				if !ok || errorObject["code"] != float64(test.wantCode) {
					t.Fatalf("error response = %#v", document)
				}
				return
			}
			result, ok := document["result"].(map[string]any)
			if !ok || result["status"] != test.status {
				t.Fatalf("health response = %#v", document)
			}
		})
	}
}

func TestExplorationJSONRPCLoadAndDisposeSpec(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	source := `---- MODULE Inc ----
EXTENDS Integers
VARIABLE
  \* @type: Int;
  x
Init == I:: x = 0
Next ==
  \/ A:: x < 3 /\ x' = x + 1
  \/ B:: x > -3 /\ x' = x - 1
Inv1 == I1:: x >= -3
Inv2 == I2:: x' - x = 1 \/ x' - x = -1
====`
	encoded := base64.StdEncoding.EncodeToString([]byte(source))
	load := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q],"invariants":["Inv1","Inv2"]},"id":1}`,
		encoded,
	))
	result, ok := load["result"].(map[string]any)
	if !ok {
		t.Fatalf("loadSpec = %#v", load)
	}
	sessionID, _ := result["sessionId"].(string)
	parameters, _ := result["specParameters"].(map[string]any)
	if sessionID == "" || result["snapshotId"] != float64(0) {
		t.Fatalf("loadSpec identity = %#v", result)
	}
	for name, count := range map[string]int{
		"initTransitions": 1, "nextTransitions": 2, "stateInvariants": 1, "actionInvariants": 1,
	} {
		entries, ok := parameters[name].([]any)
		if !ok || len(entries) != count {
			t.Fatalf("%s = %#v", name, parameters[name])
		}
	}
	assertMetadataLabels(t, parameters["initTransitions"], 0, "I")
	assertMetadataLabels(t, parameters["nextTransitions"], 0, "A")
	assertMetadataLabels(t, parameters["nextTransitions"], 1, "B")
	assertMetadataLabels(t, parameters["stateInvariants"], 0, "I1")
	assertMetadataLabels(t, parameters["actionInvariants"], 0, "I2")

	disposeBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"disposeSpec","params":{"sessionId":%q},"id":2}`,
		sessionID,
	)
	dispose := postJSONRPC(t, handler, disposeBody)
	disposed, _ := dispose["result"].(map[string]any)
	if disposed["sessionId"] != sessionID {
		t.Fatalf("disposeSpec = %#v", dispose)
	}
	missing := postJSONRPC(t, handler, disposeBody)
	errorObject, _ := missing["error"].(map[string]any)
	if errorObject["code"] != float64(JSONRPCSessionMissing) {
		t.Fatalf("second disposeSpec = %#v", missing)
	}
}

func TestExplorationJSONRPCLoadRejectsInvalidParams(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	for _, body := range []string{
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[]},"id":1}`,
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":["not base64"]},"id":2}`,
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":["e30="],"format":"qnt"},"id":3}`,
	} {
		response := postJSONRPC(t, handler, body)
		errorObject, _ := response["error"].(map[string]any)
		if errorObject["code"] != float64(JSONRPCInvalidParams) {
			t.Fatalf("loadSpec invalid params = %#v", response)
		}
	}
}

func postJSONRPC(t *testing.T, handler http.Handler, body string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var document map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func assertMetadataLabels(t *testing.T, raw any, index int, labels ...string) {
	t.Helper()
	entries := raw.([]any)
	entry := entries[index].(map[string]any)
	if entry["index"] != float64(index) {
		t.Fatalf("metadata index = %#v", entry)
	}
	actual := entry["labels"].([]any)
	if len(actual) != len(labels) {
		t.Fatalf("metadata labels = %#v, want %v", actual, labels)
	}
	for labelIndex, label := range labels {
		if actual[labelIndex] != label {
			t.Fatalf("metadata labels = %#v, want %v", actual, labels)
		}
	}
}
