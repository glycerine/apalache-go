package aig

// Source: src/tactic/aig/aig.cpp:24-207,1302-1339,1656-1681.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
)

var ErrOwner = errors.New("aig: invalid or closed owner")
var ErrLiveReferences = errors.New("aig: references remain live")

const firstNodeID = math.MaxUint32 / 2

// Literal uses a typed node and inversion bit in place of the source's tagged
// pointer. Node identity and the inversion bit together form the table key.
type Literal struct {
	node     *node
	inverted bool
}

type andKey struct{ left, right Literal }

type node struct {
	id       uint32
	refs     uint32
	children [2]Literal
	variable ast.ASTRef
	and      bool
	stage    uint8 // owning deletion cursor after a stopped AST reference release
	traced   bool  // deletion TRACE runs once before a stopped owner retries
}

type idPool struct {
	next uint32
	free []uint32
}

func (p *idPool) take() (uint32, error) {
	if n := len(p.free); n != 0 {
		id := p.free[n-1]
		p.free = p.free[:n-1]
		return id, nil
	}
	if p.next == math.MaxUint32 {
		return 0, arena.ErrBounds
	}
	id := p.next
	p.next++
	return id, nil
}
func (p *idPool) recycle(id uint32) { p.free = append(p.free, id) }

// Manager owns the concrete AIG table, references, conversions and source
// checkpoints used by the goal-rewriting tactic.
type Manager struct {
	self *Manager
	ast  *ast.ASTManager
	*managerImp
	header          uint64
	closed          bool
	tacticTraceLive bool
}

// managerImp is the separately allocated source aig_manager::imp owner. Its
// fields are promoted to Manager so the concrete gate methods keep one owner.
type managerImp struct {
	table               map[andKey]*node
	variables           idPool
	andNodes            idPool
	trueNode            *node
	toDelete            []*node
	pendingAST          []ast.ASTRef
	nodes               uint32
	activeRefs          uint32
	impHeader           uint64
	nodeBytes           uint64
	closing             bool
	defaultGateEncoding bool
	maxMemory           uint64
	runtime             *memory.Runtime
	cooperation         *cooperate.Cooperation
	impTraceLive        bool
}

// Ref owns one source aig_ref count. Call Close for each returned reference.
type Ref struct {
	manager *Manager
	lit     Literal
	closed  bool
}

func NewManager(m *ast.ASTManager) (*Manager, error) {
	return NewManagerWithGateEncoding(m, true)
}

// NewManagerWithGateEncoding selects the source's default or alternate ITE
// and IFF circuit construction branch.
func NewManagerWithGateEncoding(m *ast.ASTManager, defaultGateEncoding bool) (*Manager, error) {
	return newManagerWithGateEncoding(m, defaultGateEncoding, false)
}

func newManagerWithGateEncoding(m *ast.ASTManager, defaultGateEncoding, tacticOwner bool) (*Manager, error) {
	if m == nil {
		return nil, ErrOwner
	}
	numerals := m.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return nil, ErrOwner
	}
	managerBytes := uint64(reflect.TypeOf(Manager{}).Size())
	impBytes := uint64(reflect.TypeOf(managerImp{}).Size())
	nodeBytes := uint64(reflect.TypeOf(node{}).Size())
	if e := m.BudgetField.Reserve(managerBytes); e != nil {
		return nil, e
	}
	v := &Manager{ast: m, header: managerBytes}
	v.self = v
	if tacticOwner {
		v.traceTacticAllocated()
	}
	if e := m.BudgetField.Reserve(impBytes); e != nil {
		m.BudgetField.Release(managerBytes)
		return nil, e
	}
	v.managerImp = &managerImp{
		table: make(map[andKey]*node), impHeader: impBytes, nodeBytes: nodeBytes,
		defaultGateEncoding: defaultGateEncoding, maxMemory: math.MaxUint64, runtime: numerals.RuntimeOwner(),
	}
	v.traceImpAllocated()
	h, e := m.MkTrue()
	if e != nil {
		m.BudgetField.Release(impBytes)
		m.BudgetField.Release(managerBytes)
		return nil, e
	}
	if e = m.BudgetField.Reserve(nodeBytes); e != nil {
		m.BudgetField.Release(impBytes)
		m.BudgetField.Release(managerBytes)
		return nil, e
	}
	r, e := m.NewRef(h)
	if e != nil {
		m.BudgetField.Release(nodeBytes)
		m.BudgetField.Release(impBytes)
		m.BudgetField.Release(managerBytes)
		return nil, e
	}
	v.andNodes.next = firstNodeID
	v.trueNode = &node{id: 0, refs: 2, variable: r}
	v.variables.next = 1
	v.nodes = 1
	return v, nil
}

func (m *Manager) open() error {
	if m == nil || m.self != m || m.managerImp == nil || m.closed || m.closing {
		return ErrOwner
	}
	if e := m.drainPendingAST(); e != nil {
		return e
	}
	return nil
}

// SetMaxMemory is aig_manager::set_max_memory; zero is a real source limit.
func (m *Manager) SetMaxMemory(limit uint64) error {
	if err := m.open(); err != nil {
		return err
	}
	m.maxMemory = limit
	return nil
}

// BindCooperation borrows the process cooperation owner for source checkpoints.
func (m *Manager) BindCooperation(owner *cooperate.Cooperation) error {
	if err := m.open(); err != nil {
		return err
	}
	if owner == nil {
		return arena.ErrConfig
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	m.cooperation = owner
	return nil
}

func (m *Manager) checkpoint() error {
	used, err := m.runtime.AllocationSize()
	if err != nil {
		return err
	}
	if used > m.maxMemory {
		return tactic.NewTacticException(messages.MaxMemory)
	}
	canceled, err := m.ast.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(m.ast.Limit.CancelMessage())
	}
	if m.cooperation != nil {
		return m.cooperation.Cooperate("aig")
	}
	return nil
}

func (m *Manager) releaseAST(ref *ast.ASTRef) error {
	if ref == nil || *ref == (ast.ASTRef{}) {
		return nil
	}
	e := m.ast.DelRef(ref)
	if *ref != (ast.ASTRef{}) {
		m.pendingAST = append(m.pendingAST, *ref)
		*ref = ast.ASTRef{}
	}
	return e
}
func (m *Manager) drainPendingAST() error {
	for len(m.pendingAST) != 0 {
		e := m.ast.DelRef(&m.pendingAST[0])
		if m.pendingAST[0] == (ast.ASTRef{}) {
			m.pendingAST = m.pendingAST[1:]
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (m *Manager) newRef(l Literal) (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if l.node == nil || l.node.refs == math.MaxUint32 || m.activeRefs == math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	l.node.refs++
	m.activeRefs++
	return &Ref{manager: m, lit: l}, nil
}
func (r *Ref) literal(m *Manager) (Literal, error) {
	if r == nil || r.closed || r.manager != m || r.lit.node == nil {
		return Literal{}, ErrOwner
	}
	if e := m.open(); e != nil {
		return Literal{}, e
	}
	return r.lit, nil
}
func (r *Ref) Clone() (*Ref, error) {
	if r == nil || r.manager == nil {
		return nil, ErrOwner
	}
	l, e := r.literal(r.manager)
	if e != nil {
		return nil, e
	}
	return r.manager.newRef(l)
}
func (r *Ref) Not() (*Ref, error) {
	if r == nil || r.manager == nil {
		return nil, ErrOwner
	}
	l, e := r.literal(r.manager)
	if e != nil {
		return nil, e
	}
	l.inverted = !l.inverted
	return r.manager.newRef(l)
}
func (r *Ref) ID() (uint32, bool, error) {
	if r == nil || r.manager == nil {
		return 0, false, ErrOwner
	}
	l, e := r.literal(r.manager)
	if e != nil {
		return 0, false, e
	}
	return l.node.id, l.inverted, nil
}
func (r *Ref) Same(other *Ref) bool {
	return r != nil && other != nil && !r.closed && !other.closed && r.manager == other.manager && r.lit == other.lit
}

func (m *Manager) True() (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	return m.newRef(Literal{node: m.trueNode})
}
func (m *Manager) False() (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	return m.newRef(Literal{node: m.trueNode, inverted: true})
}

// MakeVariable is mk_var's AST-owning leaf. The source caller decides whether
// two occurrences share a variable; this operation creates one new AIG node.
func (m *Manager) MakeVariable(h ast.ASTHandle) (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if h.IsNull() {
		return nil, ErrOwner
	}
	if _, e := m.ast.GetSort(h); e != nil {
		return nil, e
	}
	if m.activeRefs == math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	if e := m.ast.BudgetField.Reserve(m.nodeBytes); e != nil {
		return nil, e
	}
	owned, e := m.ast.NewRef(h)
	if e != nil {
		m.ast.BudgetField.Release(m.nodeBytes)
		return nil, e
	}
	id, e := m.variables.take()
	if e != nil {
		_ = m.ast.DelRef(&owned)
		m.ast.BudgetField.Release(m.nodeBytes)
		return nil, e
	}
	n := &node{id: id, variable: owned}
	m.nodes++
	return m.newRef(Literal{node: n})
}

// InternAnd is the source mk_node_core table path before mk_node's Boolean
// minimization rules. It allocates a transient node even on a duplicate lookup.
func (m *Manager) InternAnd(left, right *Ref) (*Ref, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	l, e := left.literal(m)
	if e != nil {
		return nil, e
	}
	r, e := right.literal(m)
	if e != nil {
		return nil, e
	}
	if m.activeRefs == math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	result, e := m.internCore(l, r)
	if e != nil {
		return nil, e
	}
	return m.newRef(result)
}

func (m *Manager) internCore(l, r Literal) (Literal, error) {
	if e := m.ast.BudgetField.Reserve(m.nodeBytes); e != nil {
		return Literal{}, e
	}
	key := andKey{l, r}
	if prior := m.table[key]; prior != nil {
		m.ast.BudgetField.Release(m.nodeBytes)
		return Literal{node: prior}, nil
	}
	id, e := m.andNodes.take()
	if e != nil {
		m.ast.BudgetField.Release(m.nodeBytes)
		return Literal{}, e
	}
	if l.node.refs == math.MaxUint32 || r.node.refs == math.MaxUint32 || l.node == r.node && l.node.refs >= math.MaxUint32-1 {
		m.andNodes.recycle(id)
		m.ast.BudgetField.Release(m.nodeBytes)
		return Literal{}, arena.ErrBounds
	}
	n := &node{id: id, children: [2]Literal{l, r}, and: true}
	m.table[key] = n
	m.nodes++
	l.node.refs++
	r.node.refs++
	return Literal{node: n}, nil
}

func (m *Manager) dec(l Literal) {
	n := l.node
	n.refs--
	if n.refs == 0 {
		m.toDelete = append(m.toDelete, n)
	}
}
func (m *Manager) deleteNode(n *node) error {
	if !n.traced {
		n.traced = true
		m.traceDeleting(n)
	}
	if !n.and {
		if n.stage == 0 {
			if e := m.ast.DelRef(&n.variable); e != nil {
				if n.variable == (ast.ASTRef{}) {
					n.stage = 1
				}
				return e
			}
			n.stage = 1
		}
		m.variables.recycle(n.id)
	} else {
		if n.stage == 0 {
			delete(m.table, andKey{n.children[0], n.children[1]})
			m.andNodes.recycle(n.id)
			n.stage = 1
		}
		if n.stage == 1 {
			m.dec(n.children[0])
			n.stage = 2
		}
		if n.stage == 2 {
			m.dec(n.children[1])
			n.stage = 3
		}
	}
	m.ast.BudgetField.Release(m.nodeBytes)
	m.nodes--
	return nil
}
func (m *Manager) drain() error {
	for len(m.toDelete) != 0 {
		last := len(m.toDelete) - 1
		n := m.toDelete[last]
		m.toDelete = m.toDelete[:last]
		if e := m.deleteNode(n); e != nil {
			m.toDelete = append(m.toDelete, n)
			return e
		}
	}
	return nil
}
func (r *Ref) Close() error {
	if r == nil || r.manager == nil {
		return ErrOwner
	}
	if r.closed {
		return nil
	}
	m := r.manager
	if e := m.open(); e != nil {
		return e
	}
	r.closed = true
	m.activeRefs--
	m.dec(r.lit)
	return m.drain()
}
func (m *Manager) NumNodes() (uint32, error) {
	if e := m.open(); e != nil {
		return 0, e
	}
	return m.nodes, nil
}

// Close releases the two source-owned true/false references after all aig_ref
// owners are gone. A stopped AST release stays in toDelete for owning retry.
func (m *Manager) Close() error {
	if m == nil || m.self != m || m.managerImp == nil {
		return ErrOwner
	}
	if m.closed {
		return nil
	}
	if m.activeRefs != 0 {
		return ErrLiveReferences
	}
	if e := m.drainPendingAST(); e != nil {
		return e
	}
	if !m.closing {
		m.closing = true
		m.dec(Literal{node: m.trueNode})
		m.dec(Literal{node: m.trueNode, inverted: true})
	}
	if e := m.drain(); e != nil {
		return e
	}
	if m.nodes != 0 {
		return ErrLiveReferences
	}
	m.ast.BudgetField.Release(m.impHeader)
	m.traceImpDeallocated()
	m.ast.BudgetField.Release(m.header)
	m.closed = true
	m.table = nil
	return nil
}
