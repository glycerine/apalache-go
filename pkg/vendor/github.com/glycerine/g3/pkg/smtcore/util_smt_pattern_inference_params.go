package smtcore

// Source: src/ast/pattern/pattern_inference_params.h:30-50;
// src/ast/pattern/pattern_inference_params.cpp:22-47; src/ast/pattern/pattern_inference_params_helper.pyg:1-12.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Initialized is cold diagnostic metadata; unassigned scalar fields do not
// become source defaults merely because their Go representation contains zero.
type SMTPatternInferenceState struct {
	MaxMultiPatterns          uint32
	BlockLoopPatterns         bool
	Arithmetic                uint32
	UseDatabase               bool
	ArithmeticWeight          uint32
	NonNestedArithmeticWeight uint32
	PullQuantifiers           bool
	NoPatternWeight           int32
	AvoidSkolems              bool
	Warnings                  bool
	Initialized               uint16
}
type SMTPatternInferenceParams struct {
	uparams.ModuleParamsOwner
	self                     *SMTPatternInferenceParams
	state                    SMTPatternInferenceState
	header                   uint64
	closed, constructorReady bool
	constructorCause         error
}

func NewSMTPatternInferenceParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTPatternInferenceParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTPatternInferenceParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTPatternInferenceParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same actual constructor engine for standalone and embedded source bases. The
// containing object charges the embedded fields once; no extra wrapper/header.
func (s *SMTPatternInferenceParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	s.ModuleParamsOwner = uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}
	s.state = SMTPatternInferenceState{NoPatternWeight: -1, AvoidSkolems: true, Initialized: 0x180}
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

func (s *SMTPatternInferenceParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTPatternInferenceParams) State() (SMTPatternInferenceState, error) {
	if e := s.open(); e != nil {
		return SMTPatternInferenceState{}, e
	}
	return s.state, nil
}
func (s *SMTPatternInferenceParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(p)
}
func (s *SMTPatternInferenceParams) update(local *uparams.ParamsRef) (result error) {
	if e := s.open(); e != nil {
		return e
	}
	if s.Helpers != nil || s.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	s.Running = true
	defer func() { s.Running = false }()
	p, e := s.BeginHelper(local, "pi")
	if p != nil {
		defer func() { result = s.FinishHelper(p, result) }()
	}
	if e != nil {
		return e
	}
	vMaxMultiPatterns, e := s.FrameUInt(p, "max_multi_patterns", 0)
	if e != nil {
		return e
	}
	s.state.MaxMultiPatterns = vMaxMultiPatterns
	s.state.Initialized |= 1 << 0
	vBlockLoopPatterns, e := s.FrameBool(p, "block_loop_patterns", true)
	if e != nil {
		return e
	}
	s.state.BlockLoopPatterns = vBlockLoopPatterns
	s.state.Initialized |= 1 << 1
	vArithmetic, e := s.FrameUInt(p, "arith", 1)
	if e != nil {
		return e
	}
	s.state.Arithmetic = vArithmetic
	s.state.Initialized |= 1 << 2
	vUseDatabase, e := s.FrameBool(p, "use_database", false)
	if e != nil {
		return e
	}
	s.state.UseDatabase = vUseDatabase
	s.state.Initialized |= 1 << 3
	vArithmeticWeight, e := s.FrameUInt(p, "arith_weight", 5)
	if e != nil {
		return e
	}
	s.state.ArithmeticWeight = vArithmeticWeight
	s.state.Initialized |= 1 << 4
	vNonNestedArithmeticWeight, e := s.FrameUInt(p, "non_nested_arith_weight", 10)
	if e != nil {
		return e
	}
	s.state.NonNestedArithmeticWeight = vNonNestedArithmeticWeight
	s.state.Initialized |= 1 << 5
	vPullQuantifiers, e := s.FrameBool(p, "pull_quantifiers", true)
	if e != nil {
		return e
	}
	s.state.PullQuantifiers = vPullQuantifiers
	s.state.Initialized |= 1 << 6
	vWarnings, e := s.FrameBool(p, "warnings", false)
	if e != nil {
		return e
	}
	s.state.Warnings = vWarnings
	s.state.Initialized |= 1 << 9
	return nil
}
func (s *SMTPatternInferenceParams) Close() error {
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
func (s *SMTPatternInferenceParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := out.AppendCString("m_pi_max_multi_patterns="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxMultiPatterns); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_block_loop_patterns="); e != nil {
		return e
	}
	bBlockLoopPatterns := int64(0)
	if s.state.BlockLoopPatterns {
		bBlockLoopPatterns = 1
	}
	if e := out.AppendInt64(bBlockLoopPatterns); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_arith="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.Arithmetic)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_use_database="); e != nil {
		return e
	}
	bUseDatabase := int64(0)
	if s.state.UseDatabase {
		bUseDatabase = 1
	}
	if e := out.AppendInt64(bUseDatabase); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_arith_weight="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.ArithmeticWeight); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_non_nested_arith_weight="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NonNestedArithmeticWeight); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_pull_quantifiers="); e != nil {
		return e
	}
	bPullQuantifiers := int64(0)
	if s.state.PullQuantifiers {
		bPullQuantifiers = 1
	}
	if e := out.AppendInt64(bPullQuantifiers); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_nopat_weight="); e != nil {
		return e
	}
	if e := out.AppendInt32(s.state.NoPatternWeight); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_avoid_skolems="); e != nil {
		return e
	}
	bAvoidSkolems := int64(0)
	if s.state.AvoidSkolems {
		bAvoidSkolems = 1
	}
	if e := out.AppendInt64(bAvoidSkolems); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_pi_warnings="); e != nil {
		return e
	}
	bWarnings := int64(0)
	if s.state.Warnings {
		bWarnings = 1
	}
	if e := out.AppendInt64(bWarnings); e != nil {
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
