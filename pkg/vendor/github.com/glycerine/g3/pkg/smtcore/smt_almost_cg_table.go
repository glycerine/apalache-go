package smtcore

// Source: src/smt/smt_almost_cg_table.h and src/smt/smt_almost_cg_table.cpp.

import "github.com/glycerine/g3/pkg/ast"

type smtAlmostCongruenceGroup struct {
	key    *SMTEnode
	values []*SMTEnode
}

type SMTAlmostCongruenceTable struct {
	first, second *SMTEnode
	groups        []smtAlmostCongruenceGroup
}

func NewSMTAlmostCongruenceTable(first, second *SMTEnode) *SMTAlmostCongruenceTable {
	return &SMTAlmostCongruenceTable{first: first, second: second}
}

func (t *SMTAlmostCongruenceTable) equivalent(left, right *SMTEnode) (bool, error) {
	if t == nil || left == nil || right == nil {
		return false, ast.ErrASTHandle
	}
	if err := left.valid(); err != nil {
		return false, err
	}
	if err := right.valid(); err != nil {
		return false, err
	}
	if left.declaration != right.declaration || len(left.arguments) != len(right.arguments) {
		return false, nil
	}
	for i := range left.arguments {
		leftRoot, rightRoot := left.arguments[i].root, right.arguments[i].root
		if leftRoot == rightRoot {
			continue
		}
		leftSpecial := leftRoot == t.first || leftRoot == t.second
		rightSpecial := rightRoot == t.first || rightRoot == t.second
		if !leftSpecial || !rightSpecial {
			return false, nil
		}
	}
	return true, nil
}

func (t *SMTAlmostCongruenceTable) ResetFor(first, second *SMTEnode) error {
	if t == nil || first == nil || second == nil {
		return ast.ErrASTHandle
	}
	if err := first.valid(); err != nil {
		return err
	}
	if err := second.valid(); err != nil {
		return err
	}
	t.first, t.second = first.root, second.root
	t.groups = nil
	return nil
}

func (t *SMTAlmostCongruenceTable) Reset() {
	if t != nil {
		t.groups = nil
	}
}

func (t *SMTAlmostCongruenceTable) Insert(node *SMTEnode) error {
	if t == nil || node == nil {
		return ast.ErrASTHandle
	}
	if err := node.valid(); err != nil {
		return err
	}
	for i := range t.groups {
		equal, err := t.equivalent(t.groups[i].key, node)
		if err != nil {
			return err
		}
		if equal {
			t.groups[i].values = append([]*SMTEnode{node}, t.groups[i].values...)
			return nil
		}
	}
	t.groups = append(t.groups, smtAlmostCongruenceGroup{key: node, values: []*SMTEnode{node}})
	return nil
}

func (t *SMTAlmostCongruenceTable) Erase(node *SMTEnode) (bool, error) {
	if t == nil || node == nil {
		return false, ast.ErrASTHandle
	}
	for i := range t.groups {
		equal, err := t.equivalent(t.groups[i].key, node)
		if err != nil {
			return false, err
		}
		if equal {
			copy(t.groups[i:], t.groups[i+1:])
			t.groups = t.groups[:len(t.groups)-1]
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTAlmostCongruenceTable) Find(node *SMTEnode) ([]*SMTEnode, error) {
	if t == nil || node == nil {
		return nil, ast.ErrASTHandle
	}
	for i := range t.groups {
		equal, err := t.equivalent(t.groups[i].key, node)
		if err != nil {
			return nil, err
		}
		if equal {
			return append([]*SMTEnode(nil), t.groups[i].values...), nil
		}
	}
	return nil, nil
}

func (t *SMTAlmostCongruenceTable) Empty() bool {
	return t == nil || len(t.groups) == 0
}
