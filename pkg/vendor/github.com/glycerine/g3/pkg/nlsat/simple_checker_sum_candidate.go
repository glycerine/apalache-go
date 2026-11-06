package nlsat

// Concrete retained specializations; no any-valued callback/template layer.

import "github.com/glycerine/g3/pkg/sat"

type simpleSumCandidate struct {
	kind   uint8
	linear simpleLinearForm
	power  simplePowerForm
	sum    simpleSumForm
}

func (s *Solver) simpleCombinedCandidate(l sat.Literal) (c simpleSumCandidate, supported bool, err error) {
	if c.linear, supported, err = s.simpleLinearLiteral(l); err != nil || supported {
		c.kind = 1
		return
	}
	if c.power, supported, err = s.simplePowerLiteral(l); err != nil || supported {
		c.kind = 2
		return
	}
	if c.sum, supported, err = s.simpleSumLiteral(l); supported {
		c.kind = 3
	}
	return
}
func (c *simpleSumCandidate) close(s *Solver) error {
	switch c.kind {
	case 1:
		return c.linear.close(s.config.Algebraic)
	case 2:
		return c.power.close(s.config.Algebraic)
	case 3:
		return c.sum.close(s.config.Algebraic)
	}
	return nil
}
