package grobner

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *Solver) subset(a, b *Monomial) (bool, []ast.ASTHandle, error) {
	if len(a.vars) > len(b.vars) {
		return false, nil, nil
	}
	i, j := 0, 0
	var rest []ast.ASTHandle
	for {
		if i == len(a.vars) {
			return true, append(rest, b.vars[j:]...), nil
		}
		if j == len(b.vars) {
			return false, nil, nil
		}
		if a.vars[i] == b.vars[j] {
			i++
			j++
			continue
		}
		less, err := s.varLess(b.vars[j], a.vars[i])
		if err != nil {
			return false, nil, err
		}
		if less {
			rest = append(rest, b.vars[j])
			j++
			continue
		}
		return false, nil, nil
	}
}

func (s *Solver) mulAppend(start int, source *Equation, coeff *big.Rat, vars []ast.ASTHandle, out *[]*Monomial) error {
	for _, m := range source.monomials[start:] {
		c := new(big.Rat).Mul(&m.coeff, coeff)
		v := append(append([]ast.ASTHandle(nil), m.vars...), vars...)
		n, err := s.NewMonomial(c, v)
		if err != nil {
			return err
		}
		*out = append(*out, n)
	}
	return nil
}

func (s *Solver) copyEquation(e *Equation) (*Equation, error) {
	ms := make([]*Monomial, 0, len(e.monomials))
	for _, m := range e.monomials {
		n, err := s.cloneMonomial(m)
		if err != nil {
			for _, x := range ms {
				_ = s.delMonomial(x)
			}
			return nil, err
		}
		ms = append(ms, n)
	}
	r, err := s.initEquation(ms, e.dependency)
	if err != nil {
		for _, x := range ms {
			_ = s.delMonomial(x)
		}
		return nil, err
	}
	r.linear = e.linear
	return r, nil
}

func (s *Solver) replaceDependency(e *Equation, added ast.ASTDependency) error {
	joined, err := s.manager.DependencyJoin(e.dependency, added)
	if err != nil {
		return err
	}
	if joined == e.dependency {
		return nil
	}
	if err = s.manager.DependencyInc(joined); err != nil {
		return err
	}
	err = s.manager.DependencyDec(e.dependency)
	e.dependency = joined
	return err
}

// reduce returns the target (or its scope-safe copy), whether a reduction was
// made, and any failure.
func (s *Solver) reduce(source, target *Equation) (*Equation, bool, error) {
	if len(source.monomials) == 0 {
		return target, false, nil
	}
	s.Stats.Simplify++
	result := false
	for {
		simplified := false
		lead := source.monomials[0]
		kept := make([]*Monomial, 0, len(target.monomials))
		var added []*Monomial
		for i, current := range target.monomials {
			matched, rest, err := s.subset(lead, current)
			if err != nil {
				return target, result, err
			}
			if !matched {
				kept = append(kept, current)
				continue
			}
			if i == 0 {
				s.changedLeading = true
			}
			if source.scope > target.scope {
				target, err = s.copyEquation(target)
				if err != nil {
					return target, result, err
				}
				// Restart against the copy so references in kept still belong to it.
				kept = kept[:0]
				added = added[:0]
				simplified = false
				break
			}
			if !result {
				if err := s.replaceDependency(target, source.dependency); err != nil {
					return target, result, err
				}
			}
			result, simplified = true, true
			coeff := new(big.Rat).Quo(&current.coeff, &lead.coeff)
			coeff.Neg(coeff)
			if len(rest) != 0 {
				target.linear = false
			}
			if err := s.mulAppend(1, source, coeff, rest, &added); err != nil {
				return target, result, err
			}
			if err := s.delMonomial(current); err != nil {
				return target, result, err
			}
		}
		if source.scope > target.scope {
			continue
		}
		if simplified {
			target.monomials = append(kept, added...)
			var err error
			target.monomials, err = s.simplifyMonomials(target.monomials)
			if err != nil {
				return target, result, err
			}
			if inconsistent(target) && s.unsat == nil {
				s.unsat = target
			}
		}
		canceled, err := s.manager.Canceled()
		if err != nil {
			return target, result, err
		}
		if !simplified || canceled {
			return target, result, nil
		}
	}
}

func (s *Solver) reduceProcessed(e *Equation) (*Equation, bool, error) {
	result := false
	for {
		changed := false
		for _, p := range s.processed.snapshot() {
			var reduced bool
			var err error
			e, reduced, err = s.reduce(p, e)
			if err != nil {
				return e, result, err
			}
			result, changed = result || reduced, changed || reduced
			canceled, err := s.manager.Canceled()
			if err != nil || canceled {
				return e, result, err
			}
		}
		if !changed {
			return e, result, nil
		}
	}
}

func better(a, b *Equation) bool {
	if b == nil || len(a.monomials) == 0 {
		return true
	}
	if len(b.monomials) == 0 {
		return false
	}
	da, db := len(a.monomials[0].vars), len(b.monomials[0].vars)
	if da != db {
		return da < db
	}
	return len(a.monomials) < len(b.monomials)
}

func (s *Solver) pickNext() (*Equation, error) {
	var selected *Equation
	for _, e := range s.pending.snapshot() {
		if len(e.monomials) == 0 {
			if err := s.deleteEquation(e); err != nil {
				return nil, err
			}
		} else if better(e, selected) {
			selected = e
		}
	}
	if selected != nil {
		s.pending.erase(selected)
	}
	return selected, nil
}

func (s *Solver) simplifyProcessed(rule *Equation) (bool, error) {
	var insert, remove, del []*Equation
	for _, current := range s.processed.snapshot() {
		canceled, err := s.manager.Canceled()
		if err != nil || canceled {
			return false, err
		}
		s.changedLeading = false
		newCurrent, changed, err := s.reduce(rule, current)
		if err != nil {
			return false, err
		}
		if changed {
			if newCurrent != current {
				s.unfreeze = append(s.unfreeze, current)
				remove = append(remove, current)
				if s.changedLeading {
					s.pending.insert(newCurrent)
				} else {
					insert = append(insert, newCurrent)
				}
			} else if s.changedLeading {
				s.pending.insert(current)
				remove = append(remove, current)
			}
		}
		if len(newCurrent.monomials) == 0 {
			del = append(del, newCurrent)
		}
	}
	for _, e := range insert {
		s.processed.insert(e)
	}
	for _, e := range remove {
		s.processed.erase(e)
	}
	for _, e := range del {
		if err := s.deleteEquation(e); err != nil {
			return false, err
		}
	}
	canceled, err := s.manager.Canceled()
	return !canceled, err
}

func (s *Solver) simplifyPending(rule *Equation) error {
	var insert, remove, del []*Equation
	for _, current := range s.pending.snapshot() {
		newCurrent, changed, err := s.reduce(rule, current)
		if err != nil {
			return err
		}
		if changed && newCurrent != current {
			s.unfreeze = append(s.unfreeze, current)
			insert = append(insert, newCurrent)
			remove = append(remove, current)
		}
		if len(newCurrent.monomials) == 0 {
			del = append(del, newCurrent)
		}
	}
	for _, e := range insert {
		s.pending.insert(e)
	}
	for _, e := range remove {
		s.pending.erase(e)
	}
	for _, e := range del {
		if err := s.deleteEquation(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) unify(a, b *Monomial) (bool, []ast.ASTHandle, []ast.ASTHandle, error) {
	found, i, j := false, 0, 0
	var ra, rb []ast.ASTHandle
	for {
		if i == len(a.vars) {
			if found {
				return true, ra, append(rb, b.vars[j:]...), nil
			}
			return false, nil, nil, nil
		}
		if j == len(b.vars) {
			if found {
				return true, append(ra, a.vars[i:]...), rb, nil
			}
			return false, nil, nil, nil
		}
		if a.vars[i] == b.vars[j] {
			found = true
			i++
			j++
			continue
		}
		less, err := s.varLess(b.vars[j], a.vars[i])
		if err != nil {
			return false, nil, nil, err
		}
		if less {
			rb = append(rb, b.vars[j])
			j++
		} else {
			ra = append(ra, a.vars[i])
			i++
		}
	}
}

func (s *Solver) superpose(a, b *Equation) error {
	if len(a.monomials) == 0 || len(b.monomials) == 0 {
		return nil
	}
	s.Stats.Superpose++
	ok, ra, rb, err := s.unify(a.monomials[0], b.monomials[0])
	if err != nil || !ok {
		return err
	}
	var ms []*Monomial
	if err = s.mulAppend(1, a, &b.monomials[0].coeff, rb, &ms); err != nil {
		return err
	}
	c := new(big.Rat).Neg(&a.monomials[0].coeff)
	if err = s.mulAppend(1, b, c, ra, &ms); err != nil {
		return err
	}
	ms, err = s.simplifyMonomials(ms)
	if err != nil {
		return err
	}
	if len(ms) == 0 {
		return nil
	}
	s.numNew++
	dep, err := s.manager.DependencyJoin(a.dependency, b.dependency)
	if err != nil {
		return err
	}
	e, err := s.initEquation(ms, dep)
	if err != nil {
		return err
	}
	e.linear = false
	s.pending.insert(e)
	return nil
}

func (s *Solver) ComputeBasisInit() error {
	if err := s.check(); err != nil {
		return err
	}
	s.Stats.ComputeBasis++
	s.numNew = 0
	return nil
}

// ComputeBasisStep returns true when no equation remains to process.
func (s *Solver) ComputeBasisStep() (bool, error) {
	if err := s.check(); err != nil {
		return false, err
	}
	e, err := s.pickNext()
	if err != nil || e == nil {
		return e == nil, err
	}
	s.Stats.NumProcessed++
	original := e
	e, changed, err := s.reduceProcessed(e)
	if err != nil {
		return false, err
	}
	if changed && e != original {
		s.unfreeze = append(s.unfreeze, original)
	}
	canceled, err := s.manager.Canceled()
	if err != nil || canceled {
		return false, err
	}
	ok, err := s.simplifyProcessed(e)
	if err != nil || !ok {
		return false, err
	}
	for _, p := range s.processed.snapshot() {
		if err := s.superpose(e, p); err != nil {
			return false, err
		}
	}
	s.processed.insert(e)
	if err := s.simplifyPending(e); err != nil {
		return false, err
	}
	return false, nil
}

// ComputeBasis returns true when the worklist empties before the generated
// equation threshold or cancellation is reached.
func (s *Solver) ComputeBasis(threshold uint32) (bool, error) {
	if err := s.ComputeBasisInit(); err != nil {
		return false, err
	}
	for s.numNew < threshold {
		canceled, err := s.manager.Canceled()
		if err != nil || canceled {
			return false, err
		}
		done, err := s.ComputeBasisStep()
		if err != nil || done {
			return done, err
		}
	}
	return false, nil
}
