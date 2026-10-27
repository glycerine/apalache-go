package apalache

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type loadSpecParams struct {
	Sources    []string `json:"sources"`
	Init       string   `json:"init"`
	Next       string   `json:"next"`
	Invariants []string `json:"invariants"`
	Exports    []string `json:"exports"`
	Format     string   `json:"format"`
}

type disposeSpecParams struct {
	SessionID string `json:"sessionId"`
}

type specEntryMetadata struct {
	Index  int      `json:"index"`
	Labels []string `json:"labels"`
}

type specParameters struct {
	InitTransitions  []specEntryMetadata `json:"initTransitions"`
	NextTransitions  []specEntryMetadata `json:"nextTransitions"`
	StateInvariants  []specEntryMetadata `json:"stateInvariants"`
	ActionInvariants []specEntryMetadata `json:"actionInvariants"`
}

type loadSpecResult struct {
	SessionID      string         `json:"sessionId"`
	SnapshotID     int            `json:"snapshotId"`
	SpecParameters specParameters `json:"specParameters"`
}

type disposeSpecResult struct {
	SessionID string `json:"sessionId"`
}

type assumeTransitionParams struct {
	SessionID    string `json:"sessionId"`
	TransitionID int    `json:"transitionId"`
	CheckEnabled bool   `json:"checkEnabled"`
	TimeoutSec   int    `json:"timeoutSec"`
}

type assumeTransitionResult struct {
	SessionID    string `json:"sessionId"`
	SnapshotID   int    `json:"snapshotId"`
	TransitionID int    `json:"transitionId"`
	Status       string `json:"status"`
}

type nextStepParams struct {
	SessionID string `json:"sessionId"`
}

type nextStepResult struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
	NewStepNo  int    `json:"newStepNo"`
}

type rollbackParams struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
}

type rollbackResult struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
}

type checkInvariantParams struct {
	SessionID   string `json:"sessionId"`
	InvariantID int    `json:"invariantId"`
	Kind        string `json:"kind"`
	TimeoutSec  int    `json:"timeoutSec"`
}

type checkInvariantResult struct {
	SessionID       string `json:"sessionId"`
	InvariantStatus string `json:"invariantStatus"`
	Trace           any    `json:"trace"`
}

type queryParams struct {
	SessionID  string   `json:"sessionId"`
	Kinds      []string `json:"kinds"`
	Operator   string   `json:"operator"`
	TimeoutSec int      `json:"timeoutSec"`
}

type queryResult struct {
	SessionID     string `json:"sessionId"`
	Trace         any    `json:"trace"`
	State         any    `json:"state"`
	OperatorValue any    `json:"operatorValue"`
}

type nextModelParams struct {
	SessionID  string `json:"sessionId"`
	Operator   string `json:"operator"`
	TimeoutSec int    `json:"timeoutSec"`
}

type nextModelResult struct {
	SessionID string `json:"sessionId"`
	OldValue  any    `json:"oldValue"`
	HasOld    string `json:"hasOld"`
	HasNext   string `json:"hasNext"`
}

type compactParams struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
	TimeoutSec int    `json:"timeoutSec"`
}

type compactResult struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
}

type applyInOrderCall struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type applyInOrderParams struct {
	SessionID string             `json:"sessionId"`
	Calls     []applyInOrderCall `json:"calls"`
}

type applyInOrderCallResult struct {
	OK     bool          `json:"ok"`
	Method string        `json:"method"`
	Result any           `json:"result"`
	Error  *jsonRPCError `json:"error"`
}

type applyInOrderResult struct {
	Calls []applyInOrderCallResult `json:"calls"`
}

type explorationOperation struct {
	transition *int
	advance    bool
	assertion  Expr
	synthetic  Expr
}

type assumeStateParams struct {
	SessionID    string          `json:"sessionId"`
	Equalities   json.RawMessage `json:"equalities"`
	CheckEnabled bool            `json:"checkEnabled"`
	TimeoutSec   int             `json:"timeoutSec"`
}

type assumeStateResult struct {
	SessionID  string `json:"sessionId"`
	SnapshotID int    `json:"snapshotId"`
	Status     string `json:"status"`
}

type explorationSession struct {
	mu             sync.Mutex
	prepared       SafetyPipelineResult
	executor       *TransitionExecutor
	operations     []explorationOperation
	snapshots      map[int][]explorationOperation
	lastSnapshotID int
	exports        map[string]struct{}
}

type lockedExplorationSessionKey struct{}

type lockedExplorationSession struct {
	service   *ExplorationService
	sessionID string
	session   *explorationSession
}

// ExplorationService owns the independent symbolic exploration sessions used
// by the JSON-RPC adapter.
type ExplorationService struct {
	lastSessionID atomic.Uint64
	mu            sync.RWMutex
	sessions      map[string]*explorationSession
}

func NewExplorationService() *ExplorationService {
	return &ExplorationService{sessions: make(map[string]*explorationSession)}
}

func (service *ExplorationService) loadSpec(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	params, err := parseLoadSpecParams(raw)
	if err != nil {
		return nil, invalidParams(err)
	}
	frontend, cleanup, err := loadExplorationSource(ctx, params)
	defer cleanup()
	if err != nil {
		return nil, invalidParams(err)
	}
	if !frontend.OK() {
		return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Failed to load specification: " + formatDiagnostics(frontend.Diagnostics)}
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: params.Init, Next: params.Next},
			Invariants: params.Invariants,
			Persistent: params.Exports,
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Failed to load specification: " + err.Error()}
	}
	parameters, err := explorationSpecParameters(prepared)
	if err != nil {
		return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Failed to load specification: " + err.Error()}
	}
	sessionID := fmt.Sprintf("%x", service.lastSessionID.Add(1))
	executor, err := newExplorationExecutor(prepared)
	if err != nil {
		return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Failed to initialize specification: " + err.Error()}
	}
	service.mu.Lock()
	service.sessions[sessionID] = &explorationSession{
		prepared: prepared, executor: executor, snapshots: map[int][]explorationOperation{0: {}},
		exports: stringSet(params.Exports),
	}
	service.mu.Unlock()
	return loadSpecResult{SessionID: sessionID, SnapshotID: 0, SpecParameters: parameters}, nil
}

func (service *ExplorationService) assumeTransition(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params assumeTransitionParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return nil, invalidParams(fmt.Errorf("Parse error in assumeTransition: %w", err))
	}
	if params.SessionID == "" || params.TransitionID < 0 || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("assumeTransition requires a non-empty sessionId, non-negative transitionId, and non-negative timeoutSec"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		beforeID := session.takeSnapshot()
		transitions := TransitionsFromModule(session.prepared.Module, NormalizedNextPrefix)
		if session.executor.Step() == 0 {
			transitions = TransitionsFromModule(session.prepared.Module, NormalizedInitPrefix)
		}
		if params.TransitionID >= len(transitions) {
			return nil, invalidParams(fmt.Errorf("Invalid transition ID: %d not in [0, %d)", params.TransitionID, len(transitions)))
		}
		enabled, err := session.executor.PrepareTransition(params.TransitionID, transitions[params.TransitionID])
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if !enabled {
			if err = session.recoverSnapshot(beforeID); err != nil {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
			}
			return assumeTransitionResult{params.SessionID, beforeID, params.TransitionID, "DISABLED"}, nil
		}
		if err = session.executor.AssumeTransition(params.TransitionID); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		transitionID := params.TransitionID
		session.operations = append(session.operations, explorationOperation{transition: &transitionID})
		afterID := session.takeSnapshot()
		if !params.CheckEnabled {
			return assumeTransitionResult{params.SessionID, afterID, params.TransitionID, "UNKNOWN"}, nil
		}
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		defer cancel()
		status, err := session.executor.Check(checkContext)
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		switch status {
		case SMTSatisfiable:
			return assumeTransitionResult{params.SessionID, afterID, params.TransitionID, "ENABLED"}, nil
		case SMTUnsatisfiable:
			if err = session.recoverSnapshot(beforeID); err != nil {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
			}
			return assumeTransitionResult{params.SessionID, beforeID, params.TransitionID, "DISABLED"}, nil
		default:
			return assumeTransitionResult{params.SessionID, afterID, params.TransitionID, "UNKNOWN"}, nil
		}
	})
}

func (service *ExplorationService) nextStep(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params nextStepParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" {
		return nil, invalidParams(fmt.Errorf("nextStep parameters must contain a non-empty sessionId"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if err := session.executor.NextState(); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		session.operations = append(session.operations, explorationOperation{advance: true})
		snapshotID := session.takeSnapshot()
		return nextStepResult{params.SessionID, snapshotID, session.executor.Step()}, nil
	})
}

func (service *ExplorationService) assumeState(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params assumeStateParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("assumeState requires a non-empty sessionId, equalities, and non-negative timeoutSec"))
	}
	var equalities map[string]any
	decoder := json.NewDecoder(bytes.NewReader(params.Equalities))
	decoder.UseNumber()
	if len(params.Equalities) == 0 || decoder.Decode(&equalities) != nil {
		return nil, invalidParams(fmt.Errorf("assumeState equalities must be a JSON object"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		types := make(map[string]Type)
		for _, variable := range session.prepared.Module.VarDeclarations() {
			if variable.IsInternal() {
				continue
			}
			typ, typed := variable.Tag().Type()
			if typed {
				types[variable.Name()] = typ
			}
		}
		assertions := make([]Expr, 0, len(equalities))
		for _, name := range sortedStringKeys(equalities) {
			if name == ITFMetaField {
				continue
			}
			typ, found := types[name]
			if !found {
				return nil, invalidParams(fmt.Errorf("Failed to parse the state: unknown variable %s", name))
			}
			value, err := DecodeITFValue(equalities[name], typ)
			if err != nil {
				return nil, invalidParams(fmt.Errorf("Failed to parse the state variable %s: %w", name, err))
			}
			assertions = append(assertions, MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr(name, Typed(typ)), value))
		}
		beforeID := session.takeSnapshot()
		for _, assertion := range assertions {
			if err := session.executor.AssertState(assertion); err != nil {
				_ = session.recoverSnapshot(beforeID)
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
			}
			session.operations = append(session.operations, explorationOperation{assertion: assertion})
		}
		afterID := session.takeSnapshot()
		if !params.CheckEnabled {
			return assumeStateResult{params.SessionID, afterID, "UNKNOWN"}, nil
		}
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		defer cancel()
		status, err := session.executor.Check(checkContext)
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if status == SMTUnsatisfiable {
			if err = session.recoverSnapshot(beforeID); err != nil {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
			}
			return assumeStateResult{params.SessionID, beforeID, "DISABLED"}, nil
		}
		if status == SMTSatisfiable {
			return assumeStateResult{params.SessionID, afterID, "ENABLED"}, nil
		}
		return assumeStateResult{params.SessionID, afterID, "UNKNOWN"}, nil
	})
}

func (service *ExplorationService) rollback(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params rollbackParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.SnapshotID < 0 {
		return nil, invalidParams(fmt.Errorf("rollback requires a non-empty sessionId and non-negative snapshotId"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if _, found := session.snapshots[params.SnapshotID]; !found {
			return nil, invalidParams(fmt.Errorf("Snapshot not found: %d in session %s", params.SnapshotID, params.SessionID))
		}
		if err := session.recoverSnapshot(params.SnapshotID); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		return rollbackResult{params.SessionID, params.SnapshotID}, nil
	})
}

func (service *ExplorationService) checkInvariant(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params checkInvariantParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.InvariantID < 0 ||
		(params.Kind != "STATE" && params.Kind != "ACTION") || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("checkInvariant requires sessionId, a non-negative invariantId, kind STATE or ACTION, and non-negative timeoutSec"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		state, action, err := explorationInvariantViolations(session.prepared)
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		expressions := state
		if params.Kind == "ACTION" {
			expressions = action
		}
		if params.InvariantID >= len(expressions) {
			return nil, invalidParams(fmt.Errorf("Invalid invariant ID: %s invariant %d not in [0, %d)", strings.ToLower(params.Kind), params.InvariantID, len(expressions)))
		}
		if session.executor.Step() == 0 || (session.executor.Step() == 1 && params.Kind == "ACTION") {
			return checkInvariantResult{params.SessionID, "SATISFIED", nil}, nil
		}
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		defer cancel()
		var status SMTStatus
		var states []DecodedState
		if params.Kind == "STATE" {
			status, states, err = session.executor.CheckAndDecode(checkContext, expressions[params.InvariantID])
		} else {
			pending, replayErr := session.executorBeforeLastAdvance()
			if replayErr != nil {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: replayErr.Error()}
			}
			status, states, err = pending.CheckPendingAndDecode(checkContext, expressions[params.InvariantID])
		}
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if status == SMTUnsatisfiable {
			return checkInvariantResult{params.SessionID, "SATISFIED", nil}, nil
		}
		if status != SMTSatisfiable {
			return checkInvariantResult{params.SessionID, "UNKNOWN", nil}, nil
		}
		encoded, err := MarshalDecodedTraceITF(session.prepared.Module, states, ITFOptions{})
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		var trace any
		if err = json.Unmarshal(encoded, &trace); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		return checkInvariantResult{params.SessionID, "VIOLATED", trace}, nil
	})
}

func (service *ExplorationService) query(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params queryParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || len(params.Kinds) == 0 || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("query requires a non-empty sessionId and kinds, and non-negative timeoutSec"))
	}
	wantTrace, wantState, wantOperator := false, false, false
	for _, kind := range params.Kinds {
		switch kind {
		case "TRACE":
			wantTrace = true
		case "STATE":
			wantState = true
		case "OPERATOR":
			wantOperator = true
		default:
			return nil, invalidParams(fmt.Errorf("unsupported query kind %q", kind))
		}
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		result := queryResult{SessionID: params.SessionID}
		if wantOperator {
			if params.Operator == "" {
				return nil, invalidParams(fmt.Errorf("operator must be provided for an OPERATOR query"))
			}
			if _, exported := session.exports[params.Operator]; !exported {
				return nil, invalidParams(fmt.Errorf("Operator not found: %s", params.Operator))
			}
			body, found := OperatorBody(session.prepared.Module, params.Operator)
			if !found {
				return nil, invalidParams(fmt.Errorf("Operator not found: %s", params.Operator))
			}
			checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
			value, status, evaluateErr := session.executor.Evaluate(checkContext, body)
			cancel()
			if evaluateErr != nil {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: evaluateErr.Error()}
			}
			if status == SMTSatisfiable {
				result.OperatorValue, evaluateErr = EncodeITFValue(value)
				if evaluateErr != nil {
					return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: evaluateErr.Error()}
				}
			}
		}
		if !wantTrace && !wantState {
			return result, nil
		}
		truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		defer cancel()
		var status SMTStatus
		var states []DecodedState
		var err error
		if len(session.operations) > 0 && session.operations[len(session.operations)-1].transition != nil {
			status, states, err = session.executor.CheckPendingAndDecode(checkContext, truth)
		} else {
			status, states, err = session.executor.CheckAndDecode(checkContext, truth)
		}
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if status != SMTSatisfiable || len(states) == 0 {
			return result, nil
		}
		encoded, err := MarshalDecodedTraceITF(session.prepared.Module, states, ITFOptions{})
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		var trace map[string]any
		if err = json.Unmarshal(encoded, &trace); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if wantTrace {
			result.Trace = trace
		}
		if wantState {
			encodedStates, _ := trace[ITFStatesField].([]any)
			if len(encodedStates) > 0 {
				result.State = encodedStates[len(encodedStates)-1]
			}
		}
		return result, nil
	})
}

func (service *ExplorationService) nextModel(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params nextModelParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.Operator == "" || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("nextModel requires sessionId, operator, and non-negative timeoutSec"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if _, exported := session.exports[params.Operator]; !exported {
			return nil, invalidParams(fmt.Errorf("Operator not found: %s", params.Operator))
		}
		body, found := OperatorBody(session.prepared.Module, params.Operator)
		if !found {
			return nil, invalidParams(fmt.Errorf("Operator not found: %s", params.Operator))
		}
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		oldValue, status, err := session.executor.Evaluate(checkContext, body)
		cancel()
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if status != SMTSatisfiable {
			return nextModelResult{SessionID: params.SessionID, HasOld: "FALSE", HasNext: "FALSE"}, nil
		}
		encodedOld, err := EncodeITFValue(oldValue)
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		equality := MustOperExpr(Op("EQ"), Typed(BoolT1), body, oldValue)
		disequality := MustOperExpr(Op("NOT"), Typed(BoolT1), equality)
		if err = session.executor.AssertState(disequality); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		session.operations = append(session.operations, explorationOperation{assertion: disequality})
		checkContext, cancel = explorationTimeout(ctx, params.TimeoutSec)
		defer cancel()
		status, err = session.executor.Check(checkContext)
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		hasNext := "UNKNOWN"
		if status == SMTSatisfiable {
			hasNext = "TRUE"
		} else if status == SMTUnsatisfiable {
			hasNext = "FALSE"
		}
		return nextModelResult{params.SessionID, encodedOld, "TRUE", hasNext}, nil
	})
}

func (service *ExplorationService) compact(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params compactParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.SnapshotID < 0 || params.TimeoutSec < 0 {
		return nil, invalidParams(fmt.Errorf("compact requires sessionId, non-negative snapshotId, and non-negative timeoutSec"))
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if _, found := session.snapshots[params.SnapshotID]; !found {
			return nil, invalidParams(fmt.Errorf("Snapshot not found: %d in session %s", params.SnapshotID, params.SessionID))
		}
		truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
		checkContext, cancel := explorationTimeout(ctx, params.TimeoutSec)
		var status SMTStatus
		var states []DecodedState
		var err error
		if len(session.operations) > 0 && session.operations[len(session.operations)-1].transition != nil {
			status, states, err = session.executor.CheckPendingAndDecode(checkContext, truth)
		} else {
			status, states, err = session.executor.CheckAndDecode(checkContext, truth)
		}
		cancel()
		if err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		if status != SMTSatisfiable || len(states) == 0 {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Cannot compact: current context is not satisfiable"}
		}
		last := states[len(states)-1].Values
		assignments := make([]Expr, 0, len(session.prepared.Module.VarDeclarations()))
		for _, variable := range session.prepared.Module.VarDeclarations() {
			value, found := last[variable.Name()]
			if !found {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Cannot compact: decoded state has no " + variable.Name()}
			}
			typ, typed := variable.Tag().Type()
			if !typed {
				return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: "Cannot compact: untyped variable " + variable.Name()}
			}
			prime := MustOperExpr(Op("PRIME"), Typed(typ), NewNameExpr(variable.Name(), Typed(typ)))
			assignments = append(assignments, MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value))
		}
		synthetic := MustOperExpr(Op("AND"), Typed(BoolT1), assignments...)
		if err = session.recoverSnapshot(params.SnapshotID); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		enabled, err := session.executor.PrepareTransition(0, synthetic)
		if err != nil || !enabled {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: fmt.Sprintf("Cannot compact: synthetic transition enabled=%v: %v", enabled, err)}
		}
		if err = session.executor.AssumeTransition(0); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		session.operations = append(session.operations, explorationOperation{synthetic: synthetic})
		if err = session.executor.NextState(); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		session.operations = append(session.operations, explorationOperation{advance: true})
		return compactResult{params.SessionID, session.takeSnapshot()}, nil
	})
}

func (service *ExplorationService) applyInOrder(ctx context.Context, raw json.RawMessage) (any, *jsonRPCError) {
	var params applyInOrderParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || len(params.Calls) == 0 {
		return nil, invalidParams(fmt.Errorf("applyInOrder requires a non-empty sessionId and calls array"))
	}
	allowed := map[string]struct{}{
		"assumeTransition": {}, "assumeState": {}, "nextStep": {}, "query": {},
		"checkInvariant": {}, "nextModel": {}, "rollback": {}, "compact": {},
	}
	prepared := make([]applyInOrderCall, len(params.Calls))
	for index, call := range params.Calls {
		if _, found := allowed[call.Method]; !found {
			return nil, invalidParams(fmt.Errorf("Unsupported applyInOrder method: %s", call.Method))
		}
		var object map[string]json.RawMessage
		if len(call.Params) == 0 || json.Unmarshal(call.Params, &object) != nil || object == nil {
			return nil, invalidParams(fmt.Errorf("Parameters for applyInOrder step %q must be a JSON object", call.Method))
		}
		sessionJSON, _ := json.Marshal(params.SessionID)
		object["sessionId"] = sessionJSON
		encoded, err := json.Marshal(object)
		if err != nil {
			return nil, invalidParams(err)
		}
		prepared[index] = applyInOrderCall{Method: call.Method, Params: encoded}
	}
	return service.withSession(ctx, params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		batchContext := context.WithValue(ctx, lockedExplorationSessionKey{}, &lockedExplorationSession{
			service: service, sessionID: params.SessionID, session: session,
		})
		result := applyInOrderResult{Calls: make([]applyInOrderCallResult, 0, len(prepared))}
		for _, call := range prepared {
			value, callErr := service.dispatchApplyInOrder(batchContext, call)
			one := applyInOrderCallResult{OK: callErr == nil, Method: call.Method, Result: value, Error: callErr}
			result.Calls = append(result.Calls, one)
			if callErr != nil {
				break
			}
		}
		return result, nil
	})
}

func (service *ExplorationService) dispatchApplyInOrder(ctx context.Context, call applyInOrderCall) (any, *jsonRPCError) {
	switch call.Method {
	case "assumeTransition":
		return service.assumeTransition(ctx, call.Params)
	case "assumeState":
		return service.assumeState(ctx, call.Params)
	case "nextStep":
		return service.nextStep(ctx, call.Params)
	case "query":
		return service.query(ctx, call.Params)
	case "checkInvariant":
		return service.checkInvariant(ctx, call.Params)
	case "nextModel":
		return service.nextModel(ctx, call.Params)
	case "rollback":
		return service.rollback(ctx, call.Params)
	case "compact":
		return service.compact(ctx, call.Params)
	default:
		return nil, invalidParams(fmt.Errorf("Unsupported applyInOrder method: %s", call.Method))
	}
}

func (service *ExplorationService) disposeSpec(raw json.RawMessage) (any, *jsonRPCError) {
	var params disposeSpecParams
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return nil, invalidParams(fmt.Errorf("Parse error in disposeSpec: %w", err))
	}
	if params.SessionID == "" {
		return nil, invalidParams(fmt.Errorf("disposeSpec parameters must contain a non-empty sessionId"))
	}
	service.mu.Lock()
	serviceSession, found := service.sessions[params.SessionID]
	if found {
		delete(service.sessions, params.SessionID)
	}
	service.mu.Unlock()
	if !found {
		return nil, &jsonRPCError{Code: JSONRPCSessionMissing, Message: "Session not found: " + params.SessionID}
	}
	// Wait for an in-flight operation on this session before reporting that its
	// resources have been retired.
	serviceSession.mu.Lock()
	serviceSession.mu.Unlock()
	return disposeSpecResult{SessionID: params.SessionID}, nil
}

func parseLoadSpecParams(raw json.RawMessage) (loadSpecParams, error) {
	params := loadSpecParams{Init: "Init", Next: "Next", Format: "tla"}
	if err := decodeJSONRPCParams(raw, &params); err != nil {
		return loadSpecParams{}, fmt.Errorf("Parse error in loadSpec: %w", err)
	}
	if len(params.Sources) == 0 {
		return loadSpecParams{}, fmt.Errorf("loadSpec parameters must be non-empty")
	}
	for index, encoded := range params.Sources {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return loadSpecParams{}, fmt.Errorf("Parse error in loadSpec source %d: %w", index, err)
		}
		params.Sources[index] = string(decoded)
	}
	params.Format = strings.ToLower(params.Format)
	if params.Format != "tla" && params.Format != "json" {
		return loadSpecParams{}, fmt.Errorf("loadSpec does not support the %s source format", params.Format)
	}
	if params.Format != "tla" && len(params.Sources) != 1 {
		return loadSpecParams{}, fmt.Errorf("loadSpec requires exactly one source for %s input", params.Format)
	}
	return params, nil
}

func loadExplorationSource(ctx context.Context, params loadSpecParams) (FrontendResult, func(), error) {
	directory, err := os.MkdirTemp("", "apalache-explorer-")
	if err != nil {
		return FrontendResult{}, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	if params.Format == "json" {
		path := filepath.Join(directory, "spec.json")
		if err = os.WriteFile(path, []byte(params.Sources[0]), 0o600); err != nil {
			cleanup()
			return FrontendResult{}, func() {}, err
		}
		return ParseAndTypeCheckFile(ctx, path, FrontendOptions{}, TypeCheckOptions{}), cleanup, nil
	}
	rootPath := ""
	for index, source := range params.Sources {
		match := moduleHeaderPattern.FindStringSubmatch(source)
		if len(match) != 2 {
			cleanup()
			return FrontendResult{}, func() {}, fmt.Errorf("loadSpec source %d has no TLA+ module header", index)
		}
		path := filepath.Join(directory, match[1]+".tla")
		if err = os.WriteFile(path, []byte(source), 0o600); err != nil {
			cleanup()
			return FrontendResult{}, func() {}, err
		}
		if index == 0 {
			rootPath = path
		}
	}
	return ParseAndTypeCheckFile(ctx, rootPath, FrontendOptions{}, TypeCheckOptions{}), cleanup, nil
}

func explorationSpecParameters(prepared SafetyPipelineResult) (specParameters, error) {
	result := specParameters{
		InitTransitions: []specEntryMetadata{}, NextTransitions: []specEntryMetadata{},
		StateInvariants: []specEntryMetadata{}, ActionInvariants: []specEntryMetadata{},
	}
	result.InitTransitions = metadataForExpressions(TransitionsFromModule(prepared.Module, NormalizedInitPrefix))
	result.NextTransitions = metadataForExpressions(TransitionsFromModule(prepared.Module, NormalizedNextPrefix))
	levels := NewLevelFinder(prepared.Module)
	for _, name := range prepared.Predicates.Invariants {
		declaration, found := declarationNamed(prepared.Module, name)
		operator, ok := declaration.(*OperDecl)
		if !found || !ok || len(operator.FormalParams()) != 0 {
			return specParameters{}, fmt.Errorf("invariant %s was not found as a nullary operator", name)
		}
		entry := specEntryMetadata{Labels: expressionLabels(operator.Body())}
		switch levels.LevelOfExpression(operator.Body()) {
		case ConstantLevel, StateLevel:
			entry.Index = len(result.StateInvariants)
			result.StateInvariants = append(result.StateInvariants, entry)
		case ActionLevel:
			entry.Index = len(result.ActionInvariants)
			result.ActionInvariants = append(result.ActionInvariants, entry)
		default:
			return specParameters{}, fmt.Errorf("invariant %s is temporal", name)
		}
	}
	return result, nil
}

func explorationInvariantViolations(prepared SafetyPipelineResult) (state, action []Expr, err error) {
	levels := NewLevelFinder(prepared.Module)
	for _, name := range prepared.Predicates.Invariants {
		declaration, found := declarationNamed(prepared.Module, name)
		operator, ok := declaration.(*OperDecl)
		if !found || !ok || len(operator.FormalParams()) != 0 {
			return nil, nil, fmt.Errorf("invariant %s was not found as a nullary operator", name)
		}
		body := invariantAssignmentsToEquality(operator.Body())
		violation, violationErr := invariantViolationExpression(body)
		if violationErr != nil {
			return nil, nil, violationErr
		}
		switch levels.LevelOfExpression(body) {
		case ConstantLevel, StateLevel:
			state = append(state, violation)
		case ActionLevel:
			action = append(action, violation)
		default:
			return nil, nil, fmt.Errorf("invariant %s is temporal", name)
		}
	}
	return state, action, nil
}

func metadataForExpressions(expressions []Expr) []specEntryMetadata {
	result := make([]specEntryMetadata, len(expressions))
	for index, expression := range expressions {
		result[index] = specEntryMetadata{Index: index, Labels: expressionLabels(expression)}
	}
	return result
}

func expressionLabels(expression Expr) []string {
	labels := make(map[string]struct{})
	var visit func(Expr)
	visit = func(current Expr) {
		operator, ok := current.(*OperExpr)
		if ok && operator.Operator().Name() == "LABEL" && len(operator.Arguments()) >= 2 {
			value, valueOK := operator.Arguments()[1].(*ValueExpr)
			name, nameOK := StringValue(""), false
			if valueOK {
				name, nameOK = value.Value().(StringValue)
			}
			if nameOK {
				labels[string(name)] = struct{}{}
			}
		}
		switch typed := current.(type) {
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visit(argument)
			}
		case *LetInExpr:
			visit(typed.Body())
			for _, declaration := range typed.Declarations() {
				visit(declaration.Body())
			}
		}
	}
	if expression == nil {
		return []string{}
	}
	visit(expression)
	result := make([]string, 0, len(labels))
	for label := range labels {
		result = append(result, label)
	}
	sort.Strings(result)
	return result
}

func newExplorationExecutor(prepared SafetyPipelineResult) (*TransitionExecutor, error) {
	executor := NewTransitionExecutor(
		declarationNameSlice(prepared.Module.ConstDeclarations()),
		declarationNameSlice(prepared.Module.VarDeclarations()),
	)
	if initializer, found := OperatorBody(prepared.Module, NormalizedConstInit); found {
		if err := executor.InitializeConstants(initializer); err != nil {
			return nil, err
		}
	}
	for _, assumption := range prepared.Module.AssumeDeclarations() {
		if err := executor.AssertState(assumption.Body()); err != nil {
			return nil, fmt.Errorf("assert assumption %s: %w", assumption.Name(), err)
		}
	}
	return executor, nil
}

func (session *explorationSession) takeSnapshot() int {
	session.lastSnapshotID++
	session.snapshots[session.lastSnapshotID] = append([]explorationOperation(nil), session.operations...)
	return session.lastSnapshotID
}

func (session *explorationSession) recoverSnapshot(snapshotID int) error {
	operations, found := session.snapshots[snapshotID]
	if !found {
		return fmt.Errorf("snapshot %d was not found", snapshotID)
	}
	executor, err := replayExplorationOperations(session.prepared, operations)
	if err != nil {
		return err
	}
	session.executor = executor
	session.operations = append([]explorationOperation(nil), operations...)
	return nil
}

func (session *explorationSession) executorBeforeLastAdvance() (*TransitionExecutor, error) {
	if len(session.operations) == 0 || !session.operations[len(session.operations)-1].advance {
		return nil, fmt.Errorf("action invariant requires a completed action")
	}
	return replayExplorationOperations(session.prepared, session.operations[:len(session.operations)-1])
}

func replayExplorationOperations(prepared SafetyPipelineResult, operations []explorationOperation) (*TransitionExecutor, error) {
	executor, err := newExplorationExecutor(prepared)
	if err != nil {
		return nil, err
	}
	for _, operation := range operations {
		if operation.synthetic != nil {
			enabled, prepareErr := executor.PrepareTransition(0, operation.synthetic)
			if prepareErr != nil || !enabled {
				return nil, fmt.Errorf("replay synthetic transition enabled=%v: %v", enabled, prepareErr)
			}
			if err = executor.AssumeTransition(0); err != nil {
				return nil, err
			}
			continue
		}
		if operation.assertion != nil {
			if err = executor.AssertState(operation.assertion); err != nil {
				return nil, err
			}
			continue
		}
		if operation.advance {
			if err = executor.NextState(); err != nil {
				return nil, err
			}
			continue
		}
		if operation.transition == nil {
			return nil, fmt.Errorf("invalid empty exploration operation")
		}
		transitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix)
		if executor.Step() == 0 {
			transitions = TransitionsFromModule(prepared.Module, NormalizedInitPrefix)
		}
		number := *operation.transition
		if number < 0 || number >= len(transitions) {
			return nil, fmt.Errorf("transition %d is out of range while recovering snapshot", number)
		}
		enabled, prepareErr := executor.PrepareTransition(number, transitions[number])
		if prepareErr != nil {
			return nil, prepareErr
		}
		if !enabled {
			return nil, fmt.Errorf("transition %d became disabled while recovering snapshot", number)
		}
		if err = executor.AssumeTransition(number); err != nil {
			return nil, err
		}
	}
	return executor, nil
}

func (service *ExplorationService) withSession(
	ctx context.Context,
	sessionID string,
	callback func(*explorationSession) (any, *jsonRPCError),
) (any, *jsonRPCError) {
	if ctx != nil {
		locked, _ := ctx.Value(lockedExplorationSessionKey{}).(*lockedExplorationSession)
		if locked != nil && locked.service == service && locked.sessionID == sessionID {
			return callback(locked.session)
		}
	}
	service.mu.RLock()
	session, found := service.sessions[sessionID]
	if !found {
		service.mu.RUnlock()
		return nil, &jsonRPCError{Code: JSONRPCSessionMissing, Message: "Session not found: " + sessionID}
	}
	session.mu.Lock()
	service.mu.RUnlock()
	defer session.mu.Unlock()
	return callback(session)
}

func explorationTimeout(parent context.Context, seconds int) (context.Context, context.CancelFunc) {
	if seconds <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, time.Duration(seconds)*time.Second)
}

func decodeJSONRPCParams(raw json.RawMessage, destination any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return fmt.Errorf("missing parameters")
	}
	return json.Unmarshal(raw, destination)
}

func invalidParams(err error) *jsonRPCError {
	return &jsonRPCError{Code: JSONRPCInvalidParams, Message: err.Error()}
}

func formatDiagnostics(diagnostics []Diagnostic) string {
	messages := make([]string, len(diagnostics))
	for index, diagnostic := range diagnostics {
		messages[index] = diagnostic.Message
	}
	return strings.Join(messages, "; ")
}
