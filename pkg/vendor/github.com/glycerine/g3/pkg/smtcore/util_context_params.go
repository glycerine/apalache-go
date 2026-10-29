package smtcore

// Source: src/cmd_context/context_params.h:26-55;
// src/cmd_context/context_params.cpp:26-40,137-156.

import (
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// InterpolantsInitialized remains false: the source never initializes m_interpolants.
// State of a failed constructor is diagnostic, not a callable source object.
type ContextParamsState struct {
	AutoConfig, Proof, Interpolants, InterpolantsInitialized, DebugRefCount, Trace, WellSortedCheck, Model, ModelValidate, DumpModels, UnsatCore, SMTLib2Compliant bool
	Timeout, RLimit                                                                                                                                                uint32
	DefaultsInitialized                                                                                                                                            bool
}
type ContextParams struct {
	self                       *ContextParams
	context                    *uparams.ParamsContext
	globals                    *uparams.GlobalParamsService
	budget                     *arena.Budget
	dotProof, traceFile        string
	state                      ContextParamsState
	header                     uint64
	ready, closed, executing   bool
	constructorCause           error
	bodyCause, destructorCause error
	frames                     *contextParamsFrame
	nextFrame                  uint64
}

func NewContextParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, b *arena.Budget) (*ContextParams, error) {
	return newContextParams(c, g, b, nil)
}

// NewContextParamsWithAllocationHook is used by the retained API alloc
// wrapper. The hook runs after the header allocation and before construction
// of the first string member, matching memory::allocate before placement new.
func NewContextParamsWithAllocationHook(c *uparams.ParamsContext, g *uparams.GlobalParamsService, b *arena.Budget, onAllocated func(*ContextParams)) (*ContextParams, error) {
	return newContextParams(c, g, b, onAllocated)
}

// Clone is the compiler-generated context_params copy used by api::context.
// It copies the current scalar and string values without rereading globals.
func (s *ContextParams) Clone() (*ContextParams, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if !s.ready {
		return nil, uparams.ErrParamsPrecondition
	}
	n := uint64(reflect.TypeOf(ContextParams{}).Size())
	if e := s.budget.Reserve(n); e != nil {
		return nil, e
	}
	copy := &ContextParams{
		context: s.context, globals: s.globals, budget: s.budget,
		dotProof: s.dotProof, traceFile: s.traceFile, state: s.state,
		header: n, ready: true,
	}
	copy.self = copy
	return copy, nil
}

func newContextParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, b *arena.Budget, onAllocated func(*ContextParams)) (*ContextParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(ContextParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &ContextParams{context: c, globals: g, budget: b, header: n}
	s.self = s
	if onAllocated != nil {
		onAllocated(s)
	}
	s.state.UnsatCore = false
	s.state.Model = true
	s.state.ModelValidate = false
	s.state.DumpModels = false
	s.state.AutoConfig = true
	s.state.Proof = false
	s.state.Trace = false
	s.state.DebugRefCount = false
	s.state.SMTLib2Compliant = false
	s.state.WellSortedCheck = false
	s.state.Timeout = ^uint32(0)
	s.state.RLimit = 0
	s.state.DefaultsInitialized = true
	s.executing = true
	p, e := g.SourceGetRef()
	if e == nil {
		e = s.update(p)
	}
	s.executing = false
	if e != nil {
		s.constructorCause = e
	} else {
		s.ready = true
	}
	return s, e
}
func (s *ContextParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.executing {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *ContextParams) State() (ContextParamsState, error) {
	if e := s.open(); e != nil {
		return ContextParamsState{}, e
	}
	return s.state, nil
}
func (s *ContextParams) TraceFile() string {
	if e := s.open(); e != nil {
		panic(e)
	}
	return s.traceFile
}
func (s *ContextParams) DotProofFile() string {
	if e := s.open(); e != nil {
		panic(e)
	}
	return s.dotProof
}

func sourceCStringPrefix(text string) string {
	if i := strings.IndexByte(text, 0); i >= 0 {
		return text[:i]
	}
	return text
}
func (s *ContextParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.ready {
		return uparams.ErrParamsPrecondition
	}
	s.executing = true
	defer func() { s.executing = false }()
	return s.update(p)
}
func (s *ContextParams) UpdateGlobal() error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.ready {
		return uparams.ErrParamsPrecondition
	}
	s.executing = true
	defer func() { s.executing = false }()
	p, e := s.globals.SourceGetRef()
	if e != nil {
		return e
	}
	return s.update(p)
}
func (s *ContextParams) SetRLimit(n uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.ready {
		return uparams.ErrParamsPrecondition
	}
	s.state.RLimit = n
	return nil
}
func (s *ContextParams) update(p *uparams.ParamsRef) error {
	kTimeout := "timeout"
	vTimeout, e := s.context.GetUIntName(p, &kTimeout, s.state.Timeout)
	if e != nil {
		return e
	}
	s.state.Timeout = vTimeout
	kRLimit := "rlimit"
	vRLimit, e := s.context.GetUIntName(p, &kRLimit, s.state.RLimit)
	if e != nil {
		return e
	}
	s.state.RLimit = vRLimit
	kInner := "well_sorted_check"
	inner, e := s.context.GetBoolName(p, &kInner, s.state.WellSortedCheck)
	if e != nil {
		return e
	}
	kOuter := "type_check"
	outer, e := s.context.GetBoolName(p, &kOuter, inner)
	if e != nil {
		return e
	}
	s.state.WellSortedCheck = outer
	kAutoConfig := "auto_config"
	vAutoConfig, e := s.context.GetBoolName(p, &kAutoConfig, s.state.AutoConfig)
	if e != nil {
		return e
	}
	s.state.AutoConfig = vAutoConfig
	kProof := "proof"
	vProof, e := s.context.GetBoolName(p, &kProof, s.state.Proof)
	if e != nil {
		return e
	}
	s.state.Proof = vProof
	kModel := "model"
	vModel, e := s.context.GetBoolName(p, &kModel, s.state.Model)
	if e != nil {
		return e
	}
	s.state.Model = vModel
	kModelValidate := "model_validate"
	vModelValidate, e := s.context.GetBoolName(p, &kModelValidate, s.state.ModelValidate)
	if e != nil {
		return e
	}
	s.state.ModelValidate = vModelValidate
	kDumpModels := "dump_models"
	vDumpModels, e := s.context.GetBoolName(p, &kDumpModels, s.state.DumpModels)
	if e != nil {
		return e
	}
	s.state.DumpModels = vDumpModels
	kTrace := "trace"
	vTrace, e := s.context.GetBoolName(p, &kTrace, s.state.Trace)
	if e != nil {
		return e
	}
	s.state.Trace = vTrace
	ktraceFile := "trace_file_name"
	dtraceFile := "z3.log"
	vtraceFile, e := s.context.GetStringViewFallbackName(p, &ktraceFile, nil, &dtraceFile)
	if e != nil {
		return e
	}
	traceFileText, e := vtraceFile.String()
	if e != nil {
		return e
	}
	s.traceFile = sourceCStringPrefix(traceFileText)
	kdotProof := "dot_proof_file"
	ddotProof := "proof.dot"
	vdotProof, e := s.context.GetStringViewFallbackName(p, &kdotProof, nil, &ddotProof)
	if e != nil {
		return e
	}
	dotProofText, e := vdotProof.String()
	if e != nil {
		return e
	}
	s.dotProof = sourceCStringPrefix(dotProofText)
	kUnsatCore := "unsat_core"
	vUnsatCore, e := s.context.GetBoolName(p, &kUnsatCore, s.state.UnsatCore)
	if e != nil {
		return e
	}
	s.state.UnsatCore = vUnsatCore
	kDebugRefCount := "debug_ref_count"
	vDebugRefCount, e := s.context.GetBoolName(p, &kDebugRefCount, s.state.DebugRefCount)
	if e != nil {
		return e
	}
	s.state.DebugRefCount = vDebugRefCount
	kSMTLib2Compliant := "smtlib2_compliant"
	vSMTLib2Compliant, e := s.context.GetBoolName(p, &kSMTLib2Compliant, s.state.SMTLib2Compliant)
	if e != nil {
		return e
	}
	s.state.SMTLib2Compliant = vSMTLib2Compliant
	return nil
}
func (s *ContextParams) Close() error {
	if s == nil || s.self != s {
		return uparams.ErrParamsOwner
	}
	if s.closed {
		return nil
	}
	if s.executing {
		return uparams.ErrParamsPrecondition
	}
	if e := s.closeSourceFrames(); e != nil {
		return e
	}
	s.traceFile = ""
	s.dotProof = ""
	s.budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
