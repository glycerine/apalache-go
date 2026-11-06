package smtcore

// Source: src/smt/params/theory_arith_params.h:24-164;
// src/smt/params/theory_arith_params.cpp:23-94; src/smt/params/smt_params_helper.pyg:7,40-53;
// src/ast/rewriter/arith_rewriter_params.pyg:1-15; scripts/mk_genfile_common.py:895-939.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type SMTArithState struct {
	Eq2Ineq                      bool
	ProcessAllEqs                bool
	Mode                         uint32
	AutoConfigSimplex            bool
	BlandsRuleThreshold          uint32
	PropagateEqs                 bool
	BoundProp                    uint32
	StrongerLemmas               bool
	SkipRowsWithBigCoeffs        bool
	MaxLemmaSize                 uint32
	SmallLemmaSize               uint32
	Reflect                      bool
	IgnoreInt                    bool
	LazyPivotingLevel            uint32
	RandomSeed                   uint32
	RandomInitialValue           bool
	RandomLower                  int32
	RandomUpper                  int32
	Adaptive                     bool
	AdaptiveAssertionThreshold   float64
	AdaptivePropagationThreshold float64
	DumpLemmas                   bool
	EagerEqAxioms                bool
	BranchCutRatio               uint32
	IntEqBranching               bool
	EnumConstMod                 bool
	GCDTest                      bool
	EagerGCD                     bool
	AdaptiveGCD                  bool
	PropagationThreshold         uint32
	PivotStrategy                uint32
	AddBinaryBounds              bool
	PropagationStrategy          uint32
	EqBounds                     bool
	LazyAdapter                  bool
	Fixnum                       bool
	IntOnly                      bool
	NLArith                      bool
	NLArithGB                    bool
	NLArithGBThreshold           uint32
	NLArithGBEqs                 bool
	NLArithGBPerturbate          bool
	NLArithMaxDegree             uint32
	NLArithBranching             bool
	NLArithRounds                uint32
	EuclideanSolver              bool
}
type SMTArithParams struct {
	uparams.ModuleParamsOwner
	self                     *SMTArithParams
	state                    SMTArithState
	header                   uint64
	closed, constructorReady bool
	constructorCause         error
}

func NewSMTArithParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTArithParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTArithParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTArithParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTArithParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTArithParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header, state: SMTArithState{Eq2Ineq: false, ProcessAllEqs: false, Mode: 2, AutoConfigSimplex: false, BlandsRuleThreshold: 1000, PropagateEqs: true, BoundProp: 2, StrongerLemmas: true, SkipRowsWithBigCoeffs: true, MaxLemmaSize: 128, SmallLemmaSize: 16, Reflect: true, IgnoreInt: false, LazyPivotingLevel: 0, RandomSeed: 0, RandomInitialValue: false, RandomLower: -1000, RandomUpper: 1000, Adaptive: false, AdaptiveAssertionThreshold: 0.2, AdaptivePropagationThreshold: 0.4, DumpLemmas: false, EagerEqAxioms: true, BranchCutRatio: 2, IntEqBranching: false, EnumConstMod: false, GCDTest: true, EagerGCD: false, AdaptiveGCD: false, PropagationThreshold: 4294967295, PivotStrategy: 0, AddBinaryBounds: false, PropagationStrategy: 1, EqBounds: false, LazyAdapter: false, Fixnum: false, IntOnly: false, NLArith: true, NLArithGB: true, NLArithGBThreshold: 512, NLArithGBEqs: false, NLArithGBPerturbate: true, NLArithMaxDegree: 6, NLArithBranching: true, NLArithRounds: 1024, EuclideanSolver: false}}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	e := s.update(p)
	if e != nil {
		s.constructorCause = e
	} else {
		s.constructorReady = true
	}
	return e
}
func (s *SMTArithParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTArithParams) State() (SMTArithState, error) {
	if e := s.open(); e != nil {
		return SMTArithState{}, e
	}
	return s.state, nil
}
func (s *SMTArithParams) Update(local *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(local)
}
func (s *SMTArithParams) update(local *uparams.ParamsRef) (result error) {
	if e := s.open(); e != nil {
		return e
	}
	if s.Helpers != nil || s.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	s.Running = true
	defer func() { s.Running = false }()
	p, e := s.BeginHelper(local, "smt")
	if p != nil {
		defer func() { result = s.FinishHelper(p, result) }()
	}
	if e != nil {
		return e
	}
	v0, e := s.FrameBool(p, "arith.random_initial_value", false)
	if e != nil {
		return e
	}
	s.state.RandomInitialValue = v0
	v1, e := s.FrameUInt(p, "random_seed", 0)
	if e != nil {
		return e
	}
	s.state.RandomSeed = v1
	v2, e := s.FrameUInt(p, "arith.solver", 2)
	if e != nil {
		return e
	}
	s.state.Mode = v2
	v3, e := s.FrameBool(p, "arith.nl", true)
	if e != nil {
		return e
	}
	s.state.NLArith = v3
	v4, e := s.FrameBool(p, "arith.nl.gb", true)
	if e != nil {
		return e
	}
	s.state.NLArithGB = v4
	v5, e := s.FrameBool(p, "arith.nl.branching", true)
	if e != nil {
		return e
	}
	s.state.NLArithBranching = v5
	v6, e := s.FrameUInt(p, "arith.nl.rounds", 1024)
	if e != nil {
		return e
	}
	s.state.NLArithRounds = v6
	v7, e := s.FrameBool(p, "arith.euclidean_solver", false)
	if e != nil {
		return e
	}
	s.state.EuclideanSolver = v7
	v8, e := s.FrameBool(p, "arith.propagate_eqs", true)
	if e != nil {
		return e
	}
	s.state.PropagateEqs = v8
	v9, e := s.FrameUInt(p, "arith.branch_cut_ratio", 2)
	if e != nil {
		return e
	}
	s.state.BranchCutRatio = v9
	v10, e := s.FrameBool(p, "arith.int_eq_branch", false)
	if e != nil {
		return e
	}
	s.state.IntEqBranching = v10
	v11, e := s.FrameBool(p, "arith.ignore_int", false)
	if e != nil {
		return e
	}
	s.state.IgnoreInt = v11
	v12, e := s.FrameUInt(p, "arith.propagation_mode", 2)
	if e != nil {
		return e
	}
	s.state.BoundProp = v12
	v13, e := s.FrameBool(p, "arith.dump_lemmas", false)
	if e != nil {
		return e
	}
	s.state.DumpLemmas = v13
	v14, e := s.FrameBool(p, "arith.reflect", true)
	if e != nil {
		return e
	}
	s.state.Reflect = v14
	ap, e := s.BeginHelper(local, "rewriter")
	if ap != nil {
		defer func() { result = s.FinishHelper(ap, result) }()
	}
	if e != nil {
		return e
	}
	eq, e := s.FrameBool(ap, "eq2ineq", false)
	if e != nil {
		return e
	}
	s.state.Eq2Ineq = eq
	return nil
}
func (s *SMTArithParams) Close() error {
	if s == nil || s.self != s {
		return uparams.ErrParamsOwner
	}
	if s.closed {
		return nil
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	if e := s.CloseHelpers(); e != nil {
		return e
	}
	s.Budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *SMTArithParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := out.AppendCString("m_arith_eq2ineq="); e != nil {
		return e
	}
	bEq2Ineq := int64(0)
	if s.state.Eq2Ineq {
		bEq2Ineq = 1
	}
	if e := out.AppendInt64(bEq2Ineq); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_process_all_eqs="); e != nil {
		return e
	}
	bProcessAllEqs := int64(0)
	if s.state.ProcessAllEqs {
		bProcessAllEqs = 1
	}
	if e := out.AppendInt64(bProcessAllEqs); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_mode="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.Mode)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_auto_config_simplex="); e != nil {
		return e
	}
	bAutoConfigSimplex := int64(0)
	if s.state.AutoConfigSimplex {
		bAutoConfigSimplex = 1
	}
	if e := out.AppendInt64(bAutoConfigSimplex); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_blands_rule_threshold="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.BlandsRuleThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_propagate_eqs="); e != nil {
		return e
	}
	bPropagateEqs := int64(0)
	if s.state.PropagateEqs {
		bPropagateEqs = 1
	}
	if e := out.AppendInt64(bPropagateEqs); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_bound_prop="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.BoundProp)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_stronger_lemmas="); e != nil {
		return e
	}
	bStrongerLemmas := int64(0)
	if s.state.StrongerLemmas {
		bStrongerLemmas = 1
	}
	if e := out.AppendInt64(bStrongerLemmas); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_skip_rows_with_big_coeffs="); e != nil {
		return e
	}
	bSkipRowsWithBigCoeffs := int64(0)
	if s.state.SkipRowsWithBigCoeffs {
		bSkipRowsWithBigCoeffs = 1
	}
	if e := out.AppendInt64(bSkipRowsWithBigCoeffs); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_max_lemma_size="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxLemmaSize); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_small_lemma_size="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.SmallLemmaSize); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_reflect="); e != nil {
		return e
	}
	bReflect := int64(0)
	if s.state.Reflect {
		bReflect = 1
	}
	if e := out.AppendInt64(bReflect); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_ignore_int="); e != nil {
		return e
	}
	bIgnoreInt := int64(0)
	if s.state.IgnoreInt {
		bIgnoreInt = 1
	}
	if e := out.AppendInt64(bIgnoreInt); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_lazy_pivoting_lvl="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.LazyPivotingLevel); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_random_seed="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.RandomSeed); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_random_initial_value="); e != nil {
		return e
	}
	bRandomInitialValue := int64(0)
	if s.state.RandomInitialValue {
		bRandomInitialValue = 1
	}
	if e := out.AppendInt64(bRandomInitialValue); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_random_lower="); e != nil {
		return e
	}
	if e := out.AppendInt32(s.state.RandomLower); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_random_upper="); e != nil {
		return e
	}
	if e := out.AppendInt32(s.state.RandomUpper); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_adaptive="); e != nil {
		return e
	}
	bAdaptive := int64(0)
	if s.state.Adaptive {
		bAdaptive = 1
	}
	if e := out.AppendInt64(bAdaptive); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_adaptive_assertion_threshold="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.AdaptiveAssertionThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_adaptive_propagation_threshold="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.AdaptivePropagationThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_dump_lemmas="); e != nil {
		return e
	}
	bDumpLemmas := int64(0)
	if s.state.DumpLemmas {
		bDumpLemmas = 1
	}
	if e := out.AppendInt64(bDumpLemmas); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_eager_eq_axioms="); e != nil {
		return e
	}
	bEagerEqAxioms := int64(0)
	if s.state.EagerEqAxioms {
		bEagerEqAxioms = 1
	}
	if e := out.AppendInt64(bEagerEqAxioms); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_branch_cut_ratio="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.BranchCutRatio); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_int_eq_branching="); e != nil {
		return e
	}
	bIntEqBranching := int64(0)
	if s.state.IntEqBranching {
		bIntEqBranching = 1
	}
	if e := out.AppendInt64(bIntEqBranching); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_enum_const_mod="); e != nil {
		return e
	}
	bEnumConstMod := int64(0)
	if s.state.EnumConstMod {
		bEnumConstMod = 1
	}
	if e := out.AppendInt64(bEnumConstMod); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_gcd_test="); e != nil {
		return e
	}
	bGCDTest := int64(0)
	if s.state.GCDTest {
		bGCDTest = 1
	}
	if e := out.AppendInt64(bGCDTest); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_eager_gcd="); e != nil {
		return e
	}
	bEagerGCD := int64(0)
	if s.state.EagerGCD {
		bEagerGCD = 1
	}
	if e := out.AppendInt64(bEagerGCD); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_adaptive_gcd="); e != nil {
		return e
	}
	bAdaptiveGCD := int64(0)
	if s.state.AdaptiveGCD {
		bAdaptiveGCD = 1
	}
	if e := out.AppendInt64(bAdaptiveGCD); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_propagation_threshold="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.PropagationThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_pivot_strategy="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.PivotStrategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_add_binary_bounds="); e != nil {
		return e
	}
	bAddBinaryBounds := int64(0)
	if s.state.AddBinaryBounds {
		bAddBinaryBounds = 1
	}
	if e := out.AppendInt64(bAddBinaryBounds); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_propagation_strategy="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.PropagationStrategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_eq_bounds="); e != nil {
		return e
	}
	bEqBounds := int64(0)
	if s.state.EqBounds {
		bEqBounds = 1
	}
	if e := out.AppendInt64(bEqBounds); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_lazy_adapter="); e != nil {
		return e
	}
	bLazyAdapter := int64(0)
	if s.state.LazyAdapter {
		bLazyAdapter = 1
	}
	if e := out.AppendInt64(bLazyAdapter); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_fixnum="); e != nil {
		return e
	}
	bFixnum := int64(0)
	if s.state.Fixnum {
		bFixnum = 1
	}
	if e := out.AppendInt64(bFixnum); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_int_only="); e != nil {
		return e
	}
	bIntOnly := int64(0)
	if s.state.IntOnly {
		bIntOnly = 1
	}
	if e := out.AppendInt64(bIntOnly); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith="); e != nil {
		return e
	}
	bNLArith := int64(0)
	if s.state.NLArith {
		bNLArith = 1
	}
	if e := out.AppendInt64(bNLArith); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_gb="); e != nil {
		return e
	}
	bNLArithGB := int64(0)
	if s.state.NLArithGB {
		bNLArithGB = 1
	}
	if e := out.AppendInt64(bNLArithGB); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_gb_threshold="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NLArithGBThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_gb_eqs="); e != nil {
		return e
	}
	bNLArithGBEqs := int64(0)
	if s.state.NLArithGBEqs {
		bNLArithGBEqs = 1
	}
	if e := out.AppendInt64(bNLArithGBEqs); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_gb_perturbate="); e != nil {
		return e
	}
	bNLArithGBPerturbate := int64(0)
	if s.state.NLArithGBPerturbate {
		bNLArithGBPerturbate = 1
	}
	if e := out.AppendInt64(bNLArithGBPerturbate); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_max_degree="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NLArithMaxDegree); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_branching="); e != nil {
		return e
	}
	bNLArithBranching := int64(0)
	if s.state.NLArithBranching {
		bNLArithBranching = 1
	}
	if e := out.AppendInt64(bNLArithBranching); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nl_arith_rounds="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NLArithRounds); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_arith_euclidean_solver="); e != nil {
		return e
	}
	bEuclideanSolver := int64(0)
	if s.state.EuclideanSolver {
		bEuclideanSolver = 1
	}
	if e := out.AppendInt64(bEuclideanSolver); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	return nil
}
