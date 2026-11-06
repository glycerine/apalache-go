package rel

// Source: src/muz/rel/dl_lazy_table.h/.cpp lazy_table_ref hierarchy and
// force/eval implementation. This is the concrete retained HashTable
// specialization; interpreted AST filtering is provided by the finite table
// relation evaluator.

import (
	"errors"
	"fmt"
)

var ErrLazyTable = errors.New("muz/rel: invalid lazy table operation")

type LazyTableKind uint8

const (
	LazyTableBase LazyTableKind = iota
	LazyTableJoin
	LazyTableProject
	LazyTableRename
	LazyTableFilterIdentical
	LazyTableFilterEqual
	LazyTableFilterByNegation
	LazyTableFilterInterpreted
)

type LazyTableStatistics struct {
	Forces              uint32
	Joins               uint32
	JoinProjects        uint32
	Projects            uint32
	Renames             uint32
	EqualFilters        uint32
	IdenticalFilters    uint32
	NegationFilters     uint32
	SelectEqualProject  uint32
	InterpretedFilters  uint32
	InterpretedProjects uint32
}

// LazyTableEngine is the concrete counterpart of lazy_table_plugin. Tables
// from one engine may share deferred nodes and a deterministic statistics log.
type LazyTableEngine struct {
	stats      LazyTableStatistics
	closed     bool
	handles    []*LazyTable
	conditions []*lazyConditionOwner
}

func NewLazyTableEngine() *LazyTableEngine { return &LazyTableEngine{} }

func (e *LazyTableEngine) open() error {
	if e == nil || e.closed {
		return ErrLazyTable
	}
	return nil
}

func (e *LazyTableEngine) Statistics() LazyTableStatistics {
	if e == nil {
		return LazyTableStatistics{}
	}
	return e.stats
}

func (e *LazyTableEngine) ResetStatistics() {
	if e != nil && !e.closed {
		e.stats = LazyTableStatistics{}
	}
}

func (e *LazyTableEngine) Close() error {
	if e == nil {
		return nil
	}
	var result error
	handles := append([]*LazyTable(nil), e.handles...)
	for index := len(handles) - 1; index >= 0; index-- {
		table := handles[index]
		err := table.Close()
		result = errors.Join(result, err)
		if !table.closed {
			return result
		}
	}
	conditions := append([]*lazyConditionOwner(nil), e.conditions...)
	for index := len(conditions) - 1; index >= 0; index-- {
		owner := conditions[index]
		complete, err := owner.release()
		result = errors.Join(result, err)
		if !complete {
			return result
		}
	}
	e.closed = true
	return result
}

type lazyTableNode struct {
	kind        LazyTableKind
	references  uint32
	closed      bool
	signature   Signature
	evaluated   *HashTable
	left        *lazyTableNode
	right       *lazyTableNode
	columns1    []int
	columns2    []int
	value       Element
	column      int
	interpreted *lazyInterpretedFilter
	retryForce  bool
}

type LazyTable struct {
	engine *LazyTableEngine
	root   *lazyTableNode
	closed bool
}

func (e *LazyTableEngine) New(signature Signature) (*LazyTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	table := NewHashTable(signature)
	return e.adoptNode(&lazyTableNode{kind: LazyTableBase, signature: Signature{domains: signature.Domains()}, evaluated: table}), nil
}

// AdoptLazyTable transfers ownership of table to a forced base node.
func (e *LazyTableEngine) Adopt(table *HashTable) (*LazyTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if table == nil || table.open() != nil {
		return nil, ErrLazyTable
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	return e.adoptNode(&lazyTableNode{kind: LazyTableBase, signature: signature, evaluated: table}), nil
}

func (e *LazyTableEngine) adoptNode(root *lazyTableNode) *LazyTable {
	newLazyTableNode(root)
	table := &LazyTable{engine: e, root: root}
	e.handles = append(e.handles, table)
	return table
}

func (t *LazyTable) open() error {
	if t == nil || t.closed || t.root == nil || t.engine == nil || t.engine.open() != nil {
		return ErrLazyTable
	}
	return nil
}

func (t *LazyTable) Close() error {
	if t == nil || t.closed {
		return nil
	}
	consumed, err := releaseLazyTableNode(t.root)
	if !consumed {
		return err
	}
	e := t.engine
	t.closed = true
	t.root = nil
	if e != nil {
		e.handles = removeLazyTableHandle(e.handles, t)
	}
	t.engine = nil
	return err
}

func (t *LazyTable) Kind() LazyTableKind {
	if t.open() != nil {
		return LazyTableBase
	}
	return t.root.kind
}

func (t *LazyTable) Forced() bool {
	return t.open() == nil && t.root.evaluated != nil && !t.root.retryForce
}

// SizeEstimateRows and SizeEstimateBytes preserve lazy_table's fixed source
// estimates without forcing the deferred operation.
func (t *LazyTable) SizeEstimateRows() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazyTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazyTable) KnowsExactSize() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	return false, nil
}

func (t *LazyTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.root.signature.Domains()}, nil
}

func (e *LazyTableEngine) evaluate(node *lazyTableNode) (*HashTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if node == nil {
		return nil, ErrLazyTable
	}
	if node.evaluated != nil && !node.retryForce {
		return node.evaluated, nil
	}
	e.stats.Forces++
	var result *HashTable
	var err error
	switch node.kind {
	case LazyTableJoin:
		left, leftErr := e.evaluate(node.left)
		if leftErr != nil {
			return nil, leftErr
		}
		right, rightErr := e.evaluate(node.right)
		if rightErr != nil {
			return nil, rightErr
		}
		e.stats.Joins++
		result, err = left.Join(right, node.columns1, node.columns2)
	case LazyTableProject:
		if node.left != nil && node.left.kind == LazyTableJoin {
			join := node.left
			left, leftErr := e.evaluate(join.left)
			if leftErr != nil {
				return nil, leftErr
			}
			right, rightErr := e.evaluate(join.right)
			if rightErr != nil {
				return nil, rightErr
			}
			e.stats.JoinProjects++
			result, err = left.JoinProject(right, join.columns1, join.columns2, node.columns1)
		} else if node.left != nil && node.left.kind == LazyTableFilterEqual && len(node.columns1) == 1 && node.columns1[0] == node.left.column {
			filter := node.left
			source := filter.evaluated
			if source == nil {
				var sourceErr error
				source, sourceErr = e.evaluate(filter.left)
				if sourceErr != nil {
					return nil, sourceErr
				}
			}
			e.stats.SelectEqualProject++
			result, err = source.SelectEqualAndProject(filter.value, filter.column)
		} else if node.left != nil && node.left.kind == LazyTableFilterInterpreted && node.left.evaluated == nil {
			filter := node.left
			source, sourceErr := e.evaluate(filter)
			if sourceErr != nil {
				return nil, sourceErr
			}
			interpreted := filter.interpreted
			conditions, conditionErr := interpreted.handles()
			if conditionErr != nil {
				return nil, conditionErr
			}
			relation := &TableRelation{manager: interpreted.manager, sorts: interpreted.relationSorts, table: source}
			projected, projectErr := relation.FilterInterpretedAndProject(
				interpreted.rewriter,
				conditions,
				interpreted.columns,
				interpreted.sorts,
				node.columns1,
			)
			if projectErr != nil {
				if projected != nil {
					result := projected.Table()
					projected.Close()
					if result != nil {
						projectErr = errors.Join(projectErr, result.Close())
					}
				}
				return nil, projectErr
			}
			result = projected.Table()
			projected.Close()
			e.stats.InterpretedProjects++
		} else {
			source, sourceErr := e.evaluate(node.left)
			if sourceErr != nil {
				return nil, sourceErr
			}
			e.stats.Projects++
			result, err = source.Project(node.columns1)
		}
	case LazyTableRename:
		source, sourceErr := e.evaluate(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		e.stats.Renames++
		result, err = source.RenameCycle(node.columns1)
	case LazyTableFilterIdentical:
		source, sourceErr := e.transferLazySource(node, &node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		e.stats.IdenticalFilters++
		err = source.FilterIdentical(node.columns1)
		result = source
	case LazyTableFilterEqual:
		source, sourceErr := e.transferLazySource(node, &node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		e.stats.EqualFilters++
		err = source.FilterEqual(node.value, node.column)
		result = source
	case LazyTableFilterByNegation:
		target, targetErr := e.transferLazySource(node, &node.left)
		if targetErr != nil {
			return nil, targetErr
		}
		if node.right != nil && node.right.kind == LazyTableJoin {
			join := node.right
			left, leftErr := e.evaluate(join.left)
			if leftErr != nil {
				return nil, leftErr
			}
			right, rightErr := e.evaluate(join.right)
			if rightErr != nil {
				return nil, rightErr
			}
			e.stats.NegationFilters++
			err = lazyFilterByNegatedJoin(target, left, right, node.columns1, node.columns2, join.columns1, join.columns2)
		} else {
			source, sourceErr := e.evaluate(node.right)
			if sourceErr != nil {
				return nil, sourceErr
			}
			e.stats.NegationFilters++
			err = target.FilterByNegation(source, node.columns1, node.columns2)
		}
		result = target
	case LazyTableFilterInterpreted:
		source, sourceErr := e.transferLazySource(node, &node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		// A Go AST operation can refuse after the source table has transferred.
		// Keep the concrete owner but do not publish it as a completed force;
		// a retry must execute the transactional interpreted filter again.
		conditions, conditionErr := node.interpreted.handles()
		if conditionErr != nil {
			return nil, conditionErr
		}
		relation := &TableRelation{manager: node.interpreted.manager, sorts: node.interpreted.relationSorts, table: source}
		err = relation.FilterInterpreted(node.interpreted.rewriter, conditions, node.interpreted.columns, node.interpreted.sorts)
		if err == nil {
			e.stats.InterpretedFilters++
			node.retryForce = false
			result = source
		}
	default:
		return nil, fmt.Errorf("%w: unsupported deferred node kind", ErrLazyTable)
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrLazyTable
	}
	node.evaluated = result
	node.retryForce = false
	return result, nil
}

// lazyFilterByNegatedJoin is the concrete HashTable counterpart of
// mk_filter_by_negated_join_fn used by lazy_table_filter_by_negation::force.
// It observes the two join inputs directly, so the shared join node remains
// deferred and does not allocate a temporary joined table.
func lazyFilterByNegatedJoin(target, left, right *HashTable, targetColumns, joinedColumns, leftJoinColumns, rightJoinColumns []int) error {
	if err := validateColumns(left.signature, right.signature, leftJoinColumns, rightJoinColumns); err != nil {
		return err
	}
	joinedDomains := append(left.signature.Domains(), right.signature.domains...)
	joinedSignature, err := NewSignature(joinedDomains)
	if err != nil {
		return err
	}
	if err = validateColumns(target.signature, joinedSignature, targetColumns, joinedColumns); err != nil {
		return err
	}
	targetFacts, err := target.Facts()
	if err != nil {
		return err
	}
	leftFacts, err := left.Facts()
	if err != nil {
		return err
	}
	rightFacts, err := right.Facts()
	if err != nil {
		return err
	}
	joinedValue := func(first, second Fact, column int) Element {
		if column < len(first) {
			return first[column]
		}
		return second[column-len(first)]
	}
	for _, fact := range targetFacts {
		remove := false
		for _, first := range leftFacts {
			for _, second := range rightFacts {
				matches := true
				for index := range leftJoinColumns {
					if first[leftJoinColumns[index]] != second[rightJoinColumns[index]] {
						matches = false
						break
					}
				}
				for index := 0; matches && index < len(targetColumns); index++ {
					if fact[targetColumns[index]] != joinedValue(first, second, joinedColumns[index]) {
						matches = false
					}
				}
				if matches {
					remove = true
					break
				}
			}
			if remove {
				break
			}
		}
		if remove {
			if _, err = target.Remove(fact); err != nil {
				return err
			}
		}
	}
	return nil
}

// Eval forces and memoizes the operation DAG and returns a borrowed table.
func (t *LazyTable) Eval() (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.engine.evaluate(t.root)
}

func (t *LazyTable) Empty() (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	length, err := table.Len()
	return length == 0, err
}

func (t *LazyTable) Contains(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Contains(fact)
}

func (t *LazyTable) Add(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Add(fact)
}

func (t *LazyTable) Remove(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Remove(fact)
}

func (t *LazyTable) Facts() ([]Fact, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	return table.Facts()
}

func (t *LazyTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	root := &lazyTableNode{kind: LazyTableBase, references: 1, signature: Signature{domains: t.root.signature.Domains()}, evaluated: NewHashTable(t.root.signature)}
	consumed, err := releaseLazyTableNode(t.root)
	if !consumed {
		_, cleanupErr := releaseLazyTableNode(root)
		return errors.Join(err, cleanupErr)
	}
	t.root = root
	return err
}

func (t *LazyTable) Clone() (*LazyTable, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	copy, err := table.Clone()
	if err != nil {
		return nil, err
	}
	return t.engine.Adopt(copy)
}

func (t *LazyTable) Complement() (*LazyTable, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrLazyTable)
	}
	result := NewHashTable(signature)
	if signature.Len() == 0 {
		length, lengthErr := table.Len()
		if lengthErr != nil {
			return nil, lengthErr
		}
		if length == 0 {
			_, err = result.Add(Fact{})
		}
	} else {
		for value := uint64(0); value < signature.domains[0]; value++ {
			fact := Fact{Element(value)}
			found, containsErr := table.Contains(fact)
			if containsErr != nil {
				return nil, containsErr
			}
			if !found {
				_, err = result.Add(fact)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return t.engine.Adopt(result)
}
