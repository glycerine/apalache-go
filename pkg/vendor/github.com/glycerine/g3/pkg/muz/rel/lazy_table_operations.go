package rel

// Source: src/muz/rel/dl_lazy_table.cpp concrete operation factories.

import "fmt"

func (t *LazyTable) sameEngine(other *LazyTable) bool {
	return t != nil && other != nil && t.engine != nil && t.engine == other.engine
}

func (t *LazyTable) Join(other *LazyTable, leftColumns, rightColumns []int) (*LazyTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || !t.sameEngine(other) {
		return nil, ErrLazyTable
	}
	if err := validateColumns(t.root.signature, other.root.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	domains := append(t.root.signature.Domains(), other.root.signature.domains...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	node := &lazyTableNode{kind: LazyTableJoin, signature: signature, left: t.root, right: other.root, columns1: append([]int(nil), leftColumns...), columns2: append([]int(nil), rightColumns...)}
	return t.engine.adoptNode(node), nil
}

func (t *LazyTable) Project(removed []int) (*LazyTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, err := projectedSignature(t.root.signature, removed)
	if err != nil {
		return nil, err
	}
	node := &lazyTableNode{kind: LazyTableProject, signature: signature, left: t.root, columns1: append([]int(nil), removed...)}
	return t.engine.adoptNode(node), nil
}

func lazyRenameSignature(signature Signature, cycle []int) (Signature, error) {
	permutation := make([]int, signature.Len())
	for i := range permutation {
		permutation[i] = i
	}
	if len(cycle) >= 2 {
		seen := make([]bool, len(permutation))
		for _, column := range cycle {
			if column < 0 || column >= len(permutation) || seen[column] {
				return Signature{}, fmt.Errorf("%w: invalid rename cycle", ErrLazyTable)
			}
			seen[column] = true
		}
		for i := 1; i < len(cycle); i++ {
			permutation[cycle[i-1]] = cycle[i]
		}
		permutation[cycle[len(cycle)-1]] = cycle[0]
	}
	domains := make([]uint64, len(permutation))
	for i, source := range permutation {
		domains[i] = signature.domains[source]
	}
	return NewSignature(domains)
}

func (t *LazyTable) RenameCycle(cycle []int) (*LazyTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, err := lazyRenameSignature(t.root.signature, cycle)
	if err != nil {
		return nil, err
	}
	node := &lazyTableNode{kind: LazyTableRename, signature: signature, left: t.root, columns1: append([]int(nil), cycle...)}
	return t.engine.adoptNode(node), nil
}

// FilterEqual updates the lazy handle without forcing it.
func (t *LazyTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if column < 0 || column >= t.root.signature.Len() {
		return fmt.Errorf("%w: filter column out of range", ErrLazyTable)
	}
	return t.replaceRoot(&lazyTableNode{kind: LazyTableFilterEqual, signature: Signature{domains: t.root.signature.Domains()}, left: t.root, value: value, column: column})
}

// FilterIdentical updates the lazy handle without forcing it.
func (t *LazyTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, column := range columns {
		if column < 0 || column >= t.root.signature.Len() {
			return fmt.Errorf("%w: identical-filter column out of range", ErrLazyTable)
		}
	}
	return t.replaceRoot(&lazyTableNode{kind: LazyTableFilterIdentical, signature: Signature{domains: t.root.signature.Domains()}, left: t.root, columns1: append([]int(nil), columns...)})
}

// FilterByNegation updates the lazy target without forcing either operand.
func (t *LazyTable) FilterByNegation(source *LazyTable, targetColumns, sourceColumns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !t.sameEngine(source) {
		return ErrLazyTable
	}
	if err := validateColumns(t.root.signature, source.root.signature, targetColumns, sourceColumns); err != nil {
		return err
	}
	return t.replaceRoot(&lazyTableNode{kind: LazyTableFilterByNegation, signature: Signature{domains: t.root.signature.Domains()}, left: t.root, right: source.root, columns1: append([]int(nil), targetColumns...), columns2: append([]int(nil), sourceColumns...)})
}

// UnionFrom is eager in the source lazy plugin: all three operands are forced
// before dispatching the underlying union operation.
func (t *LazyTable) UnionFrom(source, delta *LazyTable) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !t.sameEngine(source) || !t.root.signature.equal(source.root.signature) {
		return fmt.Errorf("%w: union signatures or engines differ", ErrLazyTable)
	}
	sourceTable, err := source.Eval()
	if err != nil {
		return err
	}
	target, err := t.Eval()
	if err != nil {
		return err
	}
	var deltaTable *HashTable
	if delta != nil {
		if delta.open() != nil || !t.sameEngine(delta) || !t.root.signature.equal(delta.root.signature) {
			return fmt.Errorf("%w: delta signature or engine differs", ErrLazyTable)
		}
		deltaTable, err = delta.Eval()
		if err != nil {
			return err
		}
	}
	return target.Union(sourceTable, deltaTable)
}

// RemoveFacts forces the lazy table and forwards bulk removal to its concrete
// table, matching lazy_table::remove_facts.
func (t *LazyTable) RemoveFacts(facts []Fact) error {
	table, err := t.Eval()
	if err != nil {
		return err
	}
	return table.RemoveFacts(facts)
}
