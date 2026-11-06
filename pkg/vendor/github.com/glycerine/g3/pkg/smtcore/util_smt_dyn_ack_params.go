package smtcore

// Source: src/smt/params/dyn_ack_params.h:24-51; src/smt/params/dyn_ack_params.cpp:22-41;
// src/smt/params/smt_params_helper.pyg:61-66; scripts/mk_genfile_common.py:895-939.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// These scalar members can be embedded once in the future actual smt_params.
// The standalone owner below is an explicit Go lifetime adapter, not native
// dynamic allocation for the source scalar base or its lexical helper.
type SMTDynAckState struct {
	Strategy       uint32
	Equalities     bool
	Factor         float64
	Threshold, GC  uint32
	GCInverseDecay float64
}
type SMTDynAckParams struct {
	uparams.ModuleParamsOwner
	self             *SMTDynAckParams
	state            SMTDynAckState
	header           uint64
	closed           bool
	constructorCause error
	constructorReady bool
}

func NewSMTDynAckParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTDynAckParams, error) {
	if c == nil || g == nil || g.CheckOwner(c, b) != nil || b == nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTDynAckParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTDynAckParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTDynAckParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTDynAckParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header, state: SMTDynAckState{1, false, 0.1, 10, 2000, 0.8}}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	e := s.update(p)
	if e != nil {
		s.constructorCause = e
	} else {
		s.constructorReady = true
	}
	// Failed source construction retains any actual helper acquisition. Owning
	// caller Close is separate, even if a lexical source destructor stopped.
	return e
}
func (s *SMTDynAckParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTDynAckParams) State() (SMTDynAckState, error) {
	if e := s.open(); e != nil {
		return SMTDynAckState{}, e
	}
	return s.state, nil
}
func (s *SMTDynAckParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(p)
}
func (s *SMTDynAckParams) update(p *uparams.ParamsRef) (result error) {
	if e := s.open(); e != nil {
		return e
	}
	// Retained stopped helper frames cannot be replaced by an unrelated call.
	// This Go owning protocol guard has no source successful update credit.
	if s.Helpers != nil {
		return uparams.ErrParamsPrecondition
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
	key := "dack"
	u, e := s.FrameUInt(f, key, 1)
	if e != nil {
		return e
	}
	s.state.Strategy = u
	key = "dack.eq"
	v, e := s.FrameBool(f, key, false)
	if e != nil {
		return e
	}
	s.state.Equalities = v
	key = "dack.factor"
	d, e := s.FrameDouble(f, key, 0.1)
	if e != nil {
		return e
	}
	s.state.Factor = d
	key = "dack.threshold"
	u, e = s.FrameUInt(f, key, 10)
	if e != nil {
		return e
	}
	s.state.Threshold = u
	key = "dack.gc"
	u, e = s.FrameUInt(f, key, 2000)
	if e != nil {
		return e
	}
	s.state.GC = u
	key = "dack.gc_inv_decay"
	d, e = s.FrameDouble(f, key, 0.8)
	if e != nil {
		return e
	}
	s.state.GCInverseDecay = d
	return nil
}
func (s *SMTDynAckParams) Close() error {
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

// Display preserves each source insertion and endl flush, using the selected
// actual ostream profile. Arbitrary native locale/boolalpha remain explicit
// stream-profile obligations rather than a preformatted whole-message path.
func (s *SMTDynAckParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := out.AppendCString("m_dack="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.Strategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dack_eq="); e != nil {
		return e
	}
	b := int64(0)
	if s.state.Equalities {
		b = 1
	}
	if e := out.AppendInt64(b); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dack_factor="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.Factor); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dack_threshold="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.Threshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dack_gc="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.GC); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dack_gc_inv_decay="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.GCInverseDecay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}
