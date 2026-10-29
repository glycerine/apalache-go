package smtcore

// Source: src/smt/smt_model_finder.cpp mf::node.

import "github.com/glycerine/g3/pkg/ast"

type smtModelFinderAUFNode struct {
	id         uint32
	parent     *smtModelFinderAUFNode
	size       uint32
	sort       ast.ASTHandle
	mono       bool
	signed     bool
	avoid      []*smtModelFinderAUFNode
	exceptions []ast.ASTHandle
	set        *smtModelFinderInstantiationSet
	elseValue  ast.ASTHandle
	projection ast.ASTHandle
}

func newSMTModelFinderAUFNode(id uint32, sort ast.ASTHandle) (*smtModelFinderAUFNode, error) {
	if sort.IsNull() {
		return nil, ast.ErrASTHandle
	}
	return &smtModelFinderAUFNode{id: id, size: 1, sort: sort}, nil
}

func (n *smtModelFinderAUFNode) root() *smtModelFinderAUFNode {
	if n == nil {
		return nil
	}
	root := n
	for root.parent != nil {
		root = root.parent
	}
	for n.parent != nil && n.parent != root {
		next := n.parent
		n.parent = root
		n = next
	}
	return root
}

func appendUniqueNode(values []*smtModelFinderAUFNode, value *smtModelFinderAUFNode) []*smtModelFinderAUFNode {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueHandle(values []ast.ASTHandle, value ast.ASTHandle) []ast.ASTHandle {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func (n *smtModelFinderAUFNode) merge(other *smtModelFinderAUFNode) error {
	left, right := n.root(), other.root()
	if left == nil || right == nil {
		return ast.ErrASTHandle
	}
	if left == right {
		return nil
	}
	if left.set != nil || right.set != nil || left.sort != right.sort {
		return ast.ErrASTDomain
	}
	if left.size > right.size {
		left, right = right, left
	}
	left.parent = right
	right.size += left.size
	right.mono = right.mono || left.mono
	right.signed = right.signed || left.signed
	for _, avoid := range left.avoid {
		right.avoid = appendUniqueNode(right.avoid, avoid)
	}
	for _, exception := range left.exceptions {
		right.exceptions = appendUniqueHandle(right.exceptions, exception)
	}
	left.avoid = nil
	left.exceptions = nil
	return nil
}

func (n *smtModelFinderAUFNode) insertAvoid(other *smtModelFinderAUFNode) error {
	root := n.root()
	if root == nil || other == nil {
		return ast.ErrASTHandle
	}
	root.avoid = appendUniqueNode(root.avoid, other)
	return nil
}

func (n *smtModelFinderAUFNode) insertException(value ast.ASTHandle) error {
	root := n.root()
	if root == nil || value.IsNull() {
		return ast.ErrASTHandle
	}
	root.exceptions = appendUniqueHandle(root.exceptions, value)
	return nil
}

func (n *smtModelFinderAUFNode) setMono() error {
	root := n.root()
	if root == nil {
		return ast.ErrASTHandle
	}
	root.mono = true
	return nil
}

func (n *smtModelFinderAUFNode) setSigned() error {
	root := n.root()
	if root == nil {
		return ast.ErrASTHandle
	}
	root.signed = true
	return nil
}

func (n *smtModelFinderAUFNode) makeInstantiationSet(manager *ast.ASTManager) error {
	root := n.root()
	if root == nil || manager == nil {
		return ast.ErrASTHandle
	}
	if root.set != nil {
		return ast.ErrASTDomain
	}
	set, err := newSMTModelFinderInstantiationSet(manager)
	if err != nil {
		return err
	}
	root.set = set
	return nil
}

func (n *smtModelFinderAUFNode) insert(value ast.ASTHandle, generation uint32) (bool, error) {
	root := n.root()
	if root == nil || root.set == nil {
		return false, ast.ErrASTHandle
	}
	return root.set.Insert(value, generation)
}

func (n *smtModelFinderAUFNode) mustAvoidItself() bool {
	root := n.root()
	if root == nil {
		return false
	}
	for _, other := range root.avoid {
		if other.root() == root {
			return true
		}
	}
	return false
}

func (n *smtModelFinderAUFNode) setElse(value ast.ASTHandle) error {
	root := n.root()
	if root == nil || value.IsNull() {
		return ast.ErrASTHandle
	}
	if root.mono || !root.elseValue.IsNull() {
		return ast.ErrASTDomain
	}
	root.elseValue = value
	return nil
}

func (n *smtModelFinderAUFNode) setProjection(declaration ast.ASTHandle) error {
	root := n.root()
	if root == nil || declaration.IsNull() {
		return ast.ErrASTHandle
	}
	if !root.projection.IsNull() {
		return ast.ErrASTDomain
	}
	root.projection = declaration
	return nil
}

func (n *smtModelFinderAUFNode) close() error {
	if n == nil || n.set == nil {
		return nil
	}
	err := n.set.Close()
	if err == nil {
		n.set = nil
	}
	return err
}
