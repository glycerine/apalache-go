package ast

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) proofComplement(a, b ASTHandle) (bool, error) {
	if a == m.external(m.trueApp) && b == m.external(m.falseApp) {
		return true, nil
	}
	if yes, e := m.proofKind(a, ASTOpNot); e != nil {
		return false, e
	} else if yes {
		x, e := m.proofArg(a, 0)
		if e != nil {
			return false, e
		}
		if x == b {
			return true, nil
		}
	}
	if b == m.external(m.trueApp) && a == m.external(m.falseApp) {
		return true, nil
	}
	if yes, e := m.proofKind(b, ASTOpNot); e != nil {
		return false, e
	} else if yes {
		x, e := m.proofArg(b, 0)
		return x == a, e
	}
	return false, nil
}
func (m *ASTManager) MkProofUnitResolution(ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if ASTSourceDebug {
		if len(ps) < 2 {
			return ASTHandle{}, ErrASTDomain
		}
		for _, p := range ps {
			yes, e := m.proofHasFact(p)
			if e != nil {
				return ASTHandle{}, e
			}
			if !yes {
				return ASTHandle{}, ErrASTDomain
			}
		}
	}
	var args []ASTHandle
	args = append(args, ps...)
	if len(ps) < 2 {
		return ASTHandle{}, arena.ErrBounds
	}
	f, e := m.proofFact(ps[0])
	if e != nil {
		return ASTHandle{}, e
	}
	g, e := m.proofFact(ps[1])
	if e != nil {
		return ASTHandle{}, e
	}
	fact := ASTHandle{}
	complement := false
	if len(ps) == 2 {
		complement, e = m.proofComplement(f, g)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	if len(ps) == 2 && complement {
		fact = m.external(m.falseApp)
	} else {
		if ASTSourceDebug || m.proofResolutionTraceEnabled("mk_unit_resolution_bug") {
			yes, e := m.proofKind(f, ASTOpOr)
			if e != nil {
				return ASTHandle{}, e
			}
			if !yes && m.proofResolutionTraceEnabled("mk_unit_resolution_bug") {
				m.mu.Unlock()
				traceErr := m.traceUnitResolutionBadClause(f, g)
				m.mu.Lock()
				if traceErr != nil {
					return ASTHandle{}, traceErr
				}
			}
			if ASTSourceDebug && !yes {
				return ASTHandle{}, ErrASTDomain
			}
		}
		r, e := m.requireKind(f, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		var lits []ASTHandle
		var found []bool
		if ASTSourceDebug {
			found = make([]bool, len(ps))
		}
		for i := uint32(0); i < m.appAt(r.slot).count; i++ {
			lit, e := m.proofArg(f, i)
			if e != nil {
				return ASTHandle{}, e
			}
			matched := false
			for j := 1; j < len(ps); j++ {
				other, e := m.proofFact(ps[j])
				if e != nil {
					return ASTHandle{}, e
				}
				yes, e := m.proofComplement(lit, other)
				if e != nil {
					return ASTHandle{}, e
				}
				if yes {
					matched = true
					if ASTSourceDebug {
						found[j] = true
						continue
					}
					break
				}
			}
			if !matched {
				lits = append(lits, lit)
			}
		}
		if ASTSourceDebug && m.proofMode != ASTProofDisabled {
			for j := 1; j < len(ps); j++ {
				if j >= len(found) || !found[j] {
					if m.proofResolutionTraceEnabled("mk_unit_resolution_bug") {
						if e := m.traceUnitResolutionUnmatched(ps, j); e != nil {
							return ASTHandle{}, e
						}
					}
					return ASTHandle{}, ErrASTDomain
				}
			}
		}
		switch len(lits) {
		case 0:
			fact = m.external(m.falseApp)
		case 1:
			fact = lits[0]
		default:
			fact, e = m.proofMake(ASTOpOr, lits...)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		// Retire borrowed branch scratch before constructing the final proof.
		found = nil
		lits = nil
	}
	args = append(args, fact)
	proof, err := m.proofMake(ASTPrUnitResolution, args...)
	if err != nil {
		return ASTHandle{}, err
	}
	if err = m.traceUnitResolution(proof, 2964, "unit_resolution generating fact\n"); err != nil {
		return ASTHandle{}, err
	}
	return proof, nil
}
func (m *ASTManager) MkProofUnitResolutionFact(ps []ASTHandle, fact ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofResolutionTraceEnabled("unit_bug") {
		m.mu.Unlock()
		traceErr := m.traceUnitResolutionFactInput("unit_bug", 2969, ps, fact)
		m.mu.Lock()
		if traceErr != nil {
			return ASTHandle{}, traceErr
		}
	}
	var args []ASTHandle
	args = append(args, ps...)
	args = append(args, fact)
	if ASTSourceDebug {
		if len(ps) < 2 {
			return ASTHandle{}, arena.ErrBounds
		}
		f, e := m.proofFact(ps[0])
		if e != nil {
			return ASTHandle{}, e
		}
		g, e := m.proofFact(ps[1])
		if e != nil {
			return ASTHandle{}, e
		}
		yes := false
		if len(ps) == 2 {
			yes, e = m.proofComplement(f, g)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		if len(ps) == 2 && yes {
			if fact != m.external(m.falseApp) {
				return ASTHandle{}, ErrASTDomain
			}
		} else {
			yes, e := m.proofKind(f, ASTOpOr)
			if e != nil {
				return ASTHandle{}, e
			}
			if !yes {
				return ASTHandle{}, ErrASTDomain
			}
			r, e := m.requireKind(f, ASTApp)
			if e != nil {
				return ASTHandle{}, e
			}
			n := m.appAt(r.slot).count
			if len(ps) != int(n) && !(len(ps) == int(n)+1 && fact == m.external(m.falseApp)) && m.proofResolutionTraceEnabled("cunit_bug") {
				m.mu.Unlock()
				traceErr := m.traceUnitResolutionFactInput("cunit_bug", 2987, ps, fact)
				m.mu.Lock()
				if traceErr != nil {
					return ASTHandle{}, traceErr
				}
			}
			matches := uint32(0)
			for i := uint32(0); i < n; i++ {
				lit, e := m.proofArg(f, i)
				if e != nil {
					return ASTHandle{}, e
				}
				matched := false
				for j := 1; j < len(ps); j++ {
					other, e := m.proofFact(ps[j])
					if e != nil {
						return ASTHandle{}, e
					}
					yes, e := m.proofComplement(lit, other)
					if e != nil {
						return ASTHandle{}, e
					}
					if yes {
						matches++
						matched = true
						break
					}
				}
				if !matched && fact != lit {
					if m.proofResolutionTraceEnabled("unit_bug1") {
						if e := m.traceUnitResolutionWrongFact(fact, lit); e != nil {
							return ASTHandle{}, e
						}
					}
					return ASTHandle{}, ErrASTDomain
				}
			}
			if matches != n && matches != n-1 {
				return ASTHandle{}, ErrASTDomain
			}
			if matches == n {
				if fact != m.external(m.falseApp) {
					return ASTHandle{}, ErrASTDomain
				}
			}
		}
	}
	proof, err := m.proofMake(ASTPrUnitResolution, args...)
	if err != nil {
		return ASTHandle{}, err
	}
	if err = m.traceUnitResolution(proof, 3015, "unit_resolution using fact\n"); err != nil {
		return ASTHandle{}, err
	}
	return proof, nil
}
