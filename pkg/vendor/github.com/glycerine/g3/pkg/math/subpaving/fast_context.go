package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:408-514,528-589 and the concrete
// context_t<config_mpff> instantiation in src/math/subpaving/subpaving_mpff.cpp:21.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrFastContextTimestamp error = &sourceFailure{message: "subpaving: fast-float bound timestamp exhausted"}

// FastContext owns its bound-policy numerals, root node and search-created
// children. The numeral manager and resource limit remain caller-owned.
type FastContext struct {
	self                  *FastContext
	numbers               *numeral.FastFloatManager
	limit                 *control.Reslimit
	cooperation           *cooperate.Cooperation
	runtime               *memory.Runtime
	process               *memory.ProcessAccounting
	isInt                 []bool
	root                  *FastNode
	nodes                 []*FastNode
	leafQueue             []*FastNode
	searchOwned           []*FastNode
	searchNode            *FastNode
	searchPhase           uint8
	numSplits             uint64
	numBounds             uint64
	numConflicts          uint64
	owned                 [4]*numeral.ScopedFastFloat
	epsilon               *numeral.FastFloat
	maxBound              *numeral.FastFloat
	minusMaxBound         *numeral.FastFloat
	rootPrecision         *numeral.FastFloat
	zeroEpsilon           bool
	maxDepth              uint32
	maxNodes              uint32
	maxMemory             uint64
	arithmeticFailed      bool
	timestamp             uint64
	queue                 []*FastBound
	queueHead             int
	numVisited            uint64
	polynomials           map[uint32]*FastPolynomial
	monomials             map[uint32]*FastMonomial
	watchers              [][]uint32
	watchOrder            [][]fastWatch
	inequalities          []*FastInequality
	clauses               []*FastClause
	unitClauses           []fastUnitClause
	unitHead              int
	displayVariable       FastVariableDisplay
	trace                 *diagnostic.TraceTags
	initStage             uint8
	rootInitStarted       bool
	initDefHead           uint32
	initConstraintsTraced bool
	initUnitBoundsTraced  bool
	initRootBoundsTraced  bool
	closed                bool
}

// NewFastContext accepts the source's epsilon, max_bound exponent and
// nth_root_precision parameter values. The caller owns the manager and limit.
func NewFastContext(m *numeral.FastFloatManager, limit *control.Reslimit, isInt []bool, epsilon, maxBoundPower, rootPrecision uint32) (_ *FastContext, err error) {
	c, err := newFastContextOwner(m, limit, isInt)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	c.maxDepth, c.maxNodes, c.maxMemory = 128, 8192, math.MaxUint64
	if err = c.updateEpsilon(epsilon); err != nil {
		return nil, err
	}
	if err = c.updateMaxBound(maxBoundPower); err != nil {
		return nil, err
	}
	if err = c.updateRootPrecision(rootPrecision); err != nil {
		return nil, err
	}
	return c, nil
}

func newFastContextOwner(m *numeral.FastFloatManager, limit *control.Reslimit, isInt []bool) (_ *FastContext, err error) {
	if m == nil || limit == nil || len(isInt) > math.MaxUint32 {
		return nil, arena.ErrInvalidSlot
	}
	c := &FastContext{numbers: m, limit: limit, isInt: append([]bool(nil), isInt...), polynomials: make(map[uint32]*FastPolynomial), monomials: make(map[uint32]*FastMonomial), watchers: make([][]uint32, len(isInt)), watchOrder: make([][]fastWatch, len(isInt))}
	c.self = c
	if owner := m.RuntimeOwner(); owner != nil {
		c.runtime = owner
		c.process = owner.ProcessOwner()
	}
	defer func() {
		if err != nil {
			if c.root != nil {
				err = errors.Join(err, c.root.Close())
			}
			err = errors.Join(err, c.closeNumerals())
		}
	}()
	for i := range c.owned {
		c.owned[i], err = numeral.NewScopedFastFloat(m)
		if err != nil {
			return nil, err
		}
	}
	if c.epsilon, err = c.owned[0].Get(); err != nil {
		return nil, err
	}
	if c.maxBound, err = c.owned[1].Get(); err != nil {
		return nil, err
	}
	if c.minusMaxBound, err = c.owned[2].Get(); err != nil {
		return nil, err
	}
	if c.rootPrecision, err = c.owned[3].Get(); err != nil {
		return nil, err
	}
	return c, nil
}

// ensureRoot models context_t::mk_node(nullptr), which C++ calls from init
// after construction and parameter updates. Both Go constructors leave the
// root absent until Root or InitializeRoot is requested.
func (c *FastContext) ensureRoot() error {
	if c.root != nil {
		return nil
	}
	root, err := NewFastNode(c.numbers, uint32(len(c.isInt)), nil)
	if err != nil {
		return err
	}
	root.context = c
	root.id = 0
	root.queuedLeaf = true
	c.root = root
	c.nodes = []*FastNode{root}
	c.leafQueue = []*FastNode{root}
	return nil
}

func (c *FastContext) updateEpsilon(v uint32) error {
	if v == 0 {
		if err := c.numbers.Reset(c.epsilon); err != nil {
			return err
		}
		c.zeroEpsilon = true
		return nil
	}
	if err := c.numbers.SetInt32(c.epsilon, int32(v)); err != nil {
		return err
	}
	if err := c.numbers.InvInPlace(c.epsilon); err != nil {
		return err
	}
	c.zeroEpsilon = false
	return nil
}

func (c *FastContext) updateMaxBound(v uint32) error {
	if err := c.numbers.SetInt32(c.maxBound, 10); err != nil {
		return err
	}
	if err := c.numbers.Power(c.maxBound, v, c.maxBound); err != nil {
		return err
	}
	if err := c.numbers.Copy(c.minusMaxBound, c.maxBound); err != nil {
		return err
	}
	return c.numbers.Neg(c.minusMaxBound)
}

func (c *FastContext) updateRootPrecision(v uint32) error {
	if v == 0 {
		v = 1
	}
	if err := c.numbers.SetInt32(c.rootPrecision, int32(v)); err != nil {
		return err
	}
	return c.numbers.InvInPlace(c.rootPrecision)
}

func (c *FastContext) open() error {
	if c == nil || c.self != c || c.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

// NewVariable follows context_t<config_mpff>::mk_var before root
// initialization. The root remains absent while source variable and watch
// arrays are populated.
func (c *FastContext) NewVariable(integer bool) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if len(c.isInt) >= math.MaxUint32 || c.rootInitStarted || c.initStage != 0 || (c.root != nil && (c.root.children != 0 || c.root.trail != nil)) {
		return 0, arena.ErrInvalidSlot
	}
	x := uint32(len(c.isInt))
	c.isInt = append(c.isInt, integer)
	c.watchers = append(c.watchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	if c.root != nil {
		c.root.lower = append(c.root.lower, nil)
		c.root.upper = append(c.root.upper, nil)
	}
	return x, nil
}

func (c *FastContext) closeNumerals() (err error) {
	for _, owner := range c.owned {
		if owner != nil {
			err = errors.Join(err, owner.Close())
		}
	}
	return err
}

func (c *FastContext) Root() (*FastNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := c.ensureRoot(); err != nil {
		return nil, err
	}
	return c.root, nil
}

func (c *FastContext) NewChild(parent *FastNode) (*FastNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if parent == nil || parent.context != c || parent.depth == math.MaxUint32 || len(c.nodes) >= math.MaxUint32 {
		return nil, arena.ErrInvalidSlot
	}
	child, err := NewFastNode(c.numbers, uint32(len(c.isInt)), parent)
	if err != nil {
		return nil, err
	}
	child.id = uint32(len(c.nodes))
	c.nodes = append(c.nodes, child)
	c.leafQueue = append(c.leafQueue, child)
	child.queuedLeaf = true
	return child, nil
}

func (c *FastContext) XnEqY(out, src *FastInterval, degree uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return out.XnEqY(src, degree, c.rootPrecision, interval.FastRoot{Numbers: c.numbers, Limit: c.limit, Cooperation: c.cooperation})
}

func (c *FastContext) Close() error {
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
		if err := c.inequalities[i].owner.Close(); err != nil {
			return err
		}
	}
	if err := c.closeNumerals(); err != nil {
		return err
	}
	c.queue = nil
	c.queueHead = 0
	c.nodes = nil
	c.leafQueue = nil
	c.searchNode = nil
	c.displayVariable = nil
	c.trace = nil
	c.closed = true
	return nil
}
