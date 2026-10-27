package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
	JSONRPCSessionMissing = -32404
)

const maxJSONRPCRequestBytes = 16 << 20

type jsonRPCRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      json.RawMessage `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonRPCResponse struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// ExplorationJSONRPCHandler implements the transport boundary of Scala's
// explorer service. Stateful exploration methods are added to dispatch as the
// replayable Go session engine is ported.
type ExplorationJSONRPCHandler struct{}

func NewExplorationJSONRPCHandler() http.Handler { return ExplorationJSONRPCHandler{} }

func (ExplorationJSONRPCHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	if request.Method != http.MethodPost || request.URL.Path != "/rpc" {
		writeJSONRPCResponse(response, jsonRPCFailure(nil, JSONRPCInvalidRequest, "Invalid JSON-RPC request"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxJSONRPCRequestBytes+1))
	if err != nil {
		writeJSONRPCResponse(response, jsonRPCFailure(nil, JSONRPCParseError, "Parse error"))
		return
	}
	if len(body) > maxJSONRPCRequestBytes {
		writeJSONRPCResponse(response, jsonRPCFailure(nil, JSONRPCInvalidRequest, "JSON-RPC request is too large"))
		return
	}
	parsed, parseErr := parseJSONRPCRequest(body)
	if parseErr != nil {
		writeJSONRPCResponse(response, jsonRPCFailure(nil, parseErr.Code, parseErr.Message))
		return
	}
	result, serviceErr := dispatchExplorationJSONRPC(parsed.Method, parsed.Params)
	if serviceErr != nil {
		writeJSONRPCResponse(response, jsonRPCFailure(parsed.ID, serviceErr.Code, serviceErr.Message))
		return
	}
	writeJSONRPCResponse(response, jsonRPCResponse{Version: "2.0", ID: normalizeJSONRPCID(parsed.ID), Result: result})
}

func parseJSONRPCRequest(body []byte) (jsonRPCRequest, *jsonRPCError) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var request jsonRPCRequest
	if err := decoder.Decode(&request); err != nil {
		return jsonRPCRequest{}, &jsonRPCError{Code: JSONRPCParseError, Message: "Parse error"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return jsonRPCRequest{}, &jsonRPCError{Code: JSONRPCParseError, Message: "Parse error"}
	}
	if request.Version != "2.0" || request.Method == "" || !validJSONRPCID(request.ID) {
		return jsonRPCRequest{}, &jsonRPCError{Code: JSONRPCInvalidRequest, Message: "Invalid JSON-RPC request"}
	}
	return request, nil
}

func validJSONRPCID(id json.RawMessage) bool {
	if len(id) == 0 || bytes.Equal(id, []byte("null")) {
		return true
	}
	var value any
	if err := json.Unmarshal(id, &value); err != nil {
		return false
	}
	switch value.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

func normalizeJSONRPCID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func dispatchExplorationJSONRPC(method string, _ json.RawMessage) (any, *jsonRPCError) {
	switch method {
	case "health":
		return struct {
			Status string `json:"status"`
		}{Status: "OK"}, nil
	default:
		return nil, &jsonRPCError{Code: JSONRPCMethodNotFound, Message: fmt.Sprintf("Method not found: %s", method)}
	}
}

func jsonRPCFailure(id json.RawMessage, code int, message string) jsonRPCResponse {
	return jsonRPCResponse{
		Version: "2.0", ID: normalizeJSONRPCID(id),
		Error: &jsonRPCError{Code: code, Message: message},
	}
}

func writeJSONRPCResponse(output http.ResponseWriter, response jsonRPCResponse) {
	if err := json.NewEncoder(output).Encode(response); err != nil {
		http.Error(output, err.Error(), http.StatusInternalServerError)
	}
}
