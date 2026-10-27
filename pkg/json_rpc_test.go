package apalache

import (
	"encoding/json"
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
