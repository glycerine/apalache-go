package ast

// Source: src/test/sorting_network.cpp:150-181.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// PsortASTContext is the retained ast_ext2 specialization. Its manager is
// borrowed; clauses and trail own one reference per appended handle.
type PsortASTContext struct {
	self              *PsortASTContext
	manager           *ASTManager
	budget            *arena.Budget
	header            uint64
	clauses, trail    []ASTHandle
	fresh             []ASTHandle // borrowed from trail ownership
	freshPrefix       string
	closed, executing bool
	stage             uint8
	destructorCause   error
}

func NewPsortASTContext(m *ASTManager) (*PsortASTContext, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	b := m.BudgetField
	bytes := uint64(reflect.TypeOf(PsortASTContext{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	c := &PsortASTContext{manager: m, budget: b, header: bytes, freshPrefix: "x"}
	c.self = c
	return c, nil
}

// SetFreshPrefix selects the source prefix used by a retained concrete
// specialization before it emits any auxiliary variable.
func (c *PsortASTContext) SetFreshPrefix(prefix string) error {
	if e := c.open(); e != nil {
		return e
	}
	if len(c.fresh) != 0 {
		return ErrASTPending
	}
	c.freshPrefix = prefix
	return nil
}

// FreshCount and FreshApplication expose borrowed fresh applications while
// trail retains their owning references.
func (c *PsortASTContext) FreshCount() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return uint32(len(c.fresh)), nil
}

func (c *PsortASTContext) FreshApplication(index uint32) (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	if uint64(index) >= uint64(len(c.fresh)) {
		return ASTHandle{}, arena.ErrBounds
	}
	return c.fresh[index], nil
}

// TakeClauses transfers the context's owned clause references to the caller.
// The caller must eventually release each handle with ASTManager.DecRef.
func (c *PsortASTContext) TakeClauses() ([]ASTHandle, error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	result := c.clauses
	c.clauses = nil
	return result, nil
}

func (c *PsortASTContext) ClauseCount() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return uint32(len(c.clauses)), nil
}
func (c *PsortASTContext) identity() error {
	if c == nil || c.self != c {
		return ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	if c.executing {
		return ErrASTPending
	}
	return nil
}
func (c *PsortASTContext) open() error {
	if e := c.identity(); e != nil {
		return e
	}
	if c.destructorCause != nil || c.stage != 0 {
		return ErrASTPending
	}
	return nil
}
func (c *PsortASTContext) push(v *[]ASTHandle, h ASTHandle) error {
	if e := c.manager.IncRef(h); e != nil {
		return e
	}
	*v = append(*v, h)
	return nil
}
func (c *PsortASTContext) False() (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	return c.manager.MkFalse()
}
func (c *PsortASTContext) True() (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	return c.manager.MkTrue()
}
func (c *PsortASTContext) Fresh() (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	c.executing = true
	defer func() { c.executing = false }()
	sort, e := c.manager.MkBoolSort()
	if e != nil {
		return ASTHandle{}, e
	}
	h, e := c.manager.MkFreshConst(c.freshPrefix, sort)
	if e != nil {
		return h, e
	}
	if e = c.push(&c.trail, h); e != nil {
		return h, e
	}
	c.fresh = append(c.fresh, h)
	return h, nil
}
func (c *PsortASTContext) Min(a, b ASTHandle) (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	c.executing = true
	defer func() { c.executing = false }()
	h, e := c.manager.MkAnd(a, b)
	if e != nil {
		return h, e
	}
	return h, c.push(&c.trail, h)
}
func (c *PsortASTContext) Max(a, b ASTHandle) (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	c.executing = true
	defer func() { c.executing = false }()
	h, e := c.manager.MkOr(a, b)
	if e != nil {
		return h, e
	}
	return h, c.push(&c.trail, h)
}
func (c *PsortASTContext) Not(a ASTHandle) (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	c.executing = true
	defer func() { c.executing = false }()
	yes, child, e := c.manager.IsNot(a)
	if e != nil {
		return ASTHandle{}, e
	}
	if yes {
		return child, nil
	}
	h, e := c.manager.MkNot(a)
	if e != nil {
		return h, e
	}
	return h, c.push(&c.trail, h)
}
func (c *PsortASTContext) Clause(n uint32, span ASTLiteralVectorSpan) error {
	if e := c.open(); e != nil {
		return e
	}
	c.executing = true
	defer func() { c.executing = false }()
	h, e := c.manager.MkOrSpan(n, span)
	if e != nil {
		return e
	}
	return c.push(&c.clauses, h)
}
func (c *PsortASTContext) finalizeMembers() error {
	m := c.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.stage == 0 {
		clear(c.fresh)
		c.fresh = nil
	}
	// Reverse member order, ascending references within each member.
	members := [2]*[]ASTHandle{&c.trail, &c.clauses}
	for c.stage < 2 {
		v := members[c.stage]
		if !m.Closed {
			for i, h := range *v {
				if h.IsNull() {
					continue
				}
				r, e := m.internal(h)
				if e != nil {
					return e
				}
				consumed, e := m.decNode(r)
				if consumed {
					(*v)[i] = ASTHandle{}
				}
				if e != nil {
					return e
				}
				if !consumed {
					return ErrASTPending
				}
			}
		}
		*v = nil
		c.stage++
	}
	return nil
}
func (c *PsortASTContext) SourceFinalize() error {
	if e := c.identity(); e != nil {
		return e
	}
	if c.destructorCause != nil {
		return c.destructorCause
	}
	c.executing = true
	defer func() { c.executing = false }()
	if e := c.finalizeMembers(); e != nil {
		c.destructorCause = e
		return e
	}
	return nil
}
func (c *PsortASTContext) Close() error {
	if c != nil && c.self == c && c.closed {
		return nil
	}
	if e := c.identity(); e != nil {
		return e
	}
	c.executing = true
	defer func() { c.executing = false }()
	// Owning cleanup resumes consumed references; source teardown retains its cause.
	if e := c.finalizeMembers(); e != nil {
		if c.destructorCause == nil {
			c.destructorCause = e
		}
		return e
	}
	c.budget.Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
