package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:408-514,528-589 and
// 1176-1315, concrete config_mpfx bound path.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrFixedContextTimestamp = errors.New("subpaving: bound timestamp exhausted")

// FixedContext owns the live bound-policy numerals and root node. The numeral
// manager and resource limit are borrowed from the caller.
type FixedContext struct {
	self                  *FixedContext
	numbers               *numeral.FixedPointManager
	limit                 *control.Reslimit
	cooperation           *cooperate.Cooperation
	isInt                 []bool
	root                  *FixedNode
	nodes                 []*FixedNode
	epsilon               *numeral.FixedPoint
	maxBound              *numeral.FixedPoint
	minusMaxBound         *numeral.FixedPoint
	rootPrecision         *numeral.FixedPoint
	zeroEpsilon           bool
	maxDepth              uint32
	maxNodes              uint32
	maxMemory             uint64
	arithmeticFailed      bool
	timestamp             uint64
	queue                 []*FixedBound
	queueHead             int
	numVisited            uint64
	numNodes              uint32
	nextNodeID            uint32
	freeNodeIDs           []uint32
	numBounds             uint64
	numConflicts          uint64
	polynomials           map[uint32]*FixedPolynomial
	monomials             map[uint32]*FixedMonomial
	watchers              [][]uint32
	watchOrder            [][]fixedWatch
	inequalities          []*FixedInequality
	clauses               []*FixedClause
	unitClauses           []fixedUnitClause
	unitHead              int
	leafQueue             []*FixedNode
	searchOwned           []*FixedNode
	searchNode            *FixedNode
	searchStarted         bool
	searchPhase           uint8
	numSplits             uint64
	displayVariable       FixedVariableDisplay
	trace                 *diagnostic.TraceTags
	initStage             uint8
	rootInitStarted       bool
	initDefHead           uint32
	initConstraintsTraced bool
	initUnitBoundsTraced  bool
	initRootBoundsTraced  bool
	closed                bool
}

// NewFixedContext uses the source parameter values directly. The source
// defaults are epsilon=20, maxBoundPower=10 and rootPrecision=8192.
func newFixedContextOwner(m *numeral.FixedPointManager, limit *control.Reslimit, isInt []bool) (_ *FixedContext, err error) {
	if m == nil || limit == nil || len(isInt) > math.MaxUint32 {
		return nil, arena.ErrInvalidSlot
	}
	c := &FixedContext{numbers: m, limit: limit, isInt: append([]bool(nil), isInt...), polynomials: make(map[uint32]*FixedPolynomial), monomials: make(map[uint32]*FixedMonomial), watchers: make([][]uint32, len(isInt)), watchOrder: make([][]fixedWatch, len(isInt))}
	c.self = c
	defer func() {
		if err != nil {
			err = errors.Join(err, c.closeNumerals())
		}
	}()
	if c.epsilon, err = m.New(); err != nil {
		return nil, err
	}
	if c.maxBound, err = m.New(); err != nil {
		return nil, err
	}
	if c.minusMaxBound, err = m.New(); err != nil {
		return nil, err
	}
	if c.rootPrecision, err = m.New(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *FixedContext) newRoot() error {
	root, err := NewFixedNode(c.numbers, uint32(len(c.isInt)), nil)
	if err != nil {
		return err
	}
	c.root = root
	root.context = c
	root.id = c.takeNodeID()
	c.nodes = []*FixedNode{root}
	c.numNodes = 1
	return nil
}

func (c *FixedContext) takeNodeID() uint32 {
	if n := len(c.freeNodeIDs); n != 0 {
		id := c.freeNodeIDs[n-1]
		c.freeNodeIDs = c.freeNodeIDs[:n-1]
		return id
	}
	id := c.nextNodeID
	c.nextNodeID++
	return id
}

func NewFixedContext(m *numeral.FixedPointManager, limit *control.Reslimit, isInt []bool, epsilon, maxBoundPower, rootPrecision uint32) (_ *FixedContext, err error) {
	c, err := newFixedContextOwner(m, limit, isInt)
	if err != nil {
		return nil, err
	}
	c.zeroEpsilon, c.maxDepth, c.maxNodes, c.maxMemory = epsilon == 0, 128, 8192, math.MaxUint64
	defer func() {
		if err != nil {
			err = errors.Join(err, c.closeNumerals())
		}
	}()
	if epsilon != 0 {
		if err = m.SetInt64(c.epsilon, int64(epsilon)); err != nil {
			return nil, err
		}
		if err = m.Inv(c.epsilon); err != nil {
			return nil, err
		}
	}
	if err = m.SetInt64(c.maxBound, 10); err != nil {
		return nil, err
	}
	if err = m.Power(c.maxBound, maxBoundPower, c.maxBound); err != nil {
		return nil, err
	}
	if err = m.Copy(c.minusMaxBound, c.maxBound); err != nil {
		return nil, err
	}
	if err = m.Neg(c.minusMaxBound); err != nil {
		return nil, err
	}
	if rootPrecision == 0 {
		rootPrecision = 1
	}
	if err = m.SetInt64(c.rootPrecision, int64(rootPrecision)); err != nil {
		return nil, err
	}
	if err = m.Inv(c.rootPrecision); err != nil {
		return nil, err
	}
	if err = c.newRoot(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *FixedContext) open() error {
	if c == nil || c.self != c || c.numbers == nil || c.root == nil {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

// NewVariable is the reached context_t<config_mpfx>::mk_var path before
// definition and bound initialization. The root's views grow with the source
// variable, definition and watch vectors.
func (c *FixedContext) NewVariable(integer bool) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if len(c.isInt) >= math.MaxUint32 || c.rootInitStarted || c.initStage != 0 || c.root.children != 0 || c.root.trail != nil {
		return 0, arena.ErrInvalidSlot
	}
	x := uint32(len(c.isInt))
	c.isInt = append(c.isInt, integer)
	c.watchers = append(c.watchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	c.root.lower = append(c.root.lower, nil)
	c.root.upper = append(c.root.upper, nil)
	return x, nil
}

func (c *FixedContext) closeNumerals() (err error) {
	for _, v := range []*numeral.FixedPoint{c.epsilon, c.maxBound, c.minusMaxBound, c.rootPrecision} {
		if v != nil {
			err = errors.Join(err, v.Close())
		}
	}
	return err
}

func (c *FixedContext) Root() (*FixedNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.root, nil
}

func (c *FixedContext) NewChild(parent *FixedNode) (*FixedNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if parent == nil || parent.context != c {
		return nil, arena.ErrInvalidSlot
	}
	return NewFixedNode(c.numbers, uint32(len(c.isInt)), parent)
}

func (c *FixedContext) XnEqY(out, src *FixedInterval, degree uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return out.XnEqY(src, degree, c.rootPrecision, interval.FixedRoot{Numbers: c.numbers, Limit: c.limit, Cooperation: c.cooperation})
}

func (c *FixedContext) Close() error {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return nil
	}
	for len(c.searchOwned) != 0 {
		i := len(c.searchOwned) - 1
		if err := c.searchOwned[i].Close(); err != nil {
			return err
		}
		c.searchOwned[i] = nil
		c.searchOwned = c.searchOwned[:i]
	}
	if c.root != nil {
		if err := c.root.Close(); err != nil {
			return err
		}
	}
	for i := range c.unitClauses {
		if err := c.releaseUnit(i); err != nil {
			return err
		}
	}
	for _, lemma := range []bool{false, true} {
		for _, clause := range c.clauses {
			if clause.lemma == lemma {
				if err := c.releaseClause(clause); err != nil {
					return err
				}
			}
		}
	}
	for x := range c.isInt {
		if p := c.polynomials[uint32(x)]; p != nil {
			if err := p.close(); err != nil {
				return err
			}
		}
	}
	for i := len(c.inequalities) - 1; i >= 0; i-- {
		if err := c.inequalities[i].value.Close(); err != nil {
			return err
		}
	}
	if err := c.closeNumerals(); err != nil {
		return err
	}
	c.queue = nil
	c.queueHead = 0
	c.leafQueue = nil
	c.nodes = nil
	c.searchNode = nil
	c.displayVariable = nil
	c.trace = nil
	c.closed = true
	return nil
}
