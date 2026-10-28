package apalache

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONRPCParameterDefaultsMatchScala(t *testing.T) {
	source := "---- MODULE M ----\n===="
	encoded := base64.StdEncoding.EncodeToString([]byte(source))
	load, err := parseLoadSpecParams(json.RawMessage(`{"sources":["` + encoded + `"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(load.Sources, []string{source}) || load.Init != "Init" || load.Next != "Next" ||
		load.Format != "tla" || len(load.Invariants) != 0 || len(load.Exports) != 0 {
		t.Fatalf("minimal loadSpec = %+v", load)
	}
	check, err := parseCheckInvariantParams(json.RawMessage(`{"sessionId":"s","invariantId":3,"timeoutSec":300}`))
	if err != nil {
		t.Fatal(err)
	}
	if check.Kind != "STATE" || check.TimeoutSec != 300 {
		t.Fatalf("default checkInvariant = %+v", check)
	}
	compact, err := parseCompactParams(json.RawMessage(`{"sessionId":"s","snapshotId":0}`))
	if err != nil || compact.TimeoutSec != 0 {
		t.Fatalf("default compact = %+v, %v", compact, err)
	}
	query, err := parseQueryParams(json.RawMessage(`{"sessionId":"s","kinds":["STATE"]}`))
	if err != nil || query.Operator != "" || query.TimeoutSec != 0 {
		t.Fatalf("state query = %+v, %v", query, err)
	}
}

func TestJSONRPCParameterValidationMessagesMatchScala(t *testing.T) {
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{"load empty", func() error { _, err := parseLoadSpecParams(json.RawMessage(`{"sources":[]}`)); return err }, "loadSpec parameters must be non-empty."},
		{"dispose session", func() error { _, err := parseDisposeSpecParams(json.RawMessage(`{"sessionId":""}`)); return err }, "disposeSpec parameters must contain a non-empty sessionId."},
		{"transition session", func() error { _, err := parseAssumeTransitionParams(json.RawMessage(`{"transitionId":0}`)); return err }, "assumeTransition parameters must contain a non-empty sessionId."},
		{"transition id", func() error {
			_, err := parseAssumeTransitionParams(json.RawMessage(`{"sessionId":"s","transitionId":-1}`))
			return err
		}, "assumeTransition parameters must contain a non-negative transitionId."},
		{"state session", func() error { _, err := parseAssumeStateParams(json.RawMessage(`{"sessionId":""}`)); return err }, "assumeState parameters must contain a non-empty sessionId."},
		{"rollback id", func() error {
			_, err := parseRollbackParams(json.RawMessage(`{"sessionId":"s","snapshotId":-1}`))
			return err
		}, "rollback parameters must contain a non-negative snapshotId."},
		{"compact session", func() error { _, err := parseCompactParams(json.RawMessage(`{"snapshotId":0}`)); return err }, "compact parameters must contain a non-empty sessionId."},
		{"next session", func() error { _, err := parseNextStepParams(json.RawMessage(`{}`)); return err }, "nextStep parameters must contain a non-empty sessionId."},
		{"invariant id", func() error {
			_, err := parseCheckInvariantParams(json.RawMessage(`{"sessionId":"s","invariantId":-1}`))
			return err
		}, "invariantId must be non-negative."},
		{"invariant kind", func() error {
			_, err := parseCheckInvariantParams(json.RawMessage(`{"sessionId":"s","invariantId":0,"kind":"TEMPORAL"}`))
			return err
		}, "kind must be either 'STATE' or 'ACTION'."},
		{"query kinds", func() error { _, err := parseQueryParams(json.RawMessage(`{"sessionId":"s","kinds":[]}`)); return err }, "query parameters must contain a non-empty `kinds` array."},
		{"query kind", func() error {
			_, err := parseQueryParams(json.RawMessage(`{"sessionId":"s","kinds":["VIEW"]}`))
			return err
		}, "Field `kinds` may contain only: OPERATOR, TRACE, or STATE."},
		{"query operator", func() error {
			_, err := parseQueryParams(json.RawMessage(`{"sessionId":"s","kinds":["OPERATOR"]}`))
			return err
		}, "Field `operator` must be provided when `kinds` contains 'OPERATOR'."},
		{"model operator", func() error { _, err := parseNextModelParams(json.RawMessage(`{"sessionId":"s"}`)); return err }, "Field `operator` must be provided."},
		{"batch calls", func() error {
			_, err := parseApplyInOrderParams(json.RawMessage(`{"sessionId":"s","calls":[]}`))
			return err
		}, "applyInOrder parameters must contain a non-empty calls array."},
		{"batch method", func() error {
			_, err := parseApplyInOrderParams(json.RawMessage(`{"sessionId":"s","calls":[{"method":"health","params":{}}]}`))
			return err
		}, "Unsupported applyInOrder method: health."},
		{"batch params", func() error {
			_, err := parseApplyInOrderParams(json.RawMessage(`{"sessionId":"s","calls":[{"method":"query","params":[]}]}`))
			return err
		}, "Parameters for applyInOrder step 'query' must be a JSON object."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestJSONRPCParameterShapesMatchScala(t *testing.T) {
	transition, err := parseAssumeTransitionParams(json.RawMessage(`{"sessionId":"s","transitionId":1,"checkEnabled":true,"timeoutSec":600}`))
	if err != nil || transition != (assumeTransitionParams{SessionID: "s", TransitionID: 1, CheckEnabled: true, TimeoutSec: 600}) {
		t.Fatalf("transition = %+v, %v", transition, err)
	}
	state, err := parseAssumeStateParams(json.RawMessage(`{"sessionId":"s","equalities":{"msg":"hello","x":{"#bigint":"42"}},"checkEnabled":true,"timeoutSec":600}`))
	if err != nil || state.SessionID != "s" || !state.CheckEnabled || state.TimeoutSec != 600 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	var equalities map[string]any
	if err = json.Unmarshal(state.Equalities, &equalities); err != nil || len(equalities) != 2 {
		t.Fatalf("equalities = %#v, %v", equalities, err)
	}
	batch, err := parseApplyInOrderParams(json.RawMessage(`{"sessionId":"s","calls":[{"method":"nextStep","params":{}},{"method":"query","params":{"kinds":["TRACE"]}}]}`))
	if err != nil || len(batch.Calls) != 2 || batch.Calls[0].Method != "nextStep" || batch.Calls[1].Method != "query" {
		t.Fatalf("batch = %+v, %v", batch, err)
	}
}
