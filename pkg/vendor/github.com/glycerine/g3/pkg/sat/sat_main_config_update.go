package sat

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// These are the concrete generated sat_params accessors. Each uses the real
// params_ref typed local->module->literal fallback. Symbol default construction
// is evaluated even when the local value is present, as in generated C++.

func (m *satMainConfig) symbol(p, g *uparams.ParamsRef, key, def string) (usymbol.Symbol, error) {
	d, e := m.symbols.Intern(def)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	return m.context.GetSymbolFallbackName(p, &key, g, d)
}
func (m *satMainConfig) uint(p, g *uparams.ParamsRef, key string, d uint32, dst *uint32, mask uint32) error {
	var v uint32
	var e error
	if g == nil {
		v, e = m.context.GetUIntName(p, &key, d)
	} else {
		v, e = m.context.GetUIntFallbackName(p, &key, g, d)
	}
	if e != nil {
		return e
	}
	*dst = v
	m.state.Initialized |= mask
	return nil
}
func (m *satMainConfig) double(p, g *uparams.ParamsRef, key string, d float64, dst *float64, mask uint32) error {
	var v float64
	var e error
	if g == nil {
		v, e = m.context.GetDoubleName(p, &key, d)
	} else {
		v, e = m.context.GetDoubleFallbackName(p, &key, g, d)
	}
	if e != nil {
		return e
	}
	*dst = v
	m.state.Initialized |= mask
	return nil
}
func (m *satMainConfig) boolean(p, g *uparams.ParamsRef, key string, d bool, dst *bool, mask uint32) error {
	v, e := m.context.GetBoolFallbackName(p, &key, g, d)
	if e != nil {
		return e
	}
	*dst = v
	m.state.Initialized |= mask
	return nil
}

func (m *satMainConfig) Update(p *uparams.ParamsRef) (err error) {
	if e := m.open(); e != nil {
		return e
	}
	// Provenance safety guard precedes numeric actions. No caller-injected module
	// snapshot: each source sat_params constructor takes the current registry cell.
	if e := m.context.Check(p); e != nil {
		return e
	}
	g, e := m.moduleParams()
	if e != nil {
		return e
	}
	defer func() {
		if e := m.context.Del(&g); err == nil {
			err = e
		}
	}()
	key := "max_memory"
	mb, e := m.context.GetUIntFallbackName(p, &key, &g, ^uint32(0))
	if e != nil {
		return e
	}
	m.state.MaxMemory = satConfigMegabytesToBytes(mb)
	m.state.Initialized |= satConfigMaxMemoryMask
	s, e := m.symbol(p, &g, "restart", "luby")
	if e != nil {
		return e
	}
	if s == m.cached[5] {
		m.state.Restart = satRestartLuby
	} else if s == m.cached[4] {
		m.state.Restart = satRestartGeometric
	} else {
		return z3exception.NewDefaultException("invalid restart strategy")
	}
	m.state.Initialized |= satConfigRestartMask
	s, e = m.symbol(p, &g, "phase", "caching")
	if e != nil {
		return e
	}
	if s == m.cached[1] {
		m.state.Phase = satPhaseAlwaysFalse
	} else if s == m.cached[0] {
		m.state.Phase = satPhaseAlwaysTrue
	} else if s == m.cached[2] {
		m.state.Phase = satPhaseCaching
	} else if s == m.cached[3] {
		m.state.Phase = satPhaseRandom
	} else {
		return z3exception.NewDefaultException("invalid phase selection strategy")
	}
	m.state.Initialized |= satConfigPhaseMask
	if e = m.uint(p, &g, "phase.caching.on", 400, &m.state.PhaseCachingOn, satConfigPhaseCachingOnMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "phase.caching.off", 100, &m.state.PhaseCachingOff, satConfigPhaseCachingOffMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "restart.initial", 100, &m.state.RestartInitial, satConfigRestartInitialMask); e != nil {
		return e
	}
	if e = m.double(p, &g, "restart.factor", 1.5, &m.state.RestartFactor, satConfigRestartFactorMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "restart.max", ^uint32(0), &m.state.RestartMax, satConfigRestartMaxMask); e != nil {
		return e
	}
	if e = m.double(p, &g, "random_freq", .01, &m.state.RandomFreq, satConfigRandomFreqMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "random_seed", 0, &m.state.RandomSeed, satConfigRandomSeedMask); e != nil {
		return e
	}
	if m.state.RandomSeed == 0 {
		if e = m.uint(p, nil, "random_seed", 0, &m.state.RandomSeed, satConfigRandomSeedMask); e != nil {
			return e
		}
	}
	if e = m.uint(p, &g, "burst_search", 100, &m.state.BurstSearch, satConfigBurstSearchMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "max_conflicts", ^uint32(0), &m.state.MaxConflicts, satConfigMaxConflictsMask); e != nil {
		return e
	}
	if e = m.uint(p, &g, "parallel_threads", 1, &m.state.NumParallel, satConfigNumParallelMask); e != nil {
		return e
	}
	if e = m.uint(p, nil, "simplify_mult1", 300, &m.state.SimplifyMult1, satConfigSimplifyMult1Mask); e != nil {
		return e
	}
	if e = m.double(p, nil, "simplify_mult2", 1.5, &m.state.SimplifyMult2, satConfigSimplifyMult2Mask); e != nil {
		return e
	}
	if e = m.uint(p, nil, "simplify_max", 500000, &m.state.SimplifyMax, satConfigSimplifyMaxMask); e != nil {
		return e
	}
	s, e = m.symbol(p, &g, "gc", "glue_psm")
	if e != nil {
		return e
	}
	if s == m.cached[6] {
		m.state.GCStrategy = satGCDynPSM
		m.state.Initialized |= satConfigGCStrategyMask
		if e = m.uint(p, &g, "gc.initial", 20000, &m.state.GCInitial, satConfigGCInitialMask); e != nil {
			return e
		}
		if e = m.uint(p, &g, "gc.increment", 500, &m.state.GCIncrement, satConfigGCIncrementMask); e != nil {
			return e
		}
		if e = m.uint(p, &g, "gc.small_lbd", 3, &m.state.GCSmallLBD, satConfigGCSmallLBDMask); e != nil {
			return e
		}
		if e = m.uint(p, &g, "gc.k", 7, &m.state.GCK, satConfigGCKMask); e != nil {
			return e
		}
		if m.state.GCK > 255 {
			m.state.GCK = 255
		}
	} else {
		if s == m.cached[9] {
			m.state.GCStrategy = satGCGluePSM
		} else if s == m.cached[8] {
			m.state.GCStrategy = satGCGlue
		} else if s == m.cached[7] {
			m.state.GCStrategy = satGCPSM
		} else if s == m.cached[10] {
			m.state.GCStrategy = satGCPSMGlue
		} else {
			return z3exception.NewDefaultException("invalid gc strategy")
		}
		m.state.Initialized |= satConfigGCStrategyMask
		if e = m.uint(p, &g, "gc.initial", 20000, &m.state.GCInitial, satConfigGCInitialMask); e != nil {
			return e
		}
		if e = m.uint(p, &g, "gc.increment", 500, &m.state.GCIncrement, satConfigGCIncrementMask); e != nil {
			return e
		}
	}
	if e = m.boolean(p, &g, "minimize_lemmas", true, &m.state.MinimizeLemmas, satConfigMinimizeLemmasMask); e != nil {
		return e
	}
	if e = m.boolean(p, &g, "core.minimize", false, &m.state.CoreMinimize, satConfigCoreMinimizeMask); e != nil {
		return e
	}
	if e = m.boolean(p, &g, "core.minimize_partial", false, &m.state.CoreMinimizePartial, satConfigCoreMinimizePartialMask); e != nil {
		return e
	}
	if e = m.boolean(p, &g, "dyn_sub_res", true, &m.state.DynSubRes, satConfigDynSubResMask); e != nil {
		return e
	}
	return m.boolean(p, &g, "dimacs.display", false, &m.state.DimacsDisplay, satConfigDimacsDisplayMask)
}
