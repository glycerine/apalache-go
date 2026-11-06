package rewriter

// Source: src/ast/rewriter/var_subst.cpp:176-246;
// src/ast/rewriter/var_subst.h:82-102.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// ExprFreeVars collects borrowed sorts for free de Bruijn variables.
type ExprFreeVars struct {
	self    *ExprFreeVars
	manager *ast.ASTManager
	marked  map[ast.ASTHandle]struct{}
	sorts   []ast.ASTHandle
}

func NewExprFreeVars(m *ast.ASTManager) (*ExprFreeVars, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	c := &ExprFreeVars{manager: m, marked: make(map[ast.ASTHandle]struct{})}
	c.self = c
	return c, nil
}

func (c *ExprFreeVars) open() error {
	if c == nil || c.self != c || c.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *ExprFreeVars) Reset() error {
	if e := c.open(); e != nil {
		return e
	}
	clear(c.marked)
	c.sorts = c.sorts[:0]
	return nil
}

func (c *ExprFreeVars) Collect(root ast.ASTHandle) error {
	if e := c.Reset(); e != nil {
		return e
	}
	return c.Accumulate(root)
}

type exprFreeVarWork struct {
	node    ast.ASTHandle
	offset  uint32
	visited map[ast.ASTHandle]struct{}
}

func (c *ExprFreeVars) Accumulate(root ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	work := []exprFreeVarWork{{node: root, visited: c.marked}}
	for len(work) != 0 {
		i := len(work) - 1
		item := work[i]
		work = work[:i]
		if _, found := item.visited[item.node]; found {
			continue
		}
		item.visited[item.node] = struct{}{}
		kind, e := c.manager.GetKind(item.node)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTQuantifier:
			n, e := c.manager.GetNumDeclarations(item.node)
			if e != nil || n > ^uint32(0)-item.offset {
				if e != nil {
					return e
				}
				return ast.ErrASTDomain
			}
			body, e := c.manager.GetQuantifierBody(item.node)
			if e != nil {
				return e
			}
			work = append(work, exprFreeVarWork{node: body, offset: item.offset + n, visited: make(map[ast.ASTHandle]struct{})})
		case ast.ASTVar:
			index, e := c.manager.GetVarIndex(item.node)
			if e != nil {
				return e
			}
			if index < item.offset {
				continue
			}
			index -= item.offset
			if uint64(index)+1 > uint64(^uint(0)>>1) {
				return ast.ErrASTDomain
			}
			if int(index) >= len(c.sorts) {
				c.sorts = append(c.sorts, make([]ast.ASTHandle, int(index)+1-len(c.sorts))...)
			}
			sort, e := c.manager.GetSort(item.node)
			if e != nil {
				return e
			}
			if !c.sorts[index].IsNull() && c.sorts[index] != sort {
				if ast.ASTSourceDebug {
					return diagnostic.ErrDebugAssertion
				}
				continue
			}
			c.sorts[index] = sort
		case ast.ASTApp:
			n, e := c.manager.GetNumArgs(item.node)
			if e != nil {
				return e
			}
			for j := uint32(0); j < n; j++ {
				arg, e := c.manager.GetArg(item.node, j)
				if e != nil {
					return e
				}
				work = append(work, exprFreeVarWork{node: arg, offset: item.offset, visited: item.visited})
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (c *ExprFreeVars) Empty() bool { return c == nil || len(c.sorts) == 0 }
func (c *ExprFreeVars) Size() uint32 {
	if c == nil {
		return 0
	}
	return uint32(len(c.sorts))
}
func (c *ExprFreeVars) Get(index uint32) (ast.ASTHandle, error) {
	if e := c.open(); e != nil || uint64(index) >= uint64(len(c.sorts)) {
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.sorts[index], nil
}
func (c *ExprFreeVars) Contains(index uint32) bool {
	return c != nil && uint64(index) < uint64(len(c.sorts)) && !c.sorts[index].IsNull()
}
func (c *ExprFreeVars) SetDefaultSort(sort ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	for i := range c.sorts {
		if c.sorts[i].IsNull() {
			c.sorts[i] = sort
		}
	}
	return nil
}
func (c *ExprFreeVars) Reverse() error {
	if e := c.open(); e != nil {
		return e
	}
	for i, j := 0, len(c.sorts)-1; i < j; i, j = i+1, j-1 {
		c.sorts[i], c.sorts[j] = c.sorts[j], c.sorts[i]
	}
	return nil
}
func (c *ExprFreeVars) Sorts() []ast.ASTHandle {
	if c == nil || c.self != c {
		return nil
	}
	return append([]ast.ASTHandle(nil), c.sorts...)
}
