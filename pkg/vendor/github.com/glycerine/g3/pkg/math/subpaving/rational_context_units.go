package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:773-784,865-875,1773-1803,
// concrete config_mpq inequality and root-unit prefix.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrRationalContextCanceled = z3exception.NewDefaultException(messages.Canceled)

type RationalInequality struct {
	context  *RationalContext
	variable uint32
	value    numeral.Rational
	lower    bool
	open     bool
	refs     uint32
	closed   bool
}

type rationalUnitClause struct {
	atom  *RationalInequality
	axiom bool
}

// NewInequality owns the exact threshold before any unit or clause references
// are added. A stopped copy remains in context ownership for Close.
func (c *RationalContext) NewInequality(variable uint32, value *numeral.Rational, lower, open bool) (*RationalInequality, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if variable >= uint32(len(c.variables)) || value == nil {
		return nil, arena.ErrInvalidSlot
	}
	a := &RationalInequality{context: c, variable: variable, value: numeral.NewRational(), lower: lower, open: open}
	c.inequalities = append(c.inequalities, a)
	if err := c.numbers.Copy(&a.value, value); err != nil {
		return nil, err
	}
	return a, nil
}

func (c *RationalContext) AddUnitClause(a *RationalInequality, axiom bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c || a.closed || a.variable >= uint32(len(c.variables)) || a.refs == math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := c.traceUnitInequality(a); err != nil {
		return err
	}
	c.traceInequalityIncRef(a)
	a.refs++
	c.unitClauses = append(c.unitClauses, rationalUnitClause{atom: a, axiom: axiom})
	return nil
}

// AddInequality is the source add_ineq path for a single unit atom.
func (c *RationalContext) AddInequality(variable uint32, value *numeral.Rational, lower, open, axiom bool) (*RationalInequality, error) {
	a, err := c.NewInequality(variable, value, lower, open)
	if err != nil {
		return nil, err
	}
	if err := c.AddUnitClause(a, axiom); err != nil {
		return nil, err
	}
	return a, nil
}

// AssertRootUnits checkpoints before each unit and retains the completed
// prefix so cancellation resumes without replaying older bounds.
func (c *RationalContext) AssertRootUnits() (*RationalNode, error) {
	n, err := c.Root()
	if err != nil {
		return nil, err
	}
	for c.unitHead < len(c.unitClauses) && n.conflictVar == math.MaxUint32 {
		if err := sourceCheckpoint(c.limit, c.runtime, c.process, c.maxMemory, ErrRationalContextCanceled, c.cooperation); err != nil {
			return n, err
		}
		unit := c.unitClauses[c.unitHead]
		a := unit.atom
		if err := c.traceInitialUnit(a, unit.axiom); err != nil {
			return n, err
		}
		if err := c.publishBoundReason(n, a.variable, &a.value, a.lower, a.open, unit.axiom, nil, math.MaxUint32); err != nil {
			return n, err
		}
		c.queue = append(c.queue, n.trail)
		c.unitHead++
	}
	if !c.initUnitBoundsTraced {
		if err := c.traceInitialBounds(n, "bounds after init\n", 1785); err != nil {
			return n, err
		}
		c.initUnitBoundsTraced = true
	}
	return n, nil
}
