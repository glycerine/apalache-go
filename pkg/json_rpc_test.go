package apalache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
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

func TestExplorationJSONRPCGzipRequestAndResponse(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	var compressedRequest bytes.Buffer
	requestWriter := gzip.NewWriter(&compressedRequest)
	_, _ = requestWriter.Write([]byte(`{"jsonrpc":"2.0","method":"health","id":1}`))
	if err := requestWriter.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/rpc", &compressedRequest)
	request.Header.Set("Content-Encoding", "gzip")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `"status":"OK"`) {
		t.Fatalf("gzip request response = %q", response.Body.String())
	}

	largeMethod := strings.Repeat("missing", 100)
	request = httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","method":%q,"id":2}`, largeMethod,
	)))
	request.Header.Set("Accept-Encoding", "gzip")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("large response encoding = %q, size=%d", response.Header().Get("Content-Encoding"), response.Body.Len())
	}
	responseReader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(responseReader)
	if err != nil {
		t.Fatal(err)
	}
	_ = responseReader.Close()
	var document map[string]any
	if err = json.Unmarshal(decoded, &document); err != nil {
		t.Fatal(err)
	}
	errorObject, _ := document["error"].(map[string]any)
	if errorObject["code"] != float64(JSONRPCMethodNotFound) {
		t.Fatalf("gzip response = %#v", document)
	}
}

func TestExplorationJSONRPCZstdRequestResponseAndNegotiation(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	var compressedRequest bytes.Buffer
	requestWriter, err := zstd.NewWriter(&compressedRequest, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = requestWriter.Write([]byte(`{"jsonrpc":"2.0","method":"health","id":1}`))
	if err = requestWriter.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/rpc", &compressedRequest)
	request.Header.Set("Content-Encoding", "zstd")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `"status":"OK"`) {
		t.Fatalf("zstd request response = %q", response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(
		`{"jsonrpc":"2.0","method":"health","id":2}`,
	))
	request.Header.Set("Accept-Encoding", "zstd")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if encoding := response.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("small response encoding = %q", encoding)
	}

	largeMethod := strings.Repeat("missing", 100)
	request = httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","method":%q,"id":3}`, largeMethod,
	)))
	request.Header.Set("Accept-Encoding", "gzip;q=0.5, zstd;q=1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Content-Encoding") != "zstd" {
		t.Fatalf("large response encoding = %q", response.Header().Get("Content-Encoding"))
	}
	responseReader, err := zstd.NewReader(response.Body, zstd.WithDecoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(responseReader)
	responseReader.Close()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = json.Unmarshal(decoded, &document); err != nil {
		t.Fatal(err)
	}
	errorObject, _ := document["error"].(map[string]any)
	if errorObject["code"] != float64(JSONRPCMethodNotFound) {
		t.Fatalf("zstd response = %#v", document)
	}

	request = httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","method":%q,"id":4}`, largeMethod,
	)))
	request.Header.Set("Accept-Encoding", "zstd;q=0, gzip;q=0")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if encoding := response.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("zero-quality response encoding = %q", encoding)
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
  \/ C:: x > 10 /\ x' = x
Inv1 == I1:: x >= -3
Inv2 == I2:: x' - x = 1 \/ x' - x = -1
Bad == IBad:: x /= 0
View == x = 0
====`
	encoded := base64.StdEncoding.EncodeToString([]byte(source))
	load := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q],"invariants":["Inv1","Inv2","Bad"],"exports":["View"]},"id":1}`,
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
		"initTransitions": 1, "nextTransitions": 3, "stateInvariants": 2, "actionInvariants": 1,
	} {
		entries, ok := parameters[name].([]any)
		if !ok || len(entries) != count {
			t.Fatalf("%s = %#v", name, parameters[name])
		}
	}
	assertMetadataLabels(t, parameters["initTransitions"], 0, "I")
	assertMetadataLabels(t, parameters["nextTransitions"], 0, "A")
	assertMetadataLabels(t, parameters["nextTransitions"], 1, "B")
	assertMetadataLabels(t, parameters["nextTransitions"], 2, "C")
	assertMetadataLabels(t, parameters["stateInvariants"], 0, "I1")
	assertMetadataLabels(t, parameters["stateInvariants"], 1, "IBad")
	assertMetadataLabels(t, parameters["actionInvariants"], 0, "I2")
	assume := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"assumeTransition","params":{"sessionId":%q,"transitionId":0,"checkEnabled":true},"id":2}`,
		sessionID,
	))
	assumeResult, _ := assume["result"].(map[string]any)
	if assumeResult["status"] != "ENABLED" || assumeResult["transitionId"] != float64(0) {
		t.Fatalf("assumeTransition = %#v", assume)
	}
	next := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"nextStep","params":{"sessionId":%q},"id":3}`,
		sessionID,
	))
	nextResult, _ := next["result"].(map[string]any)
	if nextResult["newStepNo"] != float64(1) {
		t.Fatalf("nextStep = %#v", next)
	}
	violated := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"checkInvariant","params":{"sessionId":%q,"invariantId":1,"kind":"STATE"},"id":4}`,
		sessionID,
	))
	violatedResult, _ := violated["result"].(map[string]any)
	if violatedResult["invariantStatus"] != "VIOLATED" || violatedResult["trace"] == nil {
		t.Fatalf("checkInvariant violation = %#v", violated)
	}
	query := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"query","params":{"sessionId":%q,"kinds":["TRACE","STATE","OPERATOR"],"operator":"View"},"id":4}`,
		sessionID,
	))
	queryResult, _ := query["result"].(map[string]any)
	trace, traceOK := queryResult["trace"].(map[string]any)
	state, stateOK := queryResult["state"].(map[string]any)
	if !traceOK || !stateOK || trace["states"] == nil || state["x"] == nil || queryResult["operatorValue"] != true {
		t.Fatalf("query trace/state = %#v", query)
	}
	assumeState := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"assumeState","params":{"sessionId":%q,"equalities":{"x":1},"checkEnabled":true},"id":4}`,
		sessionID,
	))
	assumeStateResult, _ := assumeState["result"].(map[string]any)
	if assumeStateResult["status"] != "DISABLED" {
		t.Fatalf("assumeState inconsistent = %#v", assumeState)
	}
	queryAfterState := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"query","params":{"sessionId":%q,"kinds":["OPERATOR"],"operator":"View"},"id":4}`,
		sessionID,
	))
	if result, _ := queryAfterState["result"].(map[string]any); result["operatorValue"] != true {
		t.Fatalf("query after assumeState rollback = %#v", queryAfterState)
	}
	nextModel := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"nextModel","params":{"sessionId":%q,"operator":"View"},"id":4}`,
		sessionID,
	))
	nextModelResult, _ := nextModel["result"].(map[string]any)
	if nextModelResult["oldValue"] != true || nextModelResult["hasOld"] != "TRUE" || nextModelResult["hasNext"] != "FALSE" {
		t.Fatalf("nextModel deterministic operator = %#v", nextModel)
	}
	postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"rollback","params":{"sessionId":%q,"snapshotId":%d},"id":4}`,
		sessionID, int(nextResult["snapshotId"].(float64)),
	))
	disabled := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"assumeTransition","params":{"sessionId":%q,"transitionId":2,"checkEnabled":true},"id":4}`,
		sessionID,
	))
	disabledResult, _ := disabled["result"].(map[string]any)
	if disabledResult["status"] != "DISABLED" {
		t.Fatalf("disabled assumeTransition = %#v", disabled)
	}
	enabledAfterDisabled := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"assumeTransition","params":{"sessionId":%q,"transitionId":0,"checkEnabled":true},"id":5}`,
		sessionID,
	))
	if result, _ := enabledAfterDisabled["result"].(map[string]any); result["status"] != "ENABLED" {
		t.Fatalf("assumeTransition after disabled = %#v", enabledAfterDisabled)
	}
	postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"nextStep","params":{"sessionId":%q},"id":6}`,
		sessionID,
	))
	actionSatisfied := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"checkInvariant","params":{"sessionId":%q,"invariantId":0,"kind":"ACTION"},"id":7}`,
		sessionID,
	))
	actionResult, _ := actionSatisfied["result"].(map[string]any)
	if actionResult["invariantStatus"] != "SATISFIED" || actionResult["trace"] != nil {
		t.Fatalf("checkInvariant action = %#v", actionSatisfied)
	}
	compact := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"compact","params":{"sessionId":%q,"snapshotId":0},"id":7}`,
		sessionID,
	))
	compactResult, _ := compact["result"].(map[string]any)
	if compactResult["snapshotId"] == nil {
		t.Fatalf("compact = %#v", compact)
	}
	queryCompacted := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"query","params":{"sessionId":%q,"kinds":["OPERATOR"],"operator":"View"},"id":7}`,
		sessionID,
	))
	if result, _ := queryCompacted["result"].(map[string]any); result["operatorValue"] != false {
		t.Fatalf("query compacted state = %#v", queryCompacted)
	}
	rollback := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"rollback","params":{"sessionId":%q,"snapshotId":0},"id":6}`,
		sessionID,
	))
	rollbackResult, _ := rollback["result"].(map[string]any)
	if rollbackResult["snapshotId"] != float64(0) {
		t.Fatalf("rollback = %#v", rollback)
	}
	batch := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"applyInOrder","params":{"sessionId":%q,"calls":[`+
			`{"method":"assumeTransition","params":{"transitionId":0,"checkEnabled":true}},`+
			`{"method":"nextStep","params":{}},`+
			`{"method":"query","params":{"kinds":["OPERATOR"],"operator":"View"}}]},"id":7}`,
		sessionID,
	))
	batchResult, _ := batch["result"].(map[string]any)
	batchCalls, _ := batchResult["calls"].([]any)
	if len(batchCalls) != 3 {
		t.Fatalf("applyInOrder = %#v", batch)
	}
	for _, rawCall := range batchCalls {
		if call := rawCall.(map[string]any); call["ok"] != true || call["error"] != nil {
			t.Fatalf("applyInOrder call = %#v", call)
		}
	}
	failingBatch := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"applyInOrder","params":{"sessionId":%q,"calls":[`+
			`{"method":"assumeTransition","params":{"transitionId":99,"checkEnabled":true}},`+
			`{"method":"nextStep","params":{}}]},"id":8}`,
		sessionID,
	))
	failingResult, _ := failingBatch["result"].(map[string]any)
	failingCalls, _ := failingResult["calls"].([]any)
	if len(failingCalls) != 1 || failingCalls[0].(map[string]any)["ok"] != false || failingCalls[0].(map[string]any)["error"] == nil {
		t.Fatalf("failing applyInOrder = %#v", failingBatch)
	}

	disposeBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"disposeSpec","params":{"sessionId":%q},"id":8}`,
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

func TestExplorationJSONRPCLoadsAuxiliaryTLAAndCanonicalJSON(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	root := `---- MODULE Root ----
EXTENDS Aux
\* @type: Int;
VARIABLE x
Init == x = Zero
Next == x' = x
====`
	auxiliary := `---- MODULE Aux ----
\* @type: Int;
Zero == 0
====`
	loadAuxiliary := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q,%q]},"id":1}`,
		base64.StdEncoding.EncodeToString([]byte(root)), base64.StdEncoding.EncodeToString([]byte(auxiliary)),
	))
	if result, _ := loadAuxiliary["result"].(map[string]any); result["sessionId"] == nil {
		t.Fatalf("auxiliary TLA load = %#v", loadAuxiliary)
	}

	frontend := ParseAndTypeCheckSource(context.Background(), "Canonical.tla", `---- MODULE Canonical ----
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
====`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("canonical JSON frontend = %v", frontend.Diagnostics)
	}
	canonical, err := MarshalSingleModuleIRJSON(frontend.Native, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	loadJSON := postJSONRPC(t, handler, fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q],"format":"json"},"id":2}`,
		base64.StdEncoding.EncodeToString(canonical),
	))
	result, _ := loadJSON["result"].(map[string]any)
	parameters, _ := result["specParameters"].(map[string]any)
	if result["sessionId"] == nil || len(parameters["initTransitions"].([]any)) != 1 {
		t.Fatalf("canonical JSON load = %#v", loadJSON)
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
