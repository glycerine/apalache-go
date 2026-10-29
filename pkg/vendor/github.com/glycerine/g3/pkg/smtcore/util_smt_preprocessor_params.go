package smtcore

// Source: src/smt/params/preprocessor_params.h:31-81;
// src/smt/params/preprocessor_params.cpp:22-61; src/smt/params/smt_params_helper.pyg:9-11,19-20.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type SMTPreprocessorState struct {
	LiftITE                        uint32
	NonGroundLiftITE               uint32
	PullCheapITE                   bool
	PullNestedQuantifiers          bool
	EliminateTermITE               bool
	MacroFinder                    bool
	PropagateValues                bool
	PropagateBooleans              bool
	RefineInjectivityAxiom         bool
	EliminateBounds                bool
	SimplifyBitToInt               bool
	NNFCNF                         bool
	DistributeForall               bool
	ReduceArguments                bool
	QuasiMacros                    bool
	RestrictedQuasiMacros          bool
	MaxBVSharing                   bool
	PreSimplifier                  bool
	NonlinearQuantifierElimination bool
	DefaultsInitialized            bool
}
type SMTPreprocessorParams struct {
	uparams.ModuleParamsOwner
	self                                 *SMTPreprocessorParams
	pattern                              SMTPatternInferenceParams
	bit                                  bit_blaster.SMTBitBlasterParams
	state                                SMTPreprocessorState
	empty                                uparams.ParamsRef
	header                               uint64
	completedBases                       uint8
	bitEnded, patternEnded               bool
	closed, constructorReady, activeCall bool
	constructorCause                     error
}

func NewSMTPreprocessorParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTPreprocessorParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTPreprocessorParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTPreprocessorParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTPreprocessorParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTPreprocessorParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header, empty: c.EmptyValue()}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	s.activeCall = true
	// Source implicit base constructor uses its own default empty temporary.
	e := s.pattern.constructSource(c, &s.empty, g, b)
	if e != nil {
		s.constructorCause = e
		s.activeCall = false
		return e
	}
	s.completedBases = 1
	s.bit = bit_blaster.NewSMTBitBlasterParams()
	s.completedBases |= 2
	s.state = SMTPreprocessorState{0, 0, false, false, false, false, true, false, true, false, false, true, false, false, false, false, true, true, false, true}
	e = s.updateLocal(p)
	if e != nil {
		s.constructorCause = e
		if s.DestructorCause == nil && s.pattern.DestructorCause == nil {
			s.sourceUnwindBases()
		}
	} else {
		s.constructorReady = true
	}
	s.activeCall = false
	return e
}

// Trivial base lifetime ends are source actions, not owning helper abort.
func (s *SMTPreprocessorParams) sourceUnwindBases() {
	if s.completedBases&2 != 0 && !s.bitEnded {
		s.bitEnded = true
	}
	if s.completedBases&1 != 0 && !s.patternEnded {
		s.pattern.closed = true
		s.patternEnded = true
	}
}
func (s *SMTPreprocessorParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running || s.activeCall {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTPreprocessorParams) State() (SMTPreprocessorState, error) {
	if e := s.open(); e != nil {
		return SMTPreprocessorState{}, e
	}
	return s.state, nil
}
func (s *SMTPreprocessorParams) PatternState() (SMTPatternInferenceState, error) {
	if e := s.open(); e != nil {
		return SMTPatternInferenceState{}, e
	}
	return s.pattern.state, nil
}
func (s *SMTPreprocessorParams) BitBlasterState() (bit_blaster.SMTBitBlasterParams, bool, error) {
	if e := s.open(); e != nil {
		return bit_blaster.SMTBitBlasterParams{}, false, e
	}
	return s.bit, s.completedBases&2 != 0, nil
}
func (s *SMTPreprocessorParams) UpdateLocal(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	// Exact retained lexical owners bar a new mutation; read-only State remains diagnostic.
	if s.Helpers != nil || s.DestructorCause != nil || s.pattern.Helpers != nil || s.pattern.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	return s.updateLocal(p)
}
func (s *SMTPreprocessorParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	// Exact retained lexical owners bar a new mutation; read-only State remains diagnostic.
	if s.Helpers != nil || s.DestructorCause != nil || s.pattern.Helpers != nil || s.pattern.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	if e := s.pattern.Update(p); e != nil {
		return e
	}
	return s.updateLocal(p)
}
func (s *SMTPreprocessorParams) updateLocal(local *uparams.ParamsRef) (result error) {
	if !s.activeCall || s.Helpers != nil || s.DestructorCause != nil {
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
	vMacroFinder, e := s.FrameBool(p, "macro_finder", false)
	if e != nil {
		return e
	}
	s.state.MacroFinder = vMacroFinder
	vQuasiMacros, e := s.FrameBool(p, "quasi_macros", false)
	if e != nil {
		return e
	}
	s.state.QuasiMacros = vQuasiMacros
	vRestrictedQuasiMacros, e := s.FrameBool(p, "restricted_quasi_macros", false)
	if e != nil {
		return e
	}
	s.state.RestrictedQuasiMacros = vRestrictedQuasiMacros
	vPullNestedQuantifiers, e := s.FrameBool(p, "pull_nested_quantifiers", false)
	if e != nil {
		return e
	}
	s.state.PullNestedQuantifiers = vPullNestedQuantifiers
	vRefineInjectivityAxiom, e := s.FrameBool(p, "refine_inj_axioms", true)
	if e != nil {
		return e
	}
	s.state.RefineInjectivityAxiom = vRefineInjectivityAxiom
	return nil
}
func (s *SMTPreprocessorParams) Close() error {
	if s == nil || s.self != s {
		return uparams.ErrParamsOwner
	}
	if s.closed {
		return nil
	}
	if s.Running || s.activeCall {
		return uparams.ErrParamsPrecondition
	}
	// Finish only exact retained lexical leaves before any older base lifetime.
	if e := s.CloseHelpers(); e != nil {
		return e
	}
	if s.pattern.self != nil {
		if e := s.pattern.CloseHelpers(); e != nil {
			return e
		}
	}
	if s.completedBases&2 != 0 && !s.bitEnded {
		s.bitEnded = true
	}
	if s.pattern.self != nil {
		if e := s.pattern.Close(); e != nil {
			return e
		}
	}
	if s.completedBases&1 != 0 {
		s.patternEnded = true
	}
	s.Budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *SMTPreprocessorParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	if e := s.pattern.Display(out); e != nil {
		return e
	}
	if e := s.bit.Display(out); e != nil {
		return e
	}
	if e := out.AppendCString("m_lift_ite="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.LiftITE)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_ng_lift_ite="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.NonGroundLiftITE)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pull_cheap_ite="); e != nil {
		return e
	}
	bPullCheapITE := int64(0)
	if s.state.PullCheapITE {
		bPullCheapITE = 1
	}
	if e := out.AppendInt64(bPullCheapITE); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pull_nested_quantifiers="); e != nil {
		return e
	}
	bPullNestedQuantifiers := int64(0)
	if s.state.PullNestedQuantifiers {
		bPullNestedQuantifiers = 1
	}
	if e := out.AppendInt64(bPullNestedQuantifiers); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_eliminate_term_ite="); e != nil {
		return e
	}
	bEliminateTermITE := int64(0)
	if s.state.EliminateTermITE {
		bEliminateTermITE = 1
	}
	if e := out.AppendInt64(bEliminateTermITE); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_macro_finder="); e != nil {
		return e
	}
	bMacroFinder := int64(0)
	if s.state.MacroFinder {
		bMacroFinder = 1
	}
	if e := out.AppendInt64(bMacroFinder); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_propagate_values="); e != nil {
		return e
	}
	bPropagateValues := int64(0)
	if s.state.PropagateValues {
		bPropagateValues = 1
	}
	if e := out.AppendInt64(bPropagateValues); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_propagate_booleans="); e != nil {
		return e
	}
	bPropagateBooleans := int64(0)
	if s.state.PropagateBooleans {
		bPropagateBooleans = 1
	}
	if e := out.AppendInt64(bPropagateBooleans); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_refine_inj_axiom="); e != nil {
		return e
	}
	bRefineInjectivityAxiom := int64(0)
	if s.state.RefineInjectivityAxiom {
		bRefineInjectivityAxiom = 1
	}
	if e := out.AppendInt64(bRefineInjectivityAxiom); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_eliminate_bounds="); e != nil {
		return e
	}
	bEliminateBounds := int64(0)
	if s.state.EliminateBounds {
		bEliminateBounds = 1
	}
	if e := out.AppendInt64(bEliminateBounds); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_simplify_bit2int="); e != nil {
		return e
	}
	bSimplifyBitToInt := int64(0)
	if s.state.SimplifyBitToInt {
		bSimplifyBitToInt = 1
	}
	if e := out.AppendInt64(bSimplifyBitToInt); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nnf_cnf="); e != nil {
		return e
	}
	bNNFCNF := int64(0)
	if s.state.NNFCNF {
		bNNFCNF = 1
	}
	if e := out.AppendInt64(bNNFCNF); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_distribute_forall="); e != nil {
		return e
	}
	bDistributeForall := int64(0)
	if s.state.DistributeForall {
		bDistributeForall = 1
	}
	if e := out.AppendInt64(bDistributeForall); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_reduce_args="); e != nil {
		return e
	}
	bReduceArguments := int64(0)
	if s.state.ReduceArguments {
		bReduceArguments = 1
	}
	if e := out.AppendInt64(bReduceArguments); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_quasi_macros="); e != nil {
		return e
	}
	bQuasiMacros := int64(0)
	if s.state.QuasiMacros {
		bQuasiMacros = 1
	}
	if e := out.AppendInt64(bQuasiMacros); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restricted_quasi_macros="); e != nil {
		return e
	}
	bRestrictedQuasiMacros := int64(0)
	if s.state.RestrictedQuasiMacros {
		bRestrictedQuasiMacros = 1
	}
	if e := out.AppendInt64(bRestrictedQuasiMacros); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_max_bv_sharing="); e != nil {
		return e
	}
	bMaxBVSharing := int64(0)
	if s.state.MaxBVSharing {
		bMaxBVSharing = 1
	}
	if e := out.AppendInt64(bMaxBVSharing); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pre_simplifier="); e != nil {
		return e
	}
	bPreSimplifier := int64(0)
	if s.state.PreSimplifier {
		bPreSimplifier = 1
	}
	if e := out.AppendInt64(bPreSimplifier); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_nlquant_elim="); e != nil {
		return e
	}
	bNonlinearQuantifierElimination := int64(0)
	if s.state.NonlinearQuantifierElimination {
		bNonlinearQuantifierElimination = 1
	}
	if e := out.AppendInt64(bNonlinearQuantifierElimination); e != nil {
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
