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

type explorationSession struct {
	mu       sync.Mutex
	prepared SafetyPipelineResult
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
	service.mu.Lock()
	service.sessions[sessionID] = &explorationSession{prepared: prepared}
	service.mu.Unlock()
	return loadSpecResult{SessionID: sessionID, SnapshotID: 0, SpecParameters: parameters}, nil
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
	_, found := service.sessions[params.SessionID]
	if found {
		delete(service.sessions, params.SessionID)
	}
	service.mu.Unlock()
	if !found {
		return nil, &jsonRPCError{Code: JSONRPCSessionMissing, Message: "Session not found: " + params.SessionID}
	}
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
