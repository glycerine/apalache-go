//go:build resource

package apalache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

func TestExplorationJSONRPCConcurrentLoadSolveDisposeReleasesResources(t *testing.T) {
	baselineFDs := processFileDescriptorCount(t)
	baselineGoroutines := runtime.NumGoroutine()
	service := NewExplorationService()
	handler := &ExplorationJSONRPCHandler{service: service}
	source := base64.StdEncoding.EncodeToString([]byte(`---- MODULE ResourceLoad ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == x = 0
Next == x' = x + 1
====`))

	const workers = 4
	const iterations = 8
	errorsFound := make(chan error, workers*iterations)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				requestID := worker*iterations + iteration + 1
				load, err := callExplorationJSONRPC(handler, fmt.Sprintf(
					`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q]},"id":%d}`,
					source, requestID))
				if err != nil {
					errorsFound <- fmt.Errorf("worker %d iteration %d load: %w", worker, iteration, err)
					continue
				}
				result, ok := load["result"].(map[string]any)
				sessionID, sessionOK := result["sessionId"].(string)
				if !ok || !sessionOK || sessionID == "" {
					errorsFound <- fmt.Errorf("worker %d iteration %d load response: %#v", worker, iteration, load)
					continue
				}
				_, err = callExplorationJSONRPC(handler, fmt.Sprintf(
					`{"jsonrpc":"2.0","method":"assumeTransition","params":{"sessionId":%q,"transitionId":0,"checkEnabled":true},"id":%d}`,
					sessionID, requestID))
				if err != nil {
					errorsFound <- fmt.Errorf("worker %d iteration %d solve: %w", worker, iteration, err)
				}
				_, disposeErr := callExplorationJSONRPC(handler, fmt.Sprintf(
					`{"jsonrpc":"2.0","method":"disposeSpec","params":{"sessionId":%q},"id":%d}`,
					sessionID, requestID))
				if disposeErr != nil {
					errorsFound <- fmt.Errorf("worker %d iteration %d dispose: %w", worker, iteration, disposeErr)
				}
			}
		}(worker)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	service.mu.RLock()
	remainingSessions := len(service.sessions)
	service.mu.RUnlock()
	if remainingSessions != 0 {
		t.Fatalf("sessions after load/solve/dispose cycles = %d", remainingSessions)
	}
	runtime.GC()
	debug.FreeOSMemory()
	if got := processFileDescriptorCount(t); got != baselineFDs {
		t.Fatalf("file descriptors after explorer load = %d, want baseline %d", got, baselineFDs)
	}
	if got := runtime.NumGoroutine(); got > baselineGoroutines+2 {
		t.Fatalf("goroutines after explorer load = %d, baseline %d", got, baselineGoroutines)
	}
}

func callExplorationJSONRPC(handler http.Handler, body string) (map[string]any, error) {
	request := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)).WithContext(context.Background())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", response.Code)
	}
	var document map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		return nil, err
	}
	if rpcError, found := document["error"]; found {
		return nil, fmt.Errorf("JSON-RPC error: %#v", rpcError)
	}
	return document, nil
}
