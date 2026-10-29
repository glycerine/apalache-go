package smtcore

// Source: src/smt/params/smt_params.h:75-318; src/smt/params/smt_params.cpp:24-164;
// src/model/model_params.pyg:1-8; src/smt/params/smt_params_helper.pyg:5-23,67-69.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type SMTParamsState struct {
	DisplayProof               bool
	DisplayDotProof            bool
	DisplayUnsatCore           bool
	CheckProof                 bool
	EqPropagation              bool
	BinaryClauseOpt            bool
	RelevancyLvl               uint32
	RelevancyLemma             bool
	RandomSeed                 uint32
	RandomVarFreq              float64
	InvDecay                   float64
	ClauseDecay                uint32
	RandomInitialActivity      uint32
	PhaseSelection             uint32
	PhaseCachingOn             uint32
	PhaseCachingOff            uint32
	MinimizeLemmas             bool
	MaxConflicts               uint32
	SimplifyClauses            bool
	Tick                       uint32
	DisplayFeatures            bool
	NewCore2thEq               bool
	Ematching                  bool
	CaseSplitStrategy          uint32
	RelCaseSplitOrder          uint32
	LookaheadDiseq             bool
	TheoryCaseSplit            bool
	TheoryAwareBranching       bool
	DelayUnits                 bool
	DelayUnitsThreshold        uint32
	TheoryResolve              bool
	RestartStrategy            uint32
	RestartInitial             uint32
	RestartFactor              float64
	RestartAdaptive            bool
	AgilityFactor              float64
	RestartAgilityThreshold    float64
	LemmaGcStrategy            uint32
	LemmaGcHalf                bool
	RecentLemmasSize           uint32
	LemmaGcInitial             uint32
	LemmaGcFactor              float64
	NewOldRatio                uint32
	NewClauseActivity          uint32
	OldClauseActivity          uint32
	NewClauseRelevancy         uint32
	OldClauseRelevancy         uint32
	InvClauseDecay             float64
	SmtlibDumpLemmas           bool
	Logic                      usymbol.Symbol
	ProfileResSub              bool
	DisplayBoolVar2expr        bool
	DisplayLlBoolVar2expr      bool
	AbortAfterPreproc          bool
	Model                      bool
	ModelCompact               bool
	ModelOnTimeout             bool
	ModelOnFinalCheck          bool
	ProgressSamplingFreq       uint32
	DisplayInstalledTheories   bool
	CoreValidate               bool
	Preprocess                 bool
	UserTheoryPreprocessAxioms bool
	UserTheoryPersistAxioms    bool
	Timeout                    uint32
	Rlimit                     uint32
	AtLabelsCex                bool
	CheckAtLabels              bool
	DumpGoalAsSmt              bool
	AutoConfig                 bool
	DumpBenchmarks             bool
	DumpMinTime                float64
	DumpRecheck                bool
	StringSolver               usymbol.Symbol
	// DefaultsInitialized records the 70 initializers preceding StringSolver.
	// These diagnostic flags do not imply completed constructor/body lifetime.
	DefaultsInitialized, StringSolverInitialized, DumpInitialized bool
}
type SMTParams struct {
	uparams.ModuleParamsOwner
	self                                 *SMTParams
	preprocess                           SMTPreprocessorParams
	dyn                                  SMTDynAckParams
	qi                                   SMTQIParams
	arith                                SMTArithParams
	array                                SMTArrayParams
	bv                                   SMTBVParams
	str                                  SMTStrParams
	pb                                   SMTPBParams
	datatype                             SMTDatatypeParams
	state                                SMTParamsState
	empty                                uparams.ParamsRef
	header                               uint64
	completedBases, endedBases           uint16
	closed, constructorReady, activeCall bool
	constructorCause, copyCause          error
}

func NewSMTParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService, b *arena.Budget) (*SMTParams, error) {
	if c == nil || g == nil || b == nil || g.CheckOwner(c, b) != nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := c.Check(p); e != nil {
		return nil, e
	}
	if e := g.CheckOpen(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SMTParams{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &SMTParams{ModuleParamsOwner: uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: b}, header: n, empty: c.EmptyValue()}
	s.self = s
	s.ModuleParamsOwner.Self = &s.ModuleParamsOwner
	s.activeCall = true
	e := s.construct(p)
	s.activeCall = false
	if e != nil {
		s.constructorCause = e
		if s.sourceStop() == nil {
			s.endBasesSource()
		}
	} else {
		s.constructorReady = true
	}
	return s, e
}
func (s *SMTParams) construct(p *uparams.ParamsRef) error {
	if e := s.preprocess.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 0
	if e := s.dyn.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 1
	if e := s.qi.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 2
	if e := s.arith.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 3
	if e := s.array.constructSource(s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 4
	if e := s.bv.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 5
	if e := s.str.constructSource(s.Context, &s.empty, s.Globals, s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 6
	if e := s.pb.constructSource(s.Budget); e != nil {
		return e
	}
	s.completedBases |= 1 << 7
	s.datatype = NewSMTDatatypeParams()
	s.completedBases |= 1 << 8
	s.state.DisplayProof = false
	s.state.DisplayDotProof = false
	s.state.DisplayUnsatCore = false
	s.state.CheckProof = false
	s.state.EqPropagation = true
	s.state.BinaryClauseOpt = true
	s.state.RelevancyLvl = 2
	s.state.RelevancyLemma = false
	s.state.RandomSeed = 0
	s.state.RandomVarFreq = 0.01
	s.state.InvDecay = 1.052
	s.state.ClauseDecay = 1
	s.state.RandomInitialActivity = 1
	s.state.PhaseSelection = 3
	s.state.PhaseCachingOn = 400
	s.state.PhaseCachingOff = 100
	s.state.MinimizeLemmas = true
	s.state.MaxConflicts = ^uint32(0)
	s.state.SimplifyClauses = true
	s.state.Tick = 1000
	s.state.DisplayFeatures = false
	s.state.NewCore2thEq = true
	s.state.Ematching = true
	s.state.CaseSplitStrategy = 1
	s.state.RelCaseSplitOrder = 0
	s.state.LookaheadDiseq = false
	s.state.TheoryCaseSplit = false
	s.state.TheoryAwareBranching = false
	s.state.DelayUnits = false
	s.state.DelayUnitsThreshold = 32
	s.state.TheoryResolve = false
	s.state.RestartStrategy = 1
	s.state.RestartInitial = 100
	s.state.RestartFactor = 1.1
	s.state.RestartAdaptive = true
	s.state.AgilityFactor = 0.9999
	s.state.RestartAgilityThreshold = 0.18
	s.state.LemmaGcStrategy = 0
	s.state.LemmaGcHalf = false
	s.state.RecentLemmasSize = 100
	s.state.LemmaGcInitial = 5000
	s.state.LemmaGcFactor = 1.1
	s.state.NewOldRatio = 16
	s.state.NewClauseActivity = 10
	s.state.OldClauseActivity = 500
	s.state.NewClauseRelevancy = 45
	s.state.OldClauseRelevancy = 6
	s.state.InvClauseDecay = 1
	s.state.SmtlibDumpLemmas = false
	s.state.Logic = usymbol.Symbol{}
	s.state.ProfileResSub = false
	s.state.DisplayBoolVar2expr = false
	s.state.DisplayLlBoolVar2expr = false
	s.state.AbortAfterPreproc = false
	s.state.Model = true
	s.state.ModelCompact = false
	s.state.ModelOnTimeout = false
	s.state.ModelOnFinalCheck = false
	s.state.ProgressSamplingFreq = 0
	s.state.DisplayInstalledTheories = false
	s.state.CoreValidate = false
	s.state.Preprocess = true
	s.state.UserTheoryPreprocessAxioms = false
	s.state.UserTheoryPersistAxioms = false
	s.state.Timeout = 0
	s.state.Rlimit = 0
	s.state.AtLabelsCex = false
	s.state.CheckAtLabels = false
	s.state.DumpGoalAsSmt = false
	s.state.AutoConfig = true
	s.state.DefaultsInitialized = true
	symbol, e := s.Context.Symbols().Intern("auto")
	if e != nil {
		return e
	}
	s.state.StringSolver = symbol
	s.state.StringSolverInitialized = true
	return s.updateLocal(p)
}
func (s *SMTParams) open() error {
	if s == nil || s.self != s || s.closed {
		return uparams.ErrParamsOwner
	}
	if s.activeCall || s.Running {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTParams) State() (SMTParamsState, error) {
	if e := s.open(); e != nil {
		return SMTParamsState{}, e
	}
	return s.state, nil
}

// DynamicAckState exposes the embedded dyn_ack_params base used by the SMT
// context. The returned value is a scalar snapshot and does not borrow the
// parameter owner.
func (s *SMTParams) DynamicAckState() (SMTDynAckState, error) {
	if e := s.open(); e != nil {
		return SMTDynAckState{}, e
	}
	return s.dyn.State()
}

// QuantifierState exposes the embedded qi_params snapshot consumed by the
// retained quantifier manager.
func (s *SMTParams) QuantifierState() (SMTQIState, error) {
	if e := s.open(); e != nil {
		return SMTQIState{}, e
	}
	return s.qi.State()
}

func (s *SMTParams) QuantifierCostText() (string, error) {
	if e := s.open(); e != nil {
		return "", e
	}
	return s.qi.CostText()
}

func (s *SMTParams) QuantifierNewGenerationText() (string, error) {
	if e := s.open(); e != nil {
		return "", e
	}
	return s.qi.NewGenerationText()
}

// ArithmeticState exposes the embedded theory_arith_params snapshot consumed
// by retained arithmetic theory implementations.
func (s *SMTParams) ArithmeticState() (SMTArithState, error) {
	if e := s.open(); e != nil {
		return SMTArithState{}, e
	}
	return s.arith.State()
}

// BitVectorState exposes the embedded theory_bv_params snapshot consumed by
// the retained bit-vector theory.
func (s *SMTParams) BitVectorState() (SMTBVState, error) {
	if e := s.open(); e != nil {
		return SMTBVState{}, e
	}
	return s.bv.State()
}
func (s *SMTParams) sourceStop() error {
	if s.DestructorCause != nil {
		return s.DestructorCause
	}
	if s.preprocess.DestructorCause != nil {
		return s.preprocess.DestructorCause
	}
	if s.dyn.DestructorCause != nil {
		return s.dyn.DestructorCause
	}
	if s.qi.DestructorCause != nil {
		return s.qi.DestructorCause
	}
	if s.arith.DestructorCause != nil {
		return s.arith.DestructorCause
	}
	if s.array.DestructorCause != nil {
		return s.array.DestructorCause
	}
	if s.bv.DestructorCause != nil {
		return s.bv.DestructorCause
	}
	if s.str.DestructorCause != nil {
		return s.str.DestructorCause
	}
	if s.pb.DestructorCause != nil {
		return s.pb.DestructorCause
	}
	if s.preprocess.pattern.DestructorCause != nil {
		return s.preprocess.pattern.DestructorCause
	}
	return nil
}

// Actual completed trivial bases end in reverse declaration order. No owning
// cleanup of incomplete constructor roots is hidden inside source unwind.
func (s *SMTParams) endBasesSource() {
	if s.completedBases&(1<<8) != 0 && s.endedBases&(1<<8) == 0 {
		s.endedBases |= 1 << 8
	}
	if s.completedBases&(1<<7) != 0 && s.endedBases&(1<<7) == 0 {
		s.pb.closed = true
		s.endedBases |= 1 << 7
	}
	if s.completedBases&(1<<6) != 0 && s.endedBases&(1<<6) == 0 {
		s.str.closed = true
		s.endedBases |= 1 << 6
	}
	if s.completedBases&(1<<5) != 0 && s.endedBases&(1<<5) == 0 {
		s.bv.closed = true
		s.endedBases |= 1 << 5
	}
	if s.completedBases&(1<<4) != 0 && s.endedBases&(1<<4) == 0 {
		s.array.closed = true
		s.endedBases |= 1 << 4
	}
	if s.completedBases&(1<<3) != 0 && s.endedBases&(1<<3) == 0 {
		s.arith.closed = true
		s.endedBases |= 1 << 3
	}
	if s.completedBases&(1<<2) != 0 && s.endedBases&(1<<2) == 0 {
		s.qi.newGen = ""
		s.qi.cost = ""
		s.qi.closed = true
		s.endedBases |= 1 << 2
	}
	if s.completedBases&(1<<1) != 0 && s.endedBases&(1<<1) == 0 {
		s.dyn.closed = true
		s.endedBases |= 1 << 1
	}
	if s.completedBases&(1<<0) != 0 && s.endedBases&(1<<0) == 0 {
		s.preprocess.sourceUnwindBases()
		s.preprocess.closed = true
		s.endedBases |= 1 << 0
	}
}
func (s *SMTParams) admitMutation() error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady || s.sourceStop() != nil || s.Helpers != nil {
		return uparams.ErrParamsPrecondition
	}
	return nil
}
func (s *SMTParams) UpdateLocal(p *uparams.ParamsRef) error {
	if e := s.admitMutation(); e != nil {
		return e
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	return s.updateLocal(p)
}
func (s *SMTParams) Update(p *uparams.ParamsRef) error {
	if e := s.admitMutation(); e != nil {
		return e
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	if e := s.preprocess.Update(p); e != nil {
		return e
	}
	if e := s.dyn.Update(p); e != nil {
		return e
	}
	if e := s.qi.Update(p); e != nil {
		return e
	}
	if e := s.arith.Update(p); e != nil {
		return e
	}
	if e := s.bv.Update(p); e != nil {
		return e
	}
	if e := s.pb.Update(s.Context, p, s.Globals); e != nil {
		return e
	}
	return s.updateLocal(p)
}
func (s *SMTParams) UpdateContext(p *ContextParams) error {
	if e := s.admitMutation(); e != nil {
		return e
	}
	if e := p.open(); e != nil {
		return e
	}
	if !p.ready {
		return uparams.ErrParamsPrecondition
	}
	s.state.AutoConfig = p.state.AutoConfig
	s.state.Model = p.state.Model
	return nil
}
func (s *SMTParams) updateLocal(local *uparams.ParamsRef) (result error) {
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
	auto, e := s.FrameBool(p, "auto_config", true)
	if e != nil {
		return e
	}
	if auto {
		value, e := s.Globals.SourceGetValueName("auto_config")
		if e != nil {
			return e
		}
		auto = value == "true"
	}
	s.state.AutoConfig = auto
	vRandomSeed, e := s.FrameUInt(p, "random_seed", 0)
	if e != nil {
		return e
	}
	s.state.RandomSeed = vRandomSeed
	vRelevancyLvl, e := s.FrameUInt(p, "relevancy", 2)
	if e != nil {
		return e
	}
	s.state.RelevancyLvl = vRelevancyLvl
	vEmatching, e := s.FrameBool(p, "ematching", true)
	if e != nil {
		return e
	}
	s.state.Ematching = vEmatching
	vPhaseSelection, e := s.FrameUInt(p, "phase_selection", 3)
	if e != nil {
		return e
	}
	s.state.PhaseSelection = vPhaseSelection
	vRestartStrategy, e := s.FrameUInt(p, "restart_strategy", 1)
	if e != nil {
		return e
	}
	s.state.RestartStrategy = vRestartStrategy
	vRestartFactor, e := s.FrameDouble(p, "restart_factor", 1.1)
	if e != nil {
		return e
	}
	s.state.RestartFactor = vRestartFactor
	vCaseSplitStrategy, e := s.FrameUInt(p, "case_split", 1)
	if e != nil {
		return e
	}
	s.state.CaseSplitStrategy = vCaseSplitStrategy
	vTheoryCaseSplit, e := s.FrameBool(p, "theory_case_split", false)
	if e != nil {
		return e
	}
	s.state.TheoryCaseSplit = vTheoryCaseSplit
	vTheoryAwareBranching, e := s.FrameBool(p, "theory_aware_branching", false)
	if e != nil {
		return e
	}
	s.state.TheoryAwareBranching = vTheoryAwareBranching
	vDelayUnits, e := s.FrameBool(p, "delay_units", false)
	if e != nil {
		return e
	}
	s.state.DelayUnits = vDelayUnits
	vDelayUnitsThreshold, e := s.FrameUInt(p, "delay_units_threshold", 32)
	if e != nil {
		return e
	}
	s.state.DelayUnitsThreshold = vDelayUnitsThreshold
	hidden := "preprocess"
	pre, e := s.Context.GetBoolName(local, &hidden, true)
	if e != nil {
		return e
	}
	s.state.Preprocess = pre
	vTimeout, e := s.FrameUInt(p, "timeout", ^uint32(0))
	if e != nil {
		return e
	}
	s.state.Timeout = vTimeout
	vRlimit, e := s.FrameUInt(p, "rlimit", 0)
	if e != nil {
		return e
	}
	s.state.Rlimit = vRlimit
	vMaxConflicts, e := s.FrameUInt(p, "max_conflicts", ^uint32(0))
	if e != nil {
		return e
	}
	s.state.MaxConflicts = vMaxConflicts
	vCoreValidate, e := s.FrameBool(p, "core.validate", false)
	if e != nil {
		return e
	}
	s.state.CoreValidate = vCoreValidate
	logicKey := "logic"
	logic, e := s.Context.GetSymbolName(local, &logicKey, s.state.Logic)
	if e != nil {
		return e
	}
	s.state.Logic = logic
	solver, e := s.FrameSymbol(p, "string_solver", "seq")
	if e != nil {
		return e
	}
	s.state.StringSolver = solver
	mp, e := s.BeginHelper(local, "model")
	if mp != nil {
		defer func() { result = s.FinishHelper(mp, result) }()
	}
	if e != nil {
		return e
	}
	compact, e := s.FrameBool(mp, "compact", false)
	if e != nil {
		return e
	}
	s.state.ModelCompact = compact
	greatestKey := "arith.greatest_error_pivot"
	greatest, e := s.Context.GetBoolName(local, &greatestKey, false)
	if e != nil {
		return e
	}
	if greatest {
		s.arith.state.PivotStrategy = 1
	} else {
		leastKey := "arith.least_error_pivot"
		least, e := s.Context.GetBoolName(local, &leastKey, false)
		if e != nil {
			return e
		}
		if least {
			s.arith.state.PivotStrategy = 2
		}
	}
	if e = s.array.Update(s.Context, local, s.Globals); e != nil {
		// Younger nested helper's stopped DT protects both still-live older frames.
		if s.array.DestructorCause != nil && s.DestructorCause == nil {
			s.DestructorCause = s.array.DestructorCause
		}
		return e
	}
	s.state.DumpBenchmarks = false
	s.state.DumpMinTime = 0.5
	s.state.DumpRecheck = false
	s.state.DumpInitialized = true
	return nil
}
func (s *SMTParams) SourceFinalize() error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	if e := s.sourceStop(); e != nil {
		return e
	}
	s.endBasesSource()
	s.closed = true
	return nil
}
func (s *SMTParams) Close() error {
	if s == nil || s.self != s {
		return uparams.ErrParamsOwner
	}
	if s.header == 0 {
		return nil
	}
	if s.activeCall || s.Running {
		return uparams.ErrParamsPrecondition
	}
	// First drain actual youngest lexical owners. Source causes remain sticky.
	if s.array.self != nil {
		if e := s.array.CloseHelpers(); e != nil {
			return e
		}
	}
	if e := s.CloseHelpers(); e != nil {
		return e
	}
	if s.pb.self != nil {
		if e := s.pb.Close(); e != nil {
			return e
		}
	}
	if s.str.self != nil {
		if e := s.str.Close(); e != nil {
			return e
		}
	}
	if s.bv.self != nil {
		if e := s.bv.Close(); e != nil {
			return e
		}
	}
	if s.array.self != nil {
		if e := s.array.Close(); e != nil {
			return e
		}
	}
	if s.arith.self != nil {
		if e := s.arith.Close(); e != nil {
			return e
		}
	}
	if s.qi.self != nil {
		if e := s.qi.Close(); e != nil {
			return e
		}
	}
	if s.dyn.self != nil {
		if e := s.dyn.Close(); e != nil {
			return e
		}
	}
	if s.preprocess.self != nil {
		if e := s.preprocess.Close(); e != nil {
			return e
		}
	}
	s.Budget.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *SMTParams) Display(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.constructorReady {
		return uparams.ErrParamsPrecondition
	}
	s.activeCall = true
	defer func() { s.activeCall = false }()
	if e := s.preprocess.Display(out); e != nil {
		return e
	}
	if e := s.dyn.Display(out); e != nil {
		return e
	}
	if e := s.qi.Display(out); e != nil {
		return e
	}
	if e := s.arith.Display(out); e != nil {
		return e
	}
	if e := s.array.Display(out); e != nil {
		return e
	}
	if e := s.bv.Display(out); e != nil {
		return e
	}
	if e := s.pb.Display(out); e != nil {
		return e
	}
	if e := s.datatype.Display(out); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_proof="); e != nil {
		return e
	}
	var vDisplayProof int32
	if s.state.DisplayProof {
		vDisplayProof = 1
	}
	if e := out.AppendInt32(vDisplayProof); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_dot_proof="); e != nil {
		return e
	}
	var vDisplayDotProof int32
	if s.state.DisplayDotProof {
		vDisplayDotProof = 1
	}
	if e := out.AppendInt32(vDisplayDotProof); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_unsat_core="); e != nil {
		return e
	}
	var vDisplayUnsatCore int32
	if s.state.DisplayUnsatCore {
		vDisplayUnsatCore = 1
	}
	if e := out.AppendInt32(vDisplayUnsatCore); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_check_proof="); e != nil {
		return e
	}
	var vCheckProof int32
	if s.state.CheckProof {
		vCheckProof = 1
	}
	if e := out.AppendInt32(vCheckProof); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_eq_propagation="); e != nil {
		return e
	}
	var vEqPropagation int32
	if s.state.EqPropagation {
		vEqPropagation = 1
	}
	if e := out.AppendInt32(vEqPropagation); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_binary_clause_opt="); e != nil {
		return e
	}
	var vBinaryClauseOpt int32
	if s.state.BinaryClauseOpt {
		vBinaryClauseOpt = 1
	}
	if e := out.AppendInt32(vBinaryClauseOpt); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_relevancy_lvl="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.RelevancyLvl); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_relevancy_lemma="); e != nil {
		return e
	}
	var vRelevancyLemma int32
	if s.state.RelevancyLemma {
		vRelevancyLemma = 1
	}
	if e := out.AppendInt32(vRelevancyLemma); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_random_seed="); e != nil {
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
	if e := out.AppendCString("m_random_var_freq="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.RandomVarFreq); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_inv_decay="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.InvDecay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_clause_decay="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.ClauseDecay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_random_initial_activity="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.RandomInitialActivity)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_phase_selection="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.PhaseSelection)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_phase_caching_on="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.PhaseCachingOn); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_phase_caching_off="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.PhaseCachingOff); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_minimize_lemmas="); e != nil {
		return e
	}
	var vMinimizeLemmas int32
	if s.state.MinimizeLemmas {
		vMinimizeLemmas = 1
	}
	if e := out.AppendInt32(vMinimizeLemmas); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_max_conflicts="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.MaxConflicts); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_simplify_clauses="); e != nil {
		return e
	}
	var vSimplifyClauses int32
	if s.state.SimplifyClauses {
		vSimplifyClauses = 1
	}
	if e := out.AppendInt32(vSimplifyClauses); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_tick="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.Tick); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_features="); e != nil {
		return e
	}
	var vDisplayFeatures int32
	if s.state.DisplayFeatures {
		vDisplayFeatures = 1
	}
	if e := out.AppendInt32(vDisplayFeatures); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_new_core2th_eq="); e != nil {
		return e
	}
	var vNewCore2thEq int32
	if s.state.NewCore2thEq {
		vNewCore2thEq = 1
	}
	if e := out.AppendInt32(vNewCore2thEq); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_ematching="); e != nil {
		return e
	}
	var vEmatching int32
	if s.state.Ematching {
		vEmatching = 1
	}
	if e := out.AppendInt32(vEmatching); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_case_split_strategy="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.CaseSplitStrategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_rel_case_split_order="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.RelCaseSplitOrder); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_lookahead_diseq="); e != nil {
		return e
	}
	var vLookaheadDiseq int32
	if s.state.LookaheadDiseq {
		vLookaheadDiseq = 1
	}
	if e := out.AppendInt32(vLookaheadDiseq); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_delay_units="); e != nil {
		return e
	}
	var vDelayUnits int32
	if s.state.DelayUnits {
		vDelayUnits = 1
	}
	if e := out.AppendInt32(vDelayUnits); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_delay_units_threshold="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.DelayUnitsThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_theory_resolve="); e != nil {
		return e
	}
	var vTheoryResolve int32
	if s.state.TheoryResolve {
		vTheoryResolve = 1
	}
	if e := out.AppendInt32(vTheoryResolve); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restart_strategy="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.RestartStrategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restart_initial="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.RestartInitial); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restart_factor="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.RestartFactor); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restart_adaptive="); e != nil {
		return e
	}
	var vRestartAdaptive int32
	if s.state.RestartAdaptive {
		vRestartAdaptive = 1
	}
	if e := out.AppendInt32(vRestartAdaptive); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_agility_factor="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.AgilityFactor); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_restart_agility_threshold="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.RestartAgilityThreshold); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_lemma_gc_strategy="); e != nil {
		return e
	}
	if e := out.AppendInt32(int32(s.state.LemmaGcStrategy)); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_lemma_gc_half="); e != nil {
		return e
	}
	var vLemmaGcHalf int32
	if s.state.LemmaGcHalf {
		vLemmaGcHalf = 1
	}
	if e := out.AppendInt32(vLemmaGcHalf); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_recent_lemmas_size="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.RecentLemmasSize); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_lemma_gc_initial="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.LemmaGcInitial); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_lemma_gc_factor="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.LemmaGcFactor); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_new_old_ratio="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NewOldRatio); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_new_clause_activity="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NewClauseActivity); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_old_clause_activity="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.OldClauseActivity); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_new_clause_relevancy="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.NewClauseRelevancy); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_old_clause_relevancy="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.OldClauseRelevancy); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_inv_clause_decay="); e != nil {
		return e
	}
	if e := out.AppendFloat64(s.state.InvClauseDecay); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_smtlib_dump_lemmas="); e != nil {
		return e
	}
	var vSmtlibDumpLemmas int32
	if s.state.SmtlibDumpLemmas {
		vSmtlibDumpLemmas = 1
	}
	if e := out.AppendInt32(vSmtlibDumpLemmas); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_logic="); e != nil {
		return e
	}
	if e := s.Globals.AppendSymbolTo(out, s.state.Logic); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_profile_res_sub="); e != nil {
		return e
	}
	var vProfileResSub int32
	if s.state.ProfileResSub {
		vProfileResSub = 1
	}
	if e := out.AppendInt32(vProfileResSub); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_bool_var2expr="); e != nil {
		return e
	}
	var vDisplayBoolVar2expr int32
	if s.state.DisplayBoolVar2expr {
		vDisplayBoolVar2expr = 1
	}
	if e := out.AppendInt32(vDisplayBoolVar2expr); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_ll_bool_var2expr="); e != nil {
		return e
	}
	var vDisplayLlBoolVar2expr int32
	if s.state.DisplayLlBoolVar2expr {
		vDisplayLlBoolVar2expr = 1
	}
	if e := out.AppendInt32(vDisplayLlBoolVar2expr); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_abort_after_preproc="); e != nil {
		return e
	}
	var vAbortAfterPreproc int32
	if s.state.AbortAfterPreproc {
		vAbortAfterPreproc = 1
	}
	if e := out.AppendInt32(vAbortAfterPreproc); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_model="); e != nil {
		return e
	}
	var vModel int32
	if s.state.Model {
		vModel = 1
	}
	if e := out.AppendInt32(vModel); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_model_compact="); e != nil {
		return e
	}
	var vModelCompact int32
	if s.state.ModelCompact {
		vModelCompact = 1
	}
	if e := out.AppendInt32(vModelCompact); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_model_on_timeout="); e != nil {
		return e
	}
	var vModelOnTimeout int32
	if s.state.ModelOnTimeout {
		vModelOnTimeout = 1
	}
	if e := out.AppendInt32(vModelOnTimeout); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_model_on_final_check="); e != nil {
		return e
	}
	var vModelOnFinalCheck int32
	if s.state.ModelOnFinalCheck {
		vModelOnFinalCheck = 1
	}
	if e := out.AppendInt32(vModelOnFinalCheck); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_progress_sampling_freq="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.ProgressSamplingFreq); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_display_installed_theories="); e != nil {
		return e
	}
	var vDisplayInstalledTheories int32
	if s.state.DisplayInstalledTheories {
		vDisplayInstalledTheories = 1
	}
	if e := out.AppendInt32(vDisplayInstalledTheories); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_core_validate="); e != nil {
		return e
	}
	var vCoreValidate int32
	if s.state.CoreValidate {
		vCoreValidate = 1
	}
	if e := out.AppendInt32(vCoreValidate); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_preprocess="); e != nil {
		return e
	}
	var vPreprocess int32
	if s.state.Preprocess {
		vPreprocess = 1
	}
	if e := out.AppendInt32(vPreprocess); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_user_theory_preprocess_axioms="); e != nil {
		return e
	}
	var vUserTheoryPreprocessAxioms int32
	if s.state.UserTheoryPreprocessAxioms {
		vUserTheoryPreprocessAxioms = 1
	}
	if e := out.AppendInt32(vUserTheoryPreprocessAxioms); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_user_theory_persist_axioms="); e != nil {
		return e
	}
	var vUserTheoryPersistAxioms int32
	if s.state.UserTheoryPersistAxioms {
		vUserTheoryPersistAxioms = 1
	}
	if e := out.AppendInt32(vUserTheoryPersistAxioms); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_timeout="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.Timeout); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_rlimit="); e != nil {
		return e
	}
	if e := out.AppendUint32(s.state.Rlimit); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_at_labels_cex="); e != nil {
		return e
	}
	var vAtLabelsCex int32
	if s.state.AtLabelsCex {
		vAtLabelsCex = 1
	}
	if e := out.AppendInt32(vAtLabelsCex); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_check_at_labels="); e != nil {
		return e
	}
	var vCheckAtLabels int32
	if s.state.CheckAtLabels {
		vCheckAtLabels = 1
	}
	if e := out.AppendInt32(vCheckAtLabels); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_dump_goal_as_smt="); e != nil {
		return e
	}
	var vDumpGoalAsSmt int32
	if s.state.DumpGoalAsSmt {
		vDumpGoalAsSmt = 1
	}
	if e := out.AppendInt32(vDumpGoalAsSmt); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_auto_config="); e != nil {
		return e
	}
	var vAutoConfig int32
	if s.state.AutoConfig {
		vAutoConfig = 1
	}
	if e := out.AppendInt32(vAutoConfig); e != nil {
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
