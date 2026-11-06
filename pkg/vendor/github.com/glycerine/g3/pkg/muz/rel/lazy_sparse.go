package rel

// Source: src/muz/rel/dl_lazy_table.h/.cpp lazy_table_ref hierarchy with the
// concrete sparse_table_plugin registered by rel_context.

import "errors"

var ErrLazySparse = errors.New("muz/rel: invalid lazy sparse table operation")

type LazySparseStatistics struct {
	Forces, Joins, JoinProjects, Projects   uint32
	EqualFilters, IdenticalFilters          uint32
	InterpretedFilters, NegationFilters     uint32
	SelectEqualProject, InterpretedProjects uint32
}

type lazySparseNode struct {
	kind        LazyTableKind
	references  uint32
	signature   Signature
	functional  int
	evaluated   *SparseTable
	nullary     *HashTable
	left        *lazySparseNode
	right       *lazySparseNode
	columns1    []int
	columns2    []int
	value       Element
	column      int
	interpreted *lazySparseInterpretedFilter
	retryForce  bool
}

type LazySparseEngine struct {
	stats      LazySparseStatistics
	closed     bool
	handles    []*LazySparseTable
	conditions []*lazySparseConditionOwner
}

type LazySparseTable struct {
	engine *LazySparseEngine
	root   *lazySparseNode
	closed bool
}

func NewLazySparseEngine() *LazySparseEngine { return &LazySparseEngine{} }

func (e *LazySparseEngine) open() error {
	if e == nil || e.closed {
		return ErrLazySparse
	}
	return nil
}

func (e *LazySparseEngine) Statistics() LazySparseStatistics {
	if e == nil {
		return LazySparseStatistics{}
	}
	return e.stats
}

func (e *LazySparseEngine) adopt(root *lazySparseNode) *LazySparseTable {
	root.references++
	if root.left != nil {
		root.left.references++
	}
	if root.right != nil {
		root.right.references++
	}
	table := &LazySparseTable{engine: e, root: root}
	e.handles = append(e.handles, table)
	return table
}

func (e *LazySparseEngine) New(signature Signature, functional int) (*LazySparseTable, error) {
	if err := e.open(); err != nil || signature.Len() == 0 {
		return nil, ErrLazySparse
	}
	table, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	return e.adopt(&lazySparseNode{kind: LazyTableBase, signature: Signature{domains: signature.Domains()}, functional: functional, evaluated: table}), nil
}

func releaseLazySparseNode(node *lazySparseNode) error {
	if node == nil || node.references == 0 {
		return ErrLazySparse
	}
	if node.references > 1 {
		node.references--
		return nil
	}
	if node.right != nil {
		if err := releaseLazySparseNode(node.right); err != nil {
			return err
		}
		node.right = nil
	}
	if node.left != nil {
		if err := releaseLazySparseNode(node.left); err != nil {
			return err
		}
		node.left = nil
	}
	if node.interpreted != nil && node.interpreted.owner != nil {
		complete, err := node.interpreted.owner.release()
		if !complete || err != nil {
			return err
		}
		node.interpreted.owner = nil
	}
	if node.evaluated != nil {
		if err := node.evaluated.Close(); err != nil {
			return err
		}
		node.evaluated = nil
	}
	if node.nullary != nil {
		if err := node.nullary.Close(); err != nil {
			return err
		}
		node.nullary = nil
	}
	node.references = 0
	return nil
}

func (t *LazySparseTable) open() error {
	if t == nil || t.closed || t.root == nil || t.engine == nil || t.engine.open() != nil {
		return ErrLazySparse
	}
	return nil
}

func (t *LazySparseTable) Close() error {
	if t == nil || t.closed {
		return nil
	}
	if err := releaseLazySparseNode(t.root); err != nil {
		return err
	}
	for index, handle := range t.engine.handles {
		if handle == t {
			t.engine.handles = append(t.engine.handles[:index], t.engine.handles[index+1:]...)
			break
		}
	}
	t.closed, t.root, t.engine = true, nil, nil
	return nil
}

func (e *LazySparseEngine) Close() (result error) {
	if e == nil || e.closed {
		return nil
	}
	for len(e.handles) != 0 {
		err := e.handles[len(e.handles)-1].Close()
		result = errors.Join(result, err)
		if err != nil {
			return result
		}
	}
	for len(e.conditions) != 0 {
		owner := e.conditions[len(e.conditions)-1]
		complete, err := owner.release()
		result = errors.Join(result, err)
		if !complete || err != nil {
			return result
		}
	}
	e.closed = true
	return result
}

func (t *LazySparseTable) Forced() bool {
	return t.open() == nil && ((t.root.evaluated != nil && !t.root.retryForce) || t.root.nullary != nil)
}

func (t *LazySparseTable) Add(fact Fact) (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	return table.Add(fact)
}

func (t *LazySparseTable) Facts() ([]Fact, error) {
	table, err := t.finiteEval()
	if err != nil {
		return nil, err
	}
	return table.Facts()
}

func (t *LazySparseTable) FunctionalColumns() (int, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.root.functional, nil
}
