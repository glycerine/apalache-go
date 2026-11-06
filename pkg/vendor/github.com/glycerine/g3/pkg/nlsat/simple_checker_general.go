package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1099-1563; general polynomial
// interval/sign specialization over the retained linear/power/sum domains.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

func (c *simpleSumChecker) checkGeneralClause(clause *Clause) (consistent bool, err error) {
	type item struct {
		literal             sat.Literal
		candidate           simpleGeneralCandidate
		supported, possible bool
	}
	items := make([]item, 0, clause.Size())
	defer func() {
		for i := range items {
			if items[i].supported && items[i].candidate.kind == 1 {
				err = errors.Join(err, items[i].candidate.base.close(c.solver))
			}
		}
	}()
	count, forced := 0, -1
	for _, literal := range clause.Literals() {
		candidate, supported, cause := c.solver.simpleGeneralCandidate(literal)
		if cause != nil {
			return false, cause
		}
		current := item{literal: literal, candidate: candidate, supported: supported, possible: true}
		if supported {
			if candidate.kind == 1 {
				current.possible, cause = c.possible(&candidate.base)
			} else {
				current.possible, cause = c.evaluateGeneral(&candidate.general, c.domains)
			}
			if cause != nil {
				return false, cause
			}
			if !current.possible {
				c.learn(literal.Negated())
			}
		}
		items = append(items, current)
		if current.possible {
			count++
			forced = len(items) - 1
		}
	}
	if count == 0 {
		return false, nil
	}
	if count != 1 || !items[forced].supported {
		return true, nil
	}
	if items[forced].candidate.kind == 1 {
		return c.force(&items[forced].candidate.base)
	}
	return true, nil
}
func (s *Solver) SimpleCheckPolynomialSigns() (consistent bool, learned []sat.Literal, err error) {
	if err = s.open(); err != nil {
		return false, nil, err
	}
	checker, err := newSimpleSumChecker(s)
	if err != nil {
		return false, nil, err
	}
	defer func() { err = errors.Join(err, checker.close()) }()
	for {
		checker.improved = false
		for _, clause := range s.clauses.originals {
			ok, cause := checker.checkGeneralClause(clause)
			if cause != nil {
				return false, nil, cause
			}
			if !ok {
				return false, append([]sat.Literal(nil), checker.learned...), nil
			}
		}
		if !checker.improved {
			return true, append([]sat.Literal(nil), checker.learned...), nil
		}
	}
}
