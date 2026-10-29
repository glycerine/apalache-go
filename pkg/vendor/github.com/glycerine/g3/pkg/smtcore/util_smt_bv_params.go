package smtcore

// Source: src/smt/params/theory_bv_params.h:24-50;
// src/smt/params/theory_bv_params.cpp:23-40; src/smt/params/smt_params_helper.pyg:38-39;
// src/ast/rewriter/bv_rewriter_params.pyg:1-9; scripts/mk_genfile_common.py:895-939.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type SMTBVState struct {
	Mode                        uint32
	HiDiv0, Reflect, LazyLE, CC bool
	BlastMaxSize                uint32
	EnableInt2BV                bool
}
type SMTBVParams struct {
	uparams.ModuleParamsOwner
	self                     *SMTBVParams
	state                    SMTBVState
	header                   uint64
	closed, constructorReady bool
	constructorCause         error
}

func NewSMTBVParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTBVParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTBVParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTBVParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTBVParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTBVParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header, state: SMTBVState{1, false, true, false, false, 2147483647, true}}
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
func (s *SMTBVParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTBVParams) State() (SMTBVState, error) {
	if e := s.open(); e != nil {
		return SMTBVState{}, e
	}
	return s.state, nil
}
func (s *SMTBVParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(p)
}
func (s *SMTBVParams) update(local *uparams.ParamsRef) (result error) {
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
	rp, e := s.BeginHelper(local, "rewriter")
	if rp != nil {
		defer func() { result = s.FinishHelper(rp, result) }()
	}
	if e != nil {
		return e
	}
	v, e := s.FrameBool(rp, "hi_div0", true)
	if e != nil {
		return e
	}
	s.state.HiDiv0 = v
	v, e = s.FrameBool(p, "bv.reflect", true)
	if e != nil {
		return e
	}
	s.state.Reflect = v
	v, e = s.FrameBool(p, "bv.enable_int2bv", true)
	if e != nil {
		return e
	}
	s.state.EnableInt2BV = v
	return nil
}
func (s *SMTBVParams) Close() error {
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
func (s *SMTBVParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := out.AppendCString("m_bv_mode="); e != nil {
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
	if e := out.AppendCString("m_hi_div0="); e != nil {
		return e
	}
	v := int64(0)
	if s.state.HiDiv0 {
		v = 1
	}
	if e := out.AppendInt64(v); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bv_reflect="); e != nil {
		return e
	}
	v = 0
	if s.state.Reflect {
		v = 1
	}
	if e := out.AppendInt64(v); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bv_lazy_le="); e != nil {
		return e
	}
	v = 0
	if s.state.LazyLE {
		v = 1
	}
	if e := out.AppendInt64(v); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bv_cc="); e != nil {
		return e
	}
	v = 0
	if s.state.CC {
		v = 1
	}
	if e := out.AppendInt64(v); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bv_blast_max_size="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.BlastMaxSize); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bv_enable_int2bv2int="); e != nil {
		return e
	}
	v = 0
	if s.state.EnableInt2BV {
		v = 1
	}
	if e := out.AppendInt64(v); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}
