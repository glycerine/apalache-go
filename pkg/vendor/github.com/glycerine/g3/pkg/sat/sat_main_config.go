package sat

// Translation of src/sat/sat_config.h/.cpp, Leonardo de Moura, 2011-05-21.

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrSATMainConfigOwner = errors.New("g3: SAT main configuration owner or lifetime mismatch")

type satPhaseSelection uint32

const (
	satPhaseAlwaysTrue satPhaseSelection = iota
	satPhaseAlwaysFalse
	satPhaseCaching
	satPhaseRandom
)

type satRestartStrategy uint32

const (
	satRestartGeometric satRestartStrategy = iota
	satRestartLuby
)

type satGCStrategy uint32

const (
	satGCDynPSM satGCStrategy = iota
	satGCPSM
	satGCGlue
	satGCGluePSM
	satGCPSMGlue
)

// The mask describes assignments performed by the source, rather than treating
// Go's zero initialization as a source assignment. Non-dyn policies leave the
// SmallLBD/K fields indeterminate on construction and retain them on updates.
const (
	satConfigMaxMemoryMask uint32 = 1 << iota
	satConfigPhaseMask
	satConfigPhaseCachingOnMask
	satConfigPhaseCachingOffMask
	satConfigRestartMask
	satConfigRestartInitialMask
	satConfigRestartFactorMask
	satConfigRestartMaxMask
	satConfigRandomFreqMask
	satConfigRandomSeedMask
	satConfigBurstSearchMask
	satConfigMaxConflictsMask
	satConfigNumParallelMask
	satConfigSimplifyMult1Mask
	satConfigSimplifyMult2Mask
	satConfigSimplifyMaxMask
	satConfigGCStrategyMask
	satConfigGCInitialMask
	satConfigGCIncrementMask
	satConfigGCSmallLBDMask
	satConfigGCKMask
	satConfigMinimizeLemmasMask
	satConfigDynSubResMask
	satConfigCoreMinimizeMask
	satConfigCoreMinimizePartialMask
	satConfigDimacsDisplayMask
	satConfigAllFieldsMask = (satConfigDimacsDisplayMask << 1) - 1
)

type satMainConfigState struct {
	MaxMemory                                                                   uint64
	Phase                                                                       satPhaseSelection
	PhaseCachingOn, PhaseCachingOff                                             uint32
	Restart                                                                     satRestartStrategy
	RestartInitial                                                              uint32
	RestartFactor                                                               float64
	RestartMax                                                                  uint32
	RandomFreq                                                                  float64
	RandomSeed, BurstSearch, MaxConflicts, NumParallel                          uint32
	SimplifyMult1                                                               uint32
	SimplifyMult2                                                               float64
	SimplifyMax                                                                 uint32
	GCStrategy                                                                  satGCStrategy
	GCInitial, GCIncrement, GCSmallLBD, GCK                                     uint32
	MinimizeLemmas, DynSubRes, CoreMinimize, CoreMinimizePartial, DimacsDisplay bool
	Initialized                                                                 uint32
}

// One charged cold owner. The runtime's actual GlobalParams/ParamsContext and
// SymbolManager are borrowed and must outlive it. Arithmetic/configuration is
// confined by the caller; Budget is also owner-confined. Never copy this header.
type satMainConfig struct {
	self          *satMainConfig
	globals       *uparams.GlobalParams
	globalService *uparams.GlobalParamsService
	context       *uparams.ParamsContext
	symbols       *usymbol.Manager
	budget        *arena.Budget
	state         satMainConfigState
	cached        [11]usymbol.Symbol
	ownerBytes    uint64
	closed        bool
}

func newSATMainConfig(g *uparams.GlobalParams, p *uparams.ParamsRef, b *arena.Budget) (*satMainConfig, error) {
	return newSATMainConfigWithService(g, nil, p, b)
}
func newSATMainConfigWithService(g *uparams.GlobalParams, service *uparams.GlobalParamsService, p *uparams.ParamsRef, b *arena.Budget) (*satMainConfig, error) {
	if g == nil && service == nil {
		return nil, ErrSATMainConfigOwner
	}
	var context *uparams.ParamsContext
	var symbols *usymbol.Manager
	if service != nil {
		if e := service.CheckOpen(); e != nil {
			return nil, e
		}
		context, symbols = service.Context, service.Symbols
	} else {
		if e := g.CheckOpen(); e != nil {
			return nil, e
		}
		context, symbols = g.Context(), g.Symbols()
	}
	if e := context.Check(p); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satMainConfig{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	m := &satMainConfig{globals: g, globalService: service, context: context, symbols: symbols, budget: b, ownerBytes: n}
	m.self = m
	// Source initializer preceding all eleven symbol constructors.
	m.state.RestartMax = 0
	m.state.Initialized = satConfigRestartMaxMask
	names := [11]string{"always_true", "always_false", "caching", "random", "geometric", "luby", "dyn_psm", "psm", "glue", "glue_psm", "psm_glue"}
	for i, n := range names {
		s, e := m.symbols.Intern(n)
		if e != nil {
			_ = m.Close()
			return nil, e
		}
		m.cached[i] = s
	}
	// Source constructor body, after symbol constructors and before updt_params.
	m.state.NumParallel = 1
	m.state.Initialized |= satConfigNumParallelMask
	if e := m.Update(p); e != nil {
		_ = m.Close()
		return nil, e
	}
	return m, nil
}
func (m *satMainConfig) open() error {
	if m == nil || m.self != m {
		return ErrSATMainConfigOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.globalService != nil {
		return m.globalService.CheckOpen()
	}
	return m.globals.CheckOpen()
}

func (m *satMainConfig) moduleParams() (uparams.ParamsRef, error) {
	if m.globalService != nil {
		return m.globalService.SourceGetModuleName("sat")
	}
	return m.globals.GetModuleName("sat")
}
func (m *satMainConfig) State() (satMainConfigState, error) {
	if e := m.open(); e != nil {
		return satMainConfigState{}, e
	}
	return m.state, nil
}

// Clone implements the source implicit scalar/symbol copy constructor. It
// performs no parameter reads, symbol construction, or source Update call.
func (m *satMainConfig) Clone(b *arena.Budget) (*satMainConfig, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satMainConfig{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	r := &satMainConfig{globals: m.globals, globalService: m.globalService, context: m.context, symbols: m.symbols, budget: b, state: m.state, cached: m.cached, ownerBytes: n}
	r.self = r
	return r, nil
}

// Assign copies only source members. Go cold ownership/borrowed runtime bindings
// remain those of the destination. A foreign symbol domain has no source-global
// equivalent and is rejected before any assignment.
func (m *satMainConfig) Assign(src *satMainConfig) error {
	if e := m.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	if m.symbols != src.symbols {
		return ErrSATMainConfigOwner
	}
	m.state = src.state
	m.cached = src.cached
	return nil
}
func (m *satMainConfig) Close() error {
	if m == nil || m.self != m {
		return ErrSATMainConfigOwner
	}
	if m.closed {
		return nil
	}
	m.budget.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.closed = true
	return nil
}

func satConfigMegabytesForBits(mb uint32, bits uint8) uint64 {
	max := ^uint64(0)
	if bits == 32 {
		max = uint64(^uint32(0))
	}
	if mb == ^uint32(0) {
		return max
	}
	n := uint64(mb) * 1024 * 1024
	if n > max {
		return max
	}
	return n
}
func satConfigMegabytesToBytes(mb uint32) uint64 {
	return satConfigMegabytesForBits(mb, uint8(strconv.IntSize))
}
