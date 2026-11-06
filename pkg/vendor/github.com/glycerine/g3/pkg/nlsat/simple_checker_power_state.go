package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:231-246,1458-1547.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

type simplePowerChecker struct {
	solver   *Solver
	domains  []simplePowerDomain
	learned  []sat.Literal
	seen     map[sat.Literal]bool
	improved bool
}

func newSimplePowerChecker(s *Solver) (c *simplePowerChecker, err error) {
	c = &simplePowerChecker{solver: s, seen: map[sat.Literal]bool{}}
	for range s.clauses.vars.isInt {
		d, cause := newSimplePowerDomain(s.config.Algebraic)
		if cause != nil {
			return c, errors.Join(cause, c.close())
		}
		c.domains = append(c.domains, d)
	}
	return c, nil
}
func (c *simplePowerChecker) close() (err error) {
	for i := len(c.domains) - 1; i >= 0; i-- {
		err = errors.Join(err, c.domains[i].close(c.solver.config.Algebraic))
	}
	return
}
func (c *simplePowerChecker) learn(l sat.Literal) {
	if !c.seen[l] {
		c.seen[l] = true
		c.learned = append(c.learned, l)
	}
}
func (c *simplePowerChecker) possible(f *simplePowerForm) (possible bool, err error) {
	if uint64(f.variable) >= uint64(len(c.domains)) {
		return true, nil
	}
	copy, err := c.domains[f.variable].clone(c.solver.config.Algebraic)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, copy.close(c.solver.config.Algebraic)) }()
	_, possible, err = copy.apply(c.solver.config.Algebraic, f)
	return
}
func (c *simplePowerChecker) force(f *simplePowerForm) (bool, error) {
	changed, ok, err := c.domains[f.variable].apply(c.solver.config.Algebraic, f)
	c.improved = c.improved || changed
	return ok, err
}
