package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:231-246,1458-1547.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

type simpleSumChecker struct {
	solver   *Solver
	domains  []simplePowerDomain
	learned  []sat.Literal
	seen     map[sat.Literal]bool
	improved bool
}

func newSimpleSumChecker(s *Solver) (c *simpleSumChecker, err error) {
	c = &simpleSumChecker{solver: s, seen: map[sat.Literal]bool{}}
	for range s.clauses.vars.isInt {
		d, cause := newSimplePowerDomain(s.config.Algebraic)
		if cause != nil {
			return c, errors.Join(cause, c.close())
		}
		c.domains = append(c.domains, d)
	}
	return
}
func (c *simpleSumChecker) close() (err error) {
	for i := len(c.domains) - 1; i >= 0; i-- {
		err = errors.Join(err, c.domains[i].close(c.solver.config.Algebraic))
	}
	return
}
func (c *simpleSumChecker) learn(l sat.Literal) {
	if !c.seen[l] {
		c.seen[l] = true
		c.learned = append(c.learned, l)
	}
}
