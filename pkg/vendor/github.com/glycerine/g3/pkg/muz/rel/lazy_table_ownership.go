package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type lazyConditionOwner struct {
	engine  *LazyTableEngine
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func removeLazyTableHandle(handles []*LazyTable, table *LazyTable) []*LazyTable {
	for index, candidate := range handles {
		if candidate == table {
			copy(handles[index:], handles[index+1:])
			return handles[:len(handles)-1]
		}
	}
	return handles
}

func removeLazyConditionOwner(owners []*lazyConditionOwner, owner *lazyConditionOwner) []*lazyConditionOwner {
	for index, candidate := range owners {
		if candidate == owner {
			copy(owners[index:], owners[index+1:])
			return owners[:len(owners)-1]
		}
	}
	return owners
}

func (o *lazyConditionOwner) release() (complete bool, result error) {
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
		o.engine.conditions = removeLazyConditionOwner(o.engine.conditions, o)
	}
	o.engine, o.manager = nil, nil
	return true, result
}

func newLazyTableNode(node *lazyTableNode) *lazyTableNode {
	node.references = 1
	if node.left != nil {
		node.left.references++
	}
	if node.right != nil {
		node.right.references++
	}
	return node
}

func (t *LazyTable) replaceRoot(node *lazyTableNode) error {
	old := t.root
	t.root = newLazyTableNode(node)
	consumed, err := releaseLazyTableNode(old)
	if !consumed {
		t.root = old
		return err
	}
	return err
}

// releaseLazyTableNode drops one owner edge. A false result means destruction
// stopped before that edge was consumed, so the caller must retain its pointer
// and may retry Close.
func releaseLazyTableNode(node *lazyTableNode) (consumed bool, result error) {
	if node == nil {
		return true, nil
	}
	if node.closed || node.references == 0 {
		return false, ErrLazyTable
	}
	node.references--
	if node.references != 0 {
		return true, nil
	}
	releaseChild := func(child **lazyTableNode) bool {
		if *child == nil {
			return true
		}
		used, err := releaseLazyTableNode(*child)
		result = errors.Join(result, err)
		if used {
			*child = nil
		}
		return used
	}
	if !releaseChild(&node.right) {
		node.references = 1
		return false, result
	}
	if !releaseChild(&node.left) {
		node.references = 1
		return false, result
	}
	if node.interpreted != nil && node.interpreted.owner != nil {
		complete, err := node.interpreted.owner.release()
		result = errors.Join(result, err)
		if !complete {
			node.references = 1
			return false, result
		}
		node.interpreted.owner = nil
	}
	if node.evaluated != nil {
		result = errors.Join(result, node.evaluated.Close())
		node.evaluated = nil
	}
	node.closed = true
	return true, result
}
