package smtcore

// Source: src/smt/params/qi_params.h:31-109; src/smt/params/qi_params.cpp:23-64;
// src/smt/params/smt_params_helper.pyg:24-37; scripts/mk_genfile_common.py:895-939.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DefaultsInitialized records the scalar initializer frontier after the
// Go string members; State otherwise exposes diagnostic Go storage only.
type SMTQIState struct {
	DefaultsInitialized                                bool
	Ematching, EmatchingInitialized                    bool
	EagerThreshold, LazyThreshold                      float64
	MaxEagerMultipatterns, MaxLazyMultipatternMatching uint32
	Profile                                            bool
	ProfileFrequency, QuickChecker                     uint32
	LazyQuickChecker, PromoteUnsat                     bool
	MaxInstances                                       uint32
	LazyInstantiation, ConservativeFinalCheck, MBQI    bool
	MaxCexs, MaxCexsIncrement, MaxIterations           uint32
	Trace                                              bool
	ForceTemplate                                      uint32
	MBQIId                                             uparams.ParamsStringView
}
type SMTQIParams struct {
	uparams.ModuleParamsOwner
	self                     *SMTQIParams
	state                    SMTQIState
	cost, newGen             string
	header                   uint64
	completed                uint8
	constructorReady, closed bool
	constructorCause         error
}

func NewSMTQIParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTQIParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTQIParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTQIParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTQIParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTQIParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	s.cost = "(+ weight generation)"
	s.completed = 1
	s.newGen = "cost"
	s.completed = 2
	// Actual later scalar initializers run only after both earlier members.
	s.state = SMTQIState{EagerThreshold: 10, LazyThreshold: 20, MaxLazyMultipatternMatching: 2, ProfileFrequency: 4294967295, LazyQuickChecker: true, PromoteUnsat: true, MaxInstances: 4294967295, MBQI: true, MaxCexs: 1, MaxCexsIncrement: 1, MaxIterations: 1000, ForceTemplate: 10}
	s.state.DefaultsInitialized = true
	e := s.update(p)
	if e != nil {
		s.constructorCause = e
		// A younger stopped lexical DT protects older automatic members as well.
		if s.DestructorCause == nil {
			s.newGen = ""
			s.cost = ""
		}
	} else {
		s.constructorReady = true
	}
	return e
}
func (s *SMTQIParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTQIParams) State() (SMTQIState, error) {
	if e := s.open(); e != nil {
		return SMTQIState{}, e
	}
	return s.state, nil
}
func (s *SMTQIParams) CostText() (string, error) {
	if e := s.open(); e != nil {
		return "", e
	}
	if !s.constructorReady {
		return "", uparams.ErrParamsPrecondition
	}
	return s.cost, nil
}
func (s *SMTQIParams) NewGenerationText() (string, error) {
	if e := s.open(); e != nil {
		return "", e
	}
	if !s.constructorReady {
		return "", uparams.ErrParamsPrecondition
	}
	return s.newGen, nil
}
func (s *SMTQIParams) SetEmatching(v bool) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	s.state.Ematching = v
	s.state.EmatchingInitialized = true
	return nil
}
func (s *SMTQIParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(p)
}
func (s *SMTQIParams) update(local *uparams.ParamsRef) (result error) {
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
	b, e := s.FrameBool(p, "mbqi", true)
	if e != nil {
		return e
	}
	s.state.MBQI = b
	u, e := s.FrameUInt(p, "mbqi.max_cexs", 1)
	if e != nil {
		return e
	}
	s.state.MaxCexs = u
	u, e = s.FrameUInt(p, "mbqi.max_cexs_incr", 0)
	if e != nil {
		return e
	}
	s.state.MaxCexsIncrement = u
	u, e = s.FrameUInt(p, "mbqi.max_iterations", 1000)
	if e != nil {
		return e
	}
	s.state.MaxIterations = u
	b, e = s.FrameBool(p, "mbqi.trace", false)
	if e != nil {
		return e
	}
	s.state.Trace = b
	u, e = s.FrameUInt(p, "mbqi.force_template", 10)
	if e != nil {
		return e
	}
	s.state.ForceTemplate = u
	text, e := s.FrameStringView(p, "mbqi.id", "")
	if e != nil {
		return e
	}
	s.state.MBQIId = text
	b, e = s.FrameBool(p, "qi.profile", false)
	if e != nil {
		return e
	}
	s.state.Profile = b
	u, e = s.FrameUInt(p, "qi.profile_freq", 4294967295)
	if e != nil {
		return e
	}
	s.state.ProfileFrequency = u
	u, e = s.FrameUInt(p, "qi.max_instances", 4294967295)
	if e != nil {
		return e
	}
	s.state.MaxInstances = u
	d, e := s.FrameDouble(p, "qi.eager_threshold", 10)
	if e != nil {
		return e
	}
	s.state.EagerThreshold = d
	d, e = s.FrameDouble(p, "qi.lazy_threshold", 20)
	if e != nil {
		return e
	}
	s.state.LazyThreshold = d
	text, e = s.FrameStringView(p, "qi.cost", "(+ weight generation)")
	if e != nil {
		return e
	}
	cost, e := text.String()
	if e != nil {
		return e
	}
	s.cost = sourceCStringPrefix(cost)
	u, e = s.FrameUInt(p, "qi.max_multi_patterns", 0)
	if e != nil {
		return e
	}
	s.state.MaxEagerMultipatterns = u
	return nil
}
func (s *SMTQIParams) Close() error {
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
	s.newGen = ""
	s.cost = ""
	s.Budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}

func (s *SMTQIParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := out.AppendCString("m_qi_ematching="); e != nil {
		return e
	}
	if !s.state.EmatchingInitialized {
		return umemory.ErrRuntimeUninitialized
	}
	bEmatching := int64(0)
	if s.state.Ematching {
		bEmatching = 1
	}
	if e := out.AppendInt64(bEmatching); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_cost="); e != nil {
		return e
	}
	if e := out.AppendString(s.cost); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_new_gen="); e != nil {
		return e
	}
	if e := out.AppendString(s.newGen); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_eager_threshold="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.EagerThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_lazy_threshold="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.LazyThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_max_eager_multipatterns="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxEagerMultipatterns); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_max_lazy_multipattern_matching="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxLazyMultipatternMatching); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_profile="); e != nil {
		return e
	}
	bProfile := int64(0)
	if s.state.Profile {
		bProfile = 1
	}
	if e := out.AppendInt64(bProfile); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_profile_freq="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.ProfileFrequency); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_quick_checker="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.QuickChecker)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_lazy_quick_checker="); e != nil {
		return e
	}
	bLazyQuickChecker := int64(0)
	if s.state.LazyQuickChecker {
		bLazyQuickChecker = 1
	}
	if e := out.AppendInt64(bLazyQuickChecker); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_promote_unsat="); e != nil {
		return e
	}
	bPromoteUnsat := int64(0)
	if s.state.PromoteUnsat {
		bPromoteUnsat = 1
	}
	if e := out.AppendInt64(bPromoteUnsat); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_max_instances="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxInstances); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_lazy_instantiation="); e != nil {
		return e
	}
	bLazyInstantiation := int64(0)
	if s.state.LazyInstantiation {
		bLazyInstantiation = 1
	}
	if e := out.AppendInt64(bLazyInstantiation); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_qi_conservative_final_check="); e != nil {
		return e
	}
	bConservativeFinalCheck := int64(0)
	if s.state.ConservativeFinalCheck {
		bConservativeFinalCheck = 1
	}
	if e := out.AppendInt64(bConservativeFinalCheck); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi="); e != nil {
		return e
	}
	bMBQI := int64(0)
	if s.state.MBQI {
		bMBQI = 1
	}
	if e := out.AppendInt64(bMBQI); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_max_cexs="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxCexs); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_max_cexs_incr="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxCexsIncrement); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_max_iterations="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxIterations); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_trace="); e != nil {
		return e
	}
	bTrace := int64(0)
	if s.state.Trace {
		bTrace = 1
	}
	if e := out.AppendInt64(bTrace); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_force_template="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.ForceTemplate); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_mbqi_id="); e != nil {
		return e
	}
	if e := (s.state.MBQIId).AppendTo(out); e != nil {
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
