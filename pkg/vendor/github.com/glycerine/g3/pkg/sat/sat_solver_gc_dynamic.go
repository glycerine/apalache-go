package sat

// Source: src/sat/sat_solver.cpp:1518–1670.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (g *satSolverGC) gcDynPSM() error {
	g.traceGC("gc_dyn_psm", 1519)
	l := g.lifecycle()
	p := l.propagation
	v := p.variables
	a := p.clauses
	if satStorageSourceDebug && v.scopeLevel != 0 {
		return ErrSATAssertion
	}
	h, V := uint32(0), uint32(0)
	n := v.NumVars()
	for i := uint32(0); i < n; i++ {
		assigned, e := v.gcAssigned(BoolVar(i))
		if e != nil {
			return e
		}
		if assigned {
			V++
			if e = v.setGCAssigned(BoolVar(i), false); e != nil {
				return e
			}
		}
		x, e := v.phaseAt(BoolVar(i))
		if e != nil {
			return e
		}
		old, e := v.previousPhaseAt(BoolVar(i))
		if e != nil {
			return e
		}
		if x != old {
			h++
			x, e = v.phaseAt(BoolVar(i))
			if e != nil {
				return e
			}
			if e = v.setPreviousPhase(BoolVar(i), x); e != nil {
				return e
			}
		}
	}
	d := float64(n + 1)
	if V != 0 {
		d = float64(h) / float64(V)
	}
	if d < g.minDTk {
		g.minDTk = d
	}
	g.traceMinDTk()
	frozen, activated, deleted := uint32(0), uint32(0), uint32(0)
	if e := g.beginCompaction(0); e != nil {
		return e
	}
	defer func() { g.running = false }()
	cfg, e := p.config.State()
	if e != nil {
		return e
	}
	for i := uint32(0); i < g.end; i++ {
		r := l.learned[i]
		f, e := a.Frozen(r)
		if e != nil {
			return e
		}
		if !f {
			glue, e := a.Glue(r)
			if e != nil {
				return e
			}
			if cfg.Initialized&satConfigGCSmallLBDMask == 0 {
				return ErrSATGCUninitialized
			}
			if glue > cfg.GCSmallLBD {
				used, e := a.Used(r)
				if e != nil {
					return e
				}
				if used {
					if e = a.ResetInactivity(r); e != nil {
						return e
					}
				} else {
					if e = a.IncInactivity(r); e != nil {
						return e
					}
					inactive, e := a.Inactivity(r)
					if e != nil {
						return e
					}
					if cfg.Initialized&satConfigGCKMask == 0 {
						return ErrSATGCUninitialized
					}
					if inactive > cfg.GCK {
						if e = l.detachClause(r); e != nil {
							return e
						}
						if e = l.delClause(r); e != nil {
							return e
						}
						g.gcClauseStat++
						deleted++
						g.cursor = i + 1
						continue
					}
				}
				if e = a.UnmarkUsed(r); e != nil {
					return e
				}
				measure, e := g.psm(r)
				if e != nil {
					return e
				}
				sz, e := a.Size(r)
				if e != nil {
					return e
				}
				bound, e := satGCUnsigned(float64(sz) * g.minDTk)
				if e != nil {
					return e
				}
				if measure > bound {
					g.traceFreezingClause(r, sz, measure)
					if e = l.detachClause(r); e != nil {
						return e
					}
					if e = a.ResetInactivity(r); e != nil {
						return e
					}
					if e = a.Freeze(r); e != nil {
						return e
					}
					g.numFrozen++
					frozen++
				}
			}
		} else {
			measure, e := g.psm(r)
			if e != nil {
				return e
			}
			sz, e := a.Size(r)
			if e != nil {
				return e
			}
			bound, e := satGCUnsigned(float64(sz) * g.minDTk)
			if e != nil {
				return e
			}
			if measure <= bound {
				if e = a.Unfreeze(r); e != nil {
					return e
				}
				g.numFrozen--
				activated++
				keep, e := g.activateFrozenClause(r)
				if e != nil {
					return e
				}
				if !keep {
					if e = l.delClause(r); e != nil {
						return e
					}
					g.cursor = i + 1
					continue
				}
			} else {
				if e = a.IncInactivity(r); e != nil {
					return e
				}
				inactive, e := a.Inactivity(r)
				if e != nil {
					return e
				}
				if cfg.Initialized&satConfigGCKMask == 0 {
					return ErrSATGCUninitialized
				}
				if inactive > cfg.GCK {
					g.numFrozen--
					if e = l.delClause(r); e != nil {
						return e
					}
					g.gcClauseStat++
					deleted++
					g.cursor = i + 1
					continue
				}
			}
		}
		l.learned[g.out] = r
		g.out++
		g.cursor = i + 1
	}
	g.finishCompaction()
	return g.emitDynamicReport(d, frozen, activated, deleted)
}
func (g *satSolverGC) activateFrozenClause(r StoredClauseRef) (bool, error) {
	g.traceReactivatingClause(r)
	l := g.lifecycle()
	p := l.propagation
	a := p.clauses
	if satStorageSourceDebug && p.variables.scopeLevel != 0 {
		return false, ErrSATAssertion
	}
	n, e := a.Size(r)
	if e != nil {
		return false, e
	}
	j := uint32(0)
	for i := uint32(0); i < n; i++ {
		lit, e := a.Literal(r, i)
		if e != nil {
			return false, e
		}
		value, e := p.variables.valueLiteral(lit)
		if e != nil {
			return false, e
		}
		switch value {
		case lbool.True:
			return false, nil
		case lbool.False:
		case lbool.Undef:
			x, e := a.Literal(r, i)
			if e != nil {
				return false, e
			}
			if e = a.SetLiteral(r, j, x); e != nil {
				return false, e
			}
			j++
		}
	}
	g.traceAfterClauseCleanup(r, j)
	switch j {
	case 0:
		p.setConflict(satJustification{}, DefaultLiteral())
		return false, nil
	case 1:
		lit, e := a.Literal(r, 0)
		if e != nil {
			return false, e
		}
		return false, p.assign(lit, satJustification{})
	case 2:
		x, e := a.Literal(r, 0)
		if e != nil {
			return false, e
		}
		y, e := a.Literal(r, 1)
		if e != nil {
			return false, e
		}
		return false, l.mkBinClause(x, y, true)
	default:
		if e = a.Shrink(r, j); e != nil {
			return false, e
		}
		return true, l.attachClause(r)
	}
}
