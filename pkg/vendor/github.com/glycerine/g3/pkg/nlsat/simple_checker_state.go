package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:231-246,1458-1547.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

type simpleLinearChecker struct {
	solver     *Solver
	domains    []simpleInterval
	learned    []sat.Literal
	learnedSet map[sat.Literal]bool
	improved   bool
}

func newSimpleLinearChecker(s *Solver) *simpleLinearChecker {
	c := &simpleLinearChecker{solver: s, learnedSet: map[sat.Literal]bool{}}
	for range s.clauses.vars.isInt {
		c.domains = append(c.domains, newSimpleInterval())
	}
	return c
}
func (c *simpleLinearChecker) close() (err error) {
	am := c.solver.config.Algebraic
	for i := len(c.domains) - 1; i >= 0; i-- {
		err = errors.Join(err, c.domains[i].close(am))
	}
	return err
}
func (c *simpleLinearChecker) learn(l sat.Literal) {
	if !c.learnedSet[l] {
		c.learnedSet[l] = true
		c.learned = append(c.learned, l)
	}
}
func (c *simpleLinearChecker) form(l sat.Literal) (simpleLinearForm, bool, error) {
	return c.solver.simpleLinearLiteral(l)
}
func (c *simpleLinearChecker) possible(f *simpleLinearForm) (bool, error) {
	if uint64(f.variable) >= uint64(len(c.domains)) {
		return true, nil
	}
	return c.domains[f.variable].permits(c.solver.config.Algebraic, f.sign, &f.boundary)
}
func (c *simpleLinearChecker) force(f *simpleLinearForm) (bool, error) {
	if f.sign == simpleNEQ {
		return true, nil
	}
	changed, consistent, err := c.domains[f.variable].intersect(c.solver.config.Algebraic, f.sign, &f.boundary)
	c.improved = c.improved || changed
	return consistent, err
}
