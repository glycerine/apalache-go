package subpaving

// Source: src/math/subpaving/subpaving_hwf.h:25-44 and
// src/math/subpaving/subpaving_t_def.h:408-514,1641, concrete config_hwf.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// HardwareContext owns the scalar bound-policy numerals and concrete
// constraint and node records of context_hwf. Its manager and resource limit
// are borrowed. The source process-memory checkpoint and tactic owner remain.
type HardwareContext struct {
	self                  *HardwareContext
	numbers               *numeral.HardwareFloatNumeralManager
	limit                 *control.Reslimit
	cooperation           *cooperate.Cooperation
	process               *memory.ProcessAccounting
	epsilon               numeral.HardwareFloat
	maxBound              numeral.HardwareFloat
	minusMaxBound         numeral.HardwareFloat
	rootPrecision         numeral.HardwareFloat
	zeroEpsilon           bool
	arithmeticFailed      bool
	maxDepth              uint32
	maxNodes              uint32
	maxMemory             uint64
	isInt                 []bool
	polynomials           map[uint32]*HardwarePolynomial
	monomials             map[uint32]*HardwareMonomial
	watchers              [][]uint32
	watchOrder            [][]hardwareWatch
	inequalities          []*HardwareInequality
	clauses               []*HardwareClause
	root                  *HardwareNode
	nodes                 []*HardwareNode
	activeNodes           uint32
	leafHead              *HardwareNode
	leafTail              *HardwareNode
	nextNodeID            uint32
	timestamp             uint64
	queue                 []*HardwareBound
	queueHead             int
	unitClauses           []hardwareUnitClause
	unitHead              int
	numVisited            uint64
	numSplits             uint64
	numBounds             uint64
	numConflicts          uint64
	searchNode            *HardwareNode
	searchPhase           uint8
	displayVariable       HardwareVariableDisplay
	trace                 *diagnostic.TraceTags
	initStage             uint8
	rootInitStarted       bool
	initDefHead           uint32
	initConstraintsTraced bool
	initUnitBoundsTraced  bool
	initRootBoundsTraced  bool
	closed                bool
}

// BindProcessAccounting supplies the source global memory counter for the
// hardware-float context, whose pointer-free numeral manager has no Runtime.
func (c *HardwareContext) BindProcessAccounting(p *memory.ProcessAccounting) error {
	if err := c.open(); err != nil {
		return err
	}
	if !p.Valid() {
		return arena.ErrConfig
	}
	c.process = p
	return nil
}

// ResetStatistics follows context_t::reset_statistics; the live node count is
// intentionally retained because source reset_statistics does not clear it.
func (c *HardwareContext) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	c.numConflicts = 0
	c.numBounds = 0
	c.numSplits = 0
	c.numVisited = 0
	return nil
}

// CollectStatistics appends the source's five counters in source order.
func (c *HardwareContext) CollectStatistics(st *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if st == nil {
		return arena.ErrInvalidSlot
	}
	st.UpdateUint64("conflicts", c.numConflicts)
	st.UpdateUint64("new bounds", c.numBounds)
	st.UpdateUint64("splits", c.numSplits)
	st.UpdateUint("nodes", c.activeNodes)
	st.UpdateUint64("visited", c.numVisited)
	return nil
}

// HardwareContextPolicy contains the unsigned source subpaving parameters.
type HardwareContextPolicy struct {
	Epsilon, MaxBoundPower, RootPrecision uint32
	MaxDepth, MaxNodes, MaxMemoryMB       uint32
}

// NewHardwareContext applies the source epsilon, max_bound and
// nth_root_precision unsigned parameter values.
func NewHardwareContext(m *numeral.HardwareFloatNumeralManager, limit *control.Reslimit, epsilon, maxBoundPower, rootPrecision uint32) (*HardwareContext, error) {
	if m == nil || limit == nil {
		return nil, arena.ErrInvalidSlot
	}
	c := &HardwareContext{numbers: m, limit: limit}
	c.self = c
	if err := c.UpdateParams(HardwareContextPolicy{Epsilon: epsilon, MaxBoundPower: maxBoundPower, RootPrecision: rootPrecision, MaxDepth: 128, MaxNodes: 8192, MaxMemoryMB: math.MaxUint32}); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateParams preserves the source's field mutation order. A failed numeral
// operation leaves the already reached prefix visible to the owning context.
func (c *HardwareContext) UpdateParams(p HardwareContextPolicy) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.updateEpsilon(p.Epsilon); err != nil {
		return err
	}
	if err := c.updateMaxBound(p.MaxBoundPower); err != nil {
		return err
	}
	c.maxDepth = p.MaxDepth
	c.maxNodes = p.MaxNodes
	c.updateMaxMemory(p.MaxMemoryMB)
	return c.updateRootPrecision(p.RootPrecision)
}

func (c *HardwareContext) updateEpsilon(epsilon uint32) error {
	m := c.numbers
	if epsilon != 0 {
		if err := m.SetInt32(&c.epsilon, int32(epsilon)); err != nil {
			return err
		}
		if err := m.Inv(&c.epsilon); err != nil {
			return err
		}
		c.zeroEpsilon = false
	} else {
		if err := m.Reset(&c.epsilon); err != nil {
			return err
		}
		c.zeroEpsilon = true
	}
	return nil
}

func (c *HardwareContext) updateMaxBound(power uint32) error {
	m := c.numbers
	if err := m.SetInt32(&c.maxBound, 10); err != nil {
		return err
	}
	if err := m.Power(&c.maxBound, power, &c.maxBound); err != nil {
		return err
	}
	if err := m.Set(&c.minusMaxBound, &c.maxBound); err != nil {
		return err
	}
	if err := m.Neg(&c.minusMaxBound, &c.minusMaxBound); err != nil {
		return err
	}
	return nil
}

func (c *HardwareContext) updateMaxMemory(megabytes uint32) {
	c.maxMemory = memory.MegabytesToBytes(megabytes)
}

func (c *HardwareContext) updateRootPrecision(precision uint32) error {
	if precision == 0 {
		precision = 1
	}
	if err := c.numbers.SetInt32(&c.rootPrecision, int32(precision)); err != nil {
		return err
	}
	return c.numbers.Inv(&c.rootPrecision)
}

func (c *HardwareContext) open() error {
	if c == nil || c.self != c || c.numbers == nil || c.limit == nil {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func (c *HardwareContext) XnEqY(out, src *HardwareInterval, degree uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return out.XnEqY(src, degree, &c.rootPrecision, interval.HardwareRoot{Numbers: c.numbers, Limit: c.limit, Cooperation: c.cooperation})
}

func (c *HardwareContext) Close() error {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return nil
	}
	for i := len(c.nodes) - 1; i >= 0; i-- {
		if err := c.nodes[i].Close(); err != nil {
			return err
		}
	}
	c.queue = nil
	c.searchNode = nil
	for len(c.unitClauses) != 0 {
		atom := c.unitClauses[0].atom
		if atom.refs == 0 {
			return arena.ErrInvalidSlot
		}
		if err := c.traceInequalityDecRef(atom); err != nil {
			return err
		}
		atom.refs--
		c.unitClauses = c.unitClauses[1:]
	}
	for len(c.clauses) != 0 {
		clause := c.clauses[0]
		for clause.releaseAt < len(clause.atoms) {
			atom := clause.atoms[clause.releaseAt]
			if atom.refs == 0 {
				return arena.ErrInvalidSlot
			}
			if err := c.traceInequalityDecRef(atom); err != nil {
				return err
			}
			atom.refs--
			clause.releaseAt++
		}
		clause.atoms = nil
		clause.deleted = true
		c.clauses[0] = nil
		c.clauses = c.clauses[1:]
	}
	c.nodes = nil
	c.unitClauses = nil
	c.clauses = nil
	c.inequalities = nil
	c.watchers = nil
	c.watchOrder = nil
	c.polynomials = nil
	c.monomials = nil
	c.displayVariable = nil
	c.trace = nil
	c.isInt = nil
	c.leafHead, c.leafTail = nil, nil
	c.closed = true
	return nil
}
