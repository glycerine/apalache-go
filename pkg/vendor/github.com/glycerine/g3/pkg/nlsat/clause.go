package nlsat

// Source: src/nlsat/nlsat_clause.cpp/.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

// Clause is a solver-owned clause. Its placement initializer copies literals
// into caller-owned fixed-capacity storage. Atom and assumption references are
// acquired by the solver factory, not by this source constructor.
type Clause struct {
	id, size, capacity, activity uint32
	learned                      bool
	assumptions                  *AssumptionSet
	literals                     []sat.Literal
}

func (c *Clause) initialize(id uint32, lits []sat.Literal, learned bool, as *AssumptionSet, storage []sat.Literal) error {
	if len(lits) != len(storage) || uint64(len(lits)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	*c = Clause{id: id, size: uint32(len(lits)), capacity: uint32(len(lits)) & 0x7fffffff, learned: learned, assumptions: as, literals: storage}
	copy(storage, lits)
	return nil
}
func clauseObjectSize(num uint32) uint64 {
	return uint64(reflect.TypeOf(Clause{}).Size()) + uint64(num)*uint64(reflect.TypeOf(sat.Literal(0)).Size())
}
func (c *Clause) storageBytes() uint64        { return clauseObjectSize(c.capacity) }
func (c *Clause) Size() uint32                { return c.size }
func (c *Clause) ID() uint32                  { return c.id }
func (c *Clause) IsLearned() bool             { return c.learned }
func (c *Clause) Assumptions() *AssumptionSet { return c.assumptions }
func (c *Clause) Literal(i uint32) (sat.Literal, error) {
	if err := atomAssert("nlsat_clause.h", 43, "idx < m_size", i < c.size); err != nil {
		return 0, err
	}
	if i >= c.size {
		return 0, arena.ErrBounds
	}
	return c.literals[i], nil
}
func (c *Clause) BorrowLiteral(i uint32) (*sat.Literal, error) {
	if err := atomAssert("nlsat_clause.h", 42, "idx < m_size", i < c.size); err != nil {
		return nil, err
	}
	if i >= c.size {
		return nil, arena.ErrBounds
	}
	return &c.literals[i], nil
}

// Literals borrows the active prefix. Shrink retains the original allocation
// capacity, including hidden trailing literals, for the solver's deallocation.
func (c *Clause) Literals() []sat.Literal { return c.literals[:c.size] }
func (c *Clause) IncActivity()            { c.activity++ }
func (c *Clause) SetActivity(v uint32)    { c.activity = v }
func (c *Clause) Activity() uint32        { return c.activity }
func (c *Clause) ContainsLiteral(l sat.Literal) bool {
	for _, current := range c.literals[:c.size] {
		if current == l {
			return true
		}
	}
	return false
}
func (c *Clause) ContainsVariable(v sat.BoolVar) bool {
	for _, current := range c.literals[:c.size] {
		if current.Variable() == v {
			return true
		}
	}
	return false
}
func (c *Clause) Shrink(n uint32) error {
	if err := atomAssert("nlsat_clause.h", 55, "num_lits <= m_size", n <= c.size); err != nil {
		return err
	}
	if n < c.size {
		c.size = n
	}
	return nil
}
