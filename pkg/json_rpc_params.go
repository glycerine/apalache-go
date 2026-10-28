package apalache

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

func parseDisposeSpecParams(raw json.RawMessage) (disposeSpecParams, error) {
	var params disposeSpecParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in disposeSpec: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("disposeSpec parameters must contain a non-empty sessionId.")
	}
	return params, nil
}

func parseAssumeTransitionParams(raw json.RawMessage) (assumeTransitionParams, error) {
	var params assumeTransitionParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in assumeTransition: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("assumeTransition parameters must contain a non-empty sessionId.")
	}
	if params.TransitionID < 0 {
		return params, fmt.Errorf("assumeTransition parameters must contain a non-negative transitionId.")
	}
	return params, nil
}

func parseAssumeStateParams(raw json.RawMessage) (assumeStateParams, error) {
	var params assumeStateParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in assumeState: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("assumeState parameters must contain a non-empty sessionId.")
	}
	return params, nil
}

func parseRollbackParams(raw json.RawMessage) (rollbackParams, error) {
	var params rollbackParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in rollback: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("rollback parameters must contain a non-empty sessionId.")
	}
	if params.SnapshotID < 0 {
		return params, fmt.Errorf("rollback parameters must contain a non-negative snapshotId.")
	}
	return params, nil
}

func parseCompactParams(raw json.RawMessage) (compactParams, error) {
	var params compactParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in compact: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("compact parameters must contain a non-empty sessionId.")
	}
	if params.SnapshotID < 0 {
		return params, fmt.Errorf("compact parameters must contain a non-negative snapshotId.")
	}
	return params, nil
}

func parseNextStepParams(raw json.RawMessage) (nextStepParams, error) {
	var params nextStepParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in nextStep: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("nextStep parameters must contain a non-empty sessionId.")
	}
	return params, nil
}

func parseCheckInvariantParams(raw json.RawMessage) (checkInvariantParams, error) {
	params := checkInvariantParams{Kind: "STATE"}
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in checkInvariant: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("checkInvariant parameters must contain a non-empty sessionId.")
	}
	if params.InvariantID < 0 {
		return params, fmt.Errorf("invariantId must be non-negative.")
	}
	if params.Kind != "STATE" && params.Kind != "ACTION" {
		return params, fmt.Errorf("kind must be either 'STATE' or 'ACTION'.")
	}
	return params, nil
}

func parseQueryParams(raw json.RawMessage) (queryParams, error) {
	var params queryParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in nextStep: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("nextStep parameters must contain a non-empty `sessionId`.")
	}
	if len(params.Kinds) == 0 {
		return params, fmt.Errorf("query parameters must contain a non-empty `kinds` array.")
	}
	for _, kind := range params.Kinds {
		if kind != "OPERATOR" && kind != "TRACE" && kind != "STATE" {
			return params, fmt.Errorf("Field `kinds` may contain only: OPERATOR, TRACE, or STATE.")
		}
	}
	if slicesContain(params.Kinds, "OPERATOR") && params.Operator == "" {
		return params, fmt.Errorf("Field `operator` must be provided when `kinds` contains 'OPERATOR'.")
	}
	return params, nil
}

func parseNextModelParams(raw json.RawMessage) (nextModelParams, error) {
	var params nextModelParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in nextStep: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("nextStep parameters must contain a non-empty `sessionId`.")
	}
	if params.Operator == "" {
		return params, fmt.Errorf("Field `operator` must be provided.")
	}
	return params, nil
}

func parseApplyInOrderParams(raw json.RawMessage) (applyInOrderParams, error) {
	var params applyInOrderParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return params, fmt.Errorf("Parse error in applyInOrder: %w", err)
	}
	if params.SessionID == "" {
		return params, fmt.Errorf("applyInOrder parameters must contain a non-empty sessionId.")
	}
	if len(params.Calls) == 0 {
		return params, fmt.Errorf("applyInOrder parameters must contain a non-empty calls array.")
	}
	allowed := map[string]struct{}{
		"assumeTransition": {}, "assumeState": {}, "nextStep": {}, "query": {},
		"checkInvariant": {}, "nextModel": {}, "rollback": {}, "compact": {},
	}
	for _, call := range params.Calls {
		if _, found := allowed[call.Method]; !found {
			return params, fmt.Errorf("Unsupported applyInOrder method: %s.", call.Method)
		}
		var object map[string]json.RawMessage
		if len(call.Params) == 0 || json.Unmarshal(call.Params, &object) != nil || object == nil {
			return params, fmt.Errorf("Parameters for applyInOrder step '%s' must be a JSON object.", call.Method)
		}
	}
	return params, nil
}

func slicesContain(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func parseLoadSpecParams(raw json.RawMessage) (loadSpecParams, error) {
	params := loadSpecParams{Init: "Init", Next: "Next", Format: "tla", Invariants: []string{}, Exports: []string{}}
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return loadSpecParams{}, fmt.Errorf("Parse error in loadSpec: %w", err)
	}
	if len(params.Sources) == 0 {
		return loadSpecParams{}, fmt.Errorf("loadSpec parameters must be non-empty.")
	}
	for index, encoded := range params.Sources {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return loadSpecParams{}, fmt.Errorf("Parse error in loadSpec: source %d: %w", index, err)
		}
		params.Sources[index] = string(decoded)
	}
	params.Format = strings.ToLower(params.Format)
	if params.Format != "tla" && params.Format != "json" && params.Format != "qnt" {
		return loadSpecParams{}, fmt.Errorf("loadSpec does not support the %s source format", params.Format)
	}
	if params.Format != "tla" && len(params.Sources) != 1 {
		return loadSpecParams{}, fmt.Errorf("loadSpec requires exactly one source for %s input", params.Format)
	}
	return params, nil
}
