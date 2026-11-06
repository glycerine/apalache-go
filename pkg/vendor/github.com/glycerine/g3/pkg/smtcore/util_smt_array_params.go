package smtcore

// Source: src/smt/params/theory_array_params.h:24-59,61-75;
// src/smt/params/theory_array_params.cpp:22-40; src/smt/params/smt_params_helper.pyg:59-60.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// These are the actual eleven scalar source members, in declaration order.
// The standalone facade below adds only its real Go owning/lifetime metadata.
type SMTArrayState struct {
	CanonizeSimplify, Simplify                   bool
	Mode                                         uint32
	Weak, Extensional                            bool
	Laziness                                     uint32
	DelayExpAxiom, CG, AlwaysPropUpward, LazyIEq bool
	LazyIEqDelay                                 uint32
}
type SMTArrayParams struct {
	uparams.ModuleParamsOwner
	self   *SMTArrayParams
	state  SMTArrayState
	header uint64
	closed bool
}

func NewSMTArrayParams(b *arena.Budget) (*SMTArrayParams, error) {
	if b == nil {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(SMTArrayParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTArrayParams{header: n}
	e := s.constructSource(b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTArrayParams) constructSource(b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTArrayParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Budget: b}, header: s.header, state: SMTArrayState{false, true, 3, false, true, 1, true, false, true, false, 10}}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	// Source constructor has no updt_params call: no global/local/provider read.
	return nil
}
func (s *SMTArrayParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTArrayParams) State() (SMTArrayState, error) {
	if e := s.open(); e != nil {
		return SMTArrayState{}, e
	}
	return s.state, nil
}
func (s *SMTArrayParams) Update(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (result error) {
	if e := s.open(); e != nil {
		return e
	}
	if e := s.Bind(c, g); e != nil {
		return e
	}
	s.Running = true
	defer func() { s.Running = false }()
	f, e := s.BeginHelper(p, "smt")
	if f != nil {
		defer func() { result = s.FinishHelper(f, result) }()
	}
	if e != nil {
		return e
	}
	weak, e := s.FrameBool(f, "array.weak", false)
	if e != nil {
		return e
	}
	s.state.Weak = weak
	extensional, e := s.FrameBool(f, "array.extensional", true)
	if e != nil {
		return e
	}
	s.state.Extensional = extensional
	return nil
}
func (s *SMTArrayParams) Close() error {
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
func (s *SMTArrayParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_mode="); e != nil {
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
	if e := out.AppendCString("m_array_weak="); e != nil {
		return e
	}
	weak := int64(0)
	if s.state.Weak {
		weak = 1
	}
	if e := out.AppendInt64(weak); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_extensional="); e != nil {
		return e
	}
	extensional := int64(0)
	if s.state.Extensional {
		extensional = 1
	}
	if e := out.AppendInt64(extensional); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_laziness="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.Laziness); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_delay_exp_axiom="); e != nil {
		return e
	}
	delay := int64(0)
	if s.state.DelayExpAxiom {
		delay = 1
	}
	if e := out.AppendInt64(delay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_cg="); e != nil {
		return e
	}
	cg := int64(0)
	if s.state.CG {
		cg = 1
	}
	if e := out.AppendInt64(cg); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_always_prop_upward="); e != nil {
		return e
	}
	upward := int64(0)
	if s.state.AlwaysPropUpward {
		upward = 1
	}
	if e := out.AppendInt64(upward); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_lazy_ieq="); e != nil {
		return e
	}
	lazy := int64(0)
	if s.state.LazyIEq {
		lazy = 1
	}
	if e := out.AppendInt64(lazy); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_array_lazy_ieq_delay="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.LazyIEqDelay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}
