package apalache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
	JSONRPCServerErrorMin = -32099
	JSONRPCServerErrorMax = -32000
	JSONRPCSessionMissing = -32404
)

const maxJSONRPCRequestBytes = 16 << 20
const minJSONRPCCompressBytes = 512

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
// stateful explorer service.
type ExplorationJSONRPCHandler struct{ service *ExplorationService }

func NewExplorationJSONRPCHandler() http.Handler {
	service := &ExplorationJSONRPCHandler{service: NewExplorationService()}
	return explorationCompressionHandler{next: service}
}

type explorationCompressionHandler struct{ next http.Handler }

func (handler explorationCompressionHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	contentEncoding := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Encoding")))
	if contentEncoding == "gzip" || contentEncoding == "zstd" {
		reader, err := newJSONRPCCompressedReader(contentEncoding, request.Body)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			writeJSONRPCResponse(response, jsonRPCFailure(nil, JSONRPCParseError, "Parse error"))
			return
		}
		defer reader.Close()
		request.Body = reader
	}
	responseEncoding := acceptedJSONRPCEncoding(request.Header.Get("Accept-Encoding"))
	if responseEncoding == "" {
		handler.next.ServeHTTP(response, request)
		return
	}
	buffered := newBufferedJSONRPCResponse()
	handler.next.ServeHTTP(buffered, request)
	copyHTTPHeaders(response.Header(), buffered.header)
	if buffered.body.Len() < minJSONRPCCompressBytes {
		response.WriteHeader(buffered.status)
		_, _ = response.Write(buffered.body.Bytes())
		return
	}
	response.Header().Set("Content-Encoding", responseEncoding)
	response.Header().Add("Vary", "Accept-Encoding")
	response.Header().Del("Content-Length")
	response.WriteHeader(buffered.status)
	writer, err := newJSONRPCCompressedWriter(responseEncoding, response)
	if err != nil {
		return
	}
	_, _ = writer.Write(buffered.body.Bytes())
	_ = writer.Close()
}

func newJSONRPCCompressedReader(encoding string, input io.Reader) (io.ReadCloser, error) {
	if encoding == "gzip" {
		return gzip.NewReader(input)
	}
	decoder, err := zstd.NewReader(input, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(64<<20))
	if err != nil {
		return nil, err
	}
	return decoder.IOReadCloser(), nil
}

func newJSONRPCCompressedWriter(encoding string, output io.Writer) (io.WriteCloser, error) {
	if encoding == "gzip" {
		return gzip.NewWriter(output), nil
	}
	return zstd.NewWriter(output, zstd.WithEncoderConcurrency(1))
}

type bufferedJSONRPCResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedJSONRPCResponse() *bufferedJSONRPCResponse {
	return &bufferedJSONRPCResponse{header: make(http.Header), status: http.StatusOK}
}

func (response *bufferedJSONRPCResponse) Header() http.Header { return response.header }
func (response *bufferedJSONRPCResponse) WriteHeader(status int) {
	response.status = status
}
func (response *bufferedJSONRPCResponse) Write(data []byte) (int, error) {
	return response.body.Write(data)
}

func acceptedJSONRPCEncoding(header string) string {
	selected := ""
	selectedQuality := 0.0
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(value, ";")
		encoding := strings.ToLower(strings.TrimSpace(parts[0]))
		if encoding != "gzip" && encoding != "zstd" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			name, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(strings.TrimSpace(name), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		if quality > selectedQuality {
			selected, selectedQuality = encoding, quality
		}
	}
	return selected
}

func copyHTTPHeaders(destination, source http.Header) {
	for name, values := range source {
		destination[name] = append([]string(nil), values...)
	}
}

func (handler *ExplorationJSONRPCHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
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
	result, serviceErr := handler.dispatch(request.Context(), parsed.Method, parsed.Params)
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

func (handler *ExplorationJSONRPCHandler) dispatch(ctx context.Context, method string, params json.RawMessage) (any, *jsonRPCError) {
	switch method {
	case "health":
		return struct {
			Status string `json:"status"`
		}{Status: "OK"}, nil
	case "loadSpec":
		return handler.service.loadSpec(ctx, params)
	case "disposeSpec":
		return handler.service.disposeSpec(params)
	case "assumeTransition":
		return handler.service.assumeTransition(ctx, params)
	case "nextStep":
		return handler.service.nextStep(ctx, params)
	case "assumeState":
		return handler.service.assumeState(ctx, params)
	case "rollback":
		return handler.service.rollback(ctx, params)
	case "checkInvariant":
		return handler.service.checkInvariant(ctx, params)
	case "query":
		return handler.service.query(ctx, params)
	case "nextModel":
		return handler.service.nextModel(ctx, params)
	case "compact":
		return handler.service.compact(ctx, params)
	case "applyInOrder":
		return handler.service.applyInOrder(ctx, params)
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
