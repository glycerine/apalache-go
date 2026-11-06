package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:291-342,525-569, concrete config_mpq.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalContext owns the reached config_mpq variables, bounds, definitions,
// clauses and breadth-first search state.
type RationalContext struct {
	self             *RationalContext
	numbers          *numeral.RationalManager
	limit            *control.Reslimit
	cooperation      *cooperate.Cooperation
	displayVariable  RationalVariableDisplay
	trace            *diagnostic.TraceTags
	runtime          *memory.Runtime
	process          *memory.ProcessAccounting
	maxMemory        uint64
	epsilon          numeral.Rational
	maxBound         numeral.Rational
	minusMaxBound    numeral.Rational
	rootPrecision    numeral.Rational
	relevanceScratch [3]numeral.Rational
	zeroEpsilon      bool
	variables        []bool
	inequalities     []*RationalInequality
	unitClauses      []rationalUnitClause
	unitHead         int
	initStage        uint8
	rootInitStarted  bool

	initConstraintsTraced, initUnitBoundsTraced, initRootBoundsTraced bool

	clauses          []*RationalClause
	clauseWatchers   [][]*RationalClause
	watchOrder       [][]rationalWatch
	polynomials      map[uint32]*RationalPolynomial
	polynomialOwners []*RationalPolynomial
	monomials        map[uint32]*RationalMonomial
	numBuffer        []numeral.Rational
	initDefHead      uint32
	numVisited       uint64
	queue            []*RationalBound
	queueHead        int
	root             *RationalNode
	nodes            []*RationalNode
	leafHead         *RationalNode
	leafTail         *RationalNode
	numBounds        uint64
	numConflicts     uint64
	numSplits        uint64
	timestamp        uint64
	maxDepth         uint32
	maxNodes         uint32
	searchNode       *RationalNode
	searchPhase      uint8
	closed           bool
}

func NewRationalContext(m *numeral.RationalManager, limit *control.Reslimit) (_ *RationalContext, err error) {
	c, err := newRationalContextOwner(m, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.closePolicyNumerals())
		}
	}()
	if err = c.UpdateBoundPolicy(20, 10); err != nil {
		return nil, err
	}
	if err = c.UpdateRootPrecision(8192); err != nil {
		return nil, err
	}
	c.maxDepth, c.maxNodes, c.maxMemory = 128, 8192, math.MaxUint64
	return c, nil
}

func newRationalContextOwner(m *numeral.RationalManager, limit *control.Reslimit) (*RationalContext, error) {
	if m == nil || limit == nil {
		return nil, arena.ErrInvalidSlot
	}
	c := &RationalContext{numbers: m, limit: limit, runtime: m.RuntimeOwner(),
		epsilon: numeral.NewRational(), maxBound: numeral.NewRational(), minusMaxBound: numeral.NewRational(), rootPrecision: numeral.NewRational()}
	for i := range c.relevanceScratch {
		c.relevanceScratch[i] = numeral.NewRational()
	}
	c.self = c
	if c.runtime != nil {
		c.process = c.runtime.ProcessOwner()
	}
	return c, nil
}

func (c *RationalContext) closePolicyNumerals() (err error) {
	for i := range c.relevanceScratch {
		err = errors.Join(err, c.numbers.Del(&c.relevanceScratch[i]))
	}
	return errors.Join(err, c.numbers.Del(&c.rootPrecision), c.numbers.Del(&c.minusMaxBound), c.numbers.Del(&c.maxBound), c.numbers.Del(&c.epsilon))
}

func (c *RationalContext) open() error {
	if c == nil || c.self != c || c.numbers == nil || c.limit == nil {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func (c *RationalContext) NewVariable(integer bool) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || len(c.variables) >= 1<<29 {
		return 0, arena.ErrInvalidSlot
	}
	x := uint32(len(c.variables))
	c.variables = append(c.variables, integer)
	c.clauseWatchers = append(c.clauseWatchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	return x, nil
}

func (c *RationalContext) Root() (*RationalNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.root != nil {
		if err := c.root.open(); err != nil {
			return nil, err
		}
		return c.root, nil
	}
	n, err := NewRationalNode(c.numbers, uint32(len(c.variables)), nil)
	if err != nil {
		return nil, err
	}
	n.context, n.registered, n.id = c, true, uint32(len(c.nodes))
	c.root = n
	c.nodes = append(c.nodes, n)
	c.pushLeafFront(n)
	return n, nil
}

func (c *RationalContext) NewChild(parent *RationalNode) (*RationalNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.root == nil || parent == nil || parent.context != c || !parent.registered {
		return nil, arena.ErrInvalidSlot
	}
	n, err := NewRationalNode(c.numbers, uint32(len(c.variables)), parent)
	if err != nil {
		return nil, err
	}
	n.registered, n.id = true, uint32(len(c.nodes))
	c.nodes = append(c.nodes, n)
	c.pushLeafFront(n)
	return n, nil
}

// PublishBound follows mk_bound's exact rational integer normalization before
// publishing the endpoint to this node's copied bound view.
func (c *RationalContext) PublishBound(n *RationalNode, variable uint32, value *numeral.Rational, lower, open bool) (err error) {
	return c.publishBoundReason(n, variable, value, lower, open, false, nil, math.MaxUint32)
}

func (c *RationalContext) publishBoundReason(n *RationalNode, variable uint32, value *numeral.Rational, lower, open, axiom bool, reason *RationalClause, definition uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered || variable >= uint32(len(c.variables)) || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if _, _, err = n.Bounds(variable); err != nil {
		return err
	}
	if value == nil {
		return arena.ErrInvalidSlot
	}
	if c.timestamp == math.MaxUint64 {
		return ErrRationalContextTimestamp
	}
	c.numBounds++
	if !c.variables[variable] {
		return c.publishNormalized(n, variable, value, lower, open, axiom, reason, definition)
	}
	adjusted := numeral.NewRational()
	defer func() { err = errors.Join(err, c.numbers.Del(&adjusted)) }()
	integer, err := c.numbers.IsInt(value)
	if err != nil {
		return err
	}
	if !integer {
		open = false
	}
	if lower {
		err = c.numbers.Ceil(value, &adjusted)
	} else {
		err = c.numbers.Floor(value, &adjusted)
	}
	if err != nil {
		return err
	}
	if open {
		open = false
		if lower {
			err = c.numbers.Inc(&adjusted)
		} else {
			err = c.numbers.Dec(&adjusted)
		}
		if err != nil {
			return err
		}
	}
	return c.publishNormalized(n, variable, &adjusted, lower, open, axiom, reason, definition)
}

func (c *RationalContext) publishNormalized(n *RationalNode, variable uint32, value *numeral.Rational, lower, open, axiom bool, reason *RationalClause, definition uint32) error {
	if err := n.pushBoundAt(variable, value, lower, open, c.timestamp, axiom); err != nil {
		return err
	}
	n.trail.reasonClause = reason
	n.trail.reasonDefinition = definition
	if err := c.traceNewBound(n.trail); err != nil {
		return err
	}
	conflict, err := n.ConflictingBounds(variable)
	if err != nil {
		return err
	}
	if conflict {
		if err := c.traceBoundConflict(n); err != nil {
			return err
		}
		n.conflictVar = variable
		c.numConflicts++
		c.removeLeaf(n)
	}
	c.timestamp++
	if c.timestamp == math.MaxUint64 {
		return ErrRationalContextTimestamp
	}
	return nil
}

func (c *RationalContext) Close() error {
	if err := c.open(); err != nil {
		if c != nil && c.self == c && c.closed {
			return nil
		}
		return err
	}
	for i := len(c.nodes) - 1; i >= 0; i-- {
		if err := c.nodes[i].Close(); err != nil {
			return err
		}
	}
	for len(c.unitClauses) != 0 {
		a := c.unitClauses[0].atom
		if a.refs == 0 {
			return arena.ErrInvalidSlot
		}
		if err := c.traceInequalityDecRef(a); err != nil {
			return err
		}
		a.refs--
		c.unitClauses = c.unitClauses[1:]
	}
	for len(c.clauses) != 0 {
		clause := c.clauses[0]
		for clause.releaseAt < len(clause.atoms) {
			a := clause.atoms[clause.releaseAt]
			if a.refs == 0 {
				return arena.ErrInvalidSlot
			}
			if err := c.traceInequalityDecRef(a); err != nil {
				return err
			}
			a.refs--
			clause.releaseAt++
		}
		c.clauses[0] = nil
		c.clauses = c.clauses[1:]
	}
	for len(c.polynomialOwners) != 0 {
		last := len(c.polynomialOwners) - 1
		if err := c.polynomialOwners[last].Close(); err != nil {
			return err
		}
		c.polynomialOwners[last] = nil
		c.polynomialOwners = c.polynomialOwners[:last]
	}
	for i := range c.numBuffer {
		if err := c.numbers.Del(&c.numBuffer[i]); err != nil {
			return err
		}
	}
	for i := len(c.inequalities) - 1; i >= 0; i-- {
		if !c.inequalities[i].closed {
			if err := c.numbers.Del(&c.inequalities[i].value); err != nil {
				return err
			}
			c.inequalities[i].closed = true
		}
	}
	if err := c.closePolicyNumerals(); err != nil {
		return err
	}
	c.nodes, c.root, c.variables = nil, nil, nil
	c.leafHead, c.leafTail = nil, nil
	c.inequalities, c.unitClauses, c.clauses, c.clauseWatchers = nil, nil, nil, nil
	c.watchOrder, c.polynomials, c.polynomialOwners, c.numBuffer, c.monomials = nil, nil, nil, nil, nil
	c.queue, c.queueHead = nil, 0
	c.searchNode = nil
	c.cooperation = nil
	c.displayVariable = nil
	c.trace = nil
	c.closed = true
	return nil
}
