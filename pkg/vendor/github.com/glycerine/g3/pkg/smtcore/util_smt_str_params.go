package smtcore

// Source: src/smt/params/theory_str_params.h:25-143;
// src/smt/params/theory_str_params.cpp:21-40; src/smt/params/smt_params_helper.pyg:70-87.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type SMTStrState struct {
	StrongArrangements                           bool
	AggressiveLengthTesting                      bool
	AggressiveValueTesting                       bool
	AggressiveUnrollTesting                      bool
	UseFastLengthTesterCache                     bool
	UseFastValueTesterCache                      bool
	StringConstantCache                          bool
	FiniteOverlapModels                          bool
	UseBinarySearch                              bool
	BinarySearchInitialUpperBound                uint32
	OverlapTheoryAwarePriority                   float64
	RegexAutomata                                bool
	RegexAutomataDifficultyThreshold             uint32
	RegexAutomataIntersectionDifficultyThreshold uint32
	RegexAutomataFailedAutomatonThreshold        uint32
	RegexAutomataFailedIntersectionThreshold     uint32
	RegexAutomataLengthAttemptThreshold          uint32
}
type SMTStrParams struct {
	uparams.ModuleParamsOwner
	self                     *SMTStrParams
	state                    SMTStrState
	header                   uint64
	closed, constructorReady bool
	constructorCause         error
}

func NewSMTStrParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTStrParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTStrParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTStrParams{header: n}
	e := s.constructSource(c, p, g, b)
	return s, e
}

// Same source constructor engine: an actual containing header charges embedded fields once.
func (s *SMTStrParams) constructSource(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) error {
	if s == nil || s.self != nil {
		return uparams.ErrParamsOwner
	}
	*s = SMTStrParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: s.header, state: SMTStrState{true, false, false, true, false, true, true, false, false, 64, -0.1, true, 1000, 1000, 10, 10, 10}}
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
func (s *SMTStrParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTStrParams) State() (SMTStrState, error) {
	if e := s.open(); e != nil {
		return SMTStrState{}, e
	}
	return s.state, nil
}
func (s *SMTStrParams) Update(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	return s.update(p)
}
func (s *SMTStrParams) update(local *uparams.ParamsRef) (result error) {
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
	vStrongArrangements, e := s.FrameBool(p, "str.strong_arrangements", true)
	if e != nil {
		return e
	}
	s.state.StrongArrangements = vStrongArrangements
	vAggressiveLengthTesting, e := s.FrameBool(p, "str.aggressive_length_testing", false)
	if e != nil {
		return e
	}
	s.state.AggressiveLengthTesting = vAggressiveLengthTesting
	vAggressiveValueTesting, e := s.FrameBool(p, "str.aggressive_value_testing", false)
	if e != nil {
		return e
	}
	s.state.AggressiveValueTesting = vAggressiveValueTesting
	vAggressiveUnrollTesting, e := s.FrameBool(p, "str.aggressive_unroll_testing", true)
	if e != nil {
		return e
	}
	s.state.AggressiveUnrollTesting = vAggressiveUnrollTesting
	vUseFastLengthTesterCache, e := s.FrameBool(p, "str.fast_length_tester_cache", false)
	if e != nil {
		return e
	}
	s.state.UseFastLengthTesterCache = vUseFastLengthTesterCache
	vUseFastValueTesterCache, e := s.FrameBool(p, "str.fast_value_tester_cache", true)
	if e != nil {
		return e
	}
	s.state.UseFastValueTesterCache = vUseFastValueTesterCache
	vStringConstantCache, e := s.FrameBool(p, "str.string_constant_cache", true)
	if e != nil {
		return e
	}
	s.state.StringConstantCache = vStringConstantCache
	vFiniteOverlapModels, e := s.FrameBool(p, "str.finite_overlap_models", false)
	if e != nil {
		return e
	}
	s.state.FiniteOverlapModels = vFiniteOverlapModels
	vUseBinarySearch, e := s.FrameBool(p, "str.use_binary_search", false)
	if e != nil {
		return e
	}
	s.state.UseBinarySearch = vUseBinarySearch
	vBinarySearchInitialUpperBound, e := s.FrameUInt(p, "str.binary_search_start", 64)
	if e != nil {
		return e
	}
	s.state.BinarySearchInitialUpperBound = vBinarySearchInitialUpperBound
	vOverlapTheoryAwarePriority, e := s.FrameDouble(p, "str.overlap_priority", -0.1)
	if e != nil {
		return e
	}
	s.state.OverlapTheoryAwarePriority = vOverlapTheoryAwarePriority
	vRegexAutomata, e := s.FrameBool(p, "str.regex_automata", true)
	if e != nil {
		return e
	}
	s.state.RegexAutomata = vRegexAutomata
	vRegexAutomataDifficultyThreshold, e := s.FrameUInt(p, "str.regex_automata_difficulty_threshold", 1000)
	if e != nil {
		return e
	}
	s.state.RegexAutomataDifficultyThreshold = vRegexAutomataDifficultyThreshold
	vRegexAutomataIntersectionDifficultyThreshold, e := s.FrameUInt(p, "str.regex_automata_intersection_difficulty_threshold", 1000)
	if e != nil {
		return e
	}
	s.state.RegexAutomataIntersectionDifficultyThreshold = vRegexAutomataIntersectionDifficultyThreshold
	vRegexAutomataFailedAutomatonThreshold, e := s.FrameUInt(p, "str.regex_automata_failed_automaton_threshold", 10)
	if e != nil {
		return e
	}
	s.state.RegexAutomataFailedAutomatonThreshold = vRegexAutomataFailedAutomatonThreshold
	vRegexAutomataFailedIntersectionThreshold, e := s.FrameUInt(p, "str.regex_automata_failed_intersection_threshold", 10)
	if e != nil {
		return e
	}
	s.state.RegexAutomataFailedIntersectionThreshold = vRegexAutomataFailedIntersectionThreshold
	vRegexAutomataLengthAttemptThreshold, e := s.FrameUInt(p, "str.regex_automata_length_attempt_threshold", 10)
	if e != nil {
		return e
	}
	s.state.RegexAutomataLengthAttemptThreshold = vRegexAutomataLengthAttemptThreshold
	return nil
}
func (s *SMTStrParams) Close() error {
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
