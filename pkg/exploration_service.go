package apalache

import (
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

type explorationOperation struct {
	transition *int
	advance    bool
}

type explorationSession struct {
	mu             sync.Mutex
	prepared       SafetyPipelineResult
	executor       *TransitionExecutor
	operations     []explorationOperation
	snapshots      map[int][]explorationOperation
	lastSnapshotID int
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
	return service.withSession(params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
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

func (service *ExplorationService) nextStep(raw json.RawMessage) (any, *jsonRPCError) {
	var params nextStepParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" {
		return nil, invalidParams(fmt.Errorf("nextStep parameters must contain a non-empty sessionId"))
	}
	return service.withSession(params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if err := session.executor.NextState(); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		session.operations = append(session.operations, explorationOperation{advance: true})
		snapshotID := session.takeSnapshot()
		return nextStepResult{params.SessionID, snapshotID, session.executor.Step()}, nil
	})
}

func (service *ExplorationService) rollback(raw json.RawMessage) (any, *jsonRPCError) {
	var params rollbackParams
	if err := decodeJSONRPCParams(raw, &params); err != nil || params.SessionID == "" || params.SnapshotID < 0 {
		return nil, invalidParams(fmt.Errorf("rollback requires a non-empty sessionId and non-negative snapshotId"))
	}
	return service.withSession(params.SessionID, func(session *explorationSession) (any, *jsonRPCError) {
		if _, found := session.snapshots[params.SnapshotID]; !found {
			return nil, invalidParams(fmt.Errorf("Snapshot not found: %d in session %s", params.SnapshotID, params.SessionID))
		}
		if err := session.recoverSnapshot(params.SnapshotID); err != nil {
			return nil, &jsonRPCError{Code: JSONRPCInternalError, Message: err.Error()}
		}
		return rollbackResult{params.SessionID, params.SnapshotID}, nil
	})
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
	executor, err := newExplorationExecutor(session.prepared)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		if operation.advance {
			if err = executor.NextState(); err != nil {
				return err
			}
			continue
		}
		if operation.transition == nil {
			return fmt.Errorf("invalid empty exploration operation")
		}
		transitions := TransitionsFromModule(session.prepared.Module, NormalizedNextPrefix)
		if executor.Step() == 0 {
			transitions = TransitionsFromModule(session.prepared.Module, NormalizedInitPrefix)
		}
		number := *operation.transition
		if number < 0 || number >= len(transitions) {
			return fmt.Errorf("transition %d is out of range while recovering snapshot", number)
		}
		enabled, prepareErr := executor.PrepareTransition(number, transitions[number])
		if prepareErr != nil {
			return prepareErr
		}
		if !enabled {
			return fmt.Errorf("transition %d became disabled while recovering snapshot", number)
		}
		if err = executor.AssumeTransition(number); err != nil {
			return err
		}
	}
	session.executor = executor
	session.operations = append([]explorationOperation(nil), operations...)
	return nil
}

func (service *ExplorationService) withSession(
	sessionID string,
	callback func(*explorationSession) (any, *jsonRPCError),
) (any, *jsonRPCError) {
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
