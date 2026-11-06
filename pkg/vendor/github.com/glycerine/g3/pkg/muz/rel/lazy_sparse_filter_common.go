package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type lazySparseConditionOwner struct {
	engine  *LazySparseEngine
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func removeLazySparseConditionOwner(owners []*lazySparseConditionOwner, owner *lazySparseConditionOwner) []*lazySparseConditionOwner {
	for index, candidate := range owners {
		if candidate == owner {
			copy(owners[index:], owners[index+1:])
			return owners[:len(owners)-1]
		}
	}
	return owners
}

func (o *lazySparseConditionOwner) release() (complete bool, result error) {
	if o == nil {
		return true, nil
	}
	for len(o.refs) != 0 {
		index := len(o.refs) - 1
		err := o.manager.DelRef(&o.refs[index])
		result = errors.Join(result, err)
		if o.refs[index] != (ast.ASTRef{}) {
			return false, result
		}
		o.refs = o.refs[:index]
	}
	if o.engine != nil {
		o.engine.conditions = removeLazySparseConditionOwner(o.engine.conditions, o)
	}
	o.engine, o.manager = nil, nil
	return true, result
}

func newLazySparseDeferredNode(node *lazySparseNode) *lazySparseNode {
	node.references = 1
	if node.left != nil {
		node.left.references++
	}
	if node.right != nil {
		node.right.references++
	}
	return node
}

func (t *LazySparseTable) replaceLazySparseRoot(node *lazySparseNode) error {
	if err := t.open(); err != nil {
		return err
	}
	old := t.root
	newLazySparseDeferredNode(node)
	t.root = node
	if err := releaseLazySparseNode(old); err != nil {
		t.root = old
		node.references = 0
		if node.left != nil && node.left.references != 0 {
			node.left.references--
		}
		if node.right != nil && node.right.references != 0 {
			node.right.references--
		}
		return err
	}
	return nil
}
