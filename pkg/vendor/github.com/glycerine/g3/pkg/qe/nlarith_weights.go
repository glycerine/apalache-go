package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"math"
)

type NLWeightCache struct {
	manager *ast.ASTManager
	util    *NonlinearArithmeticUtil
	roots   map[ast.ASTHandle]ast.ASTRef
	weights map[ast.ASTHandle]map[ast.ASTHandle]uint32
	closed  bool
}

func NewNLWeightCache(manager *ast.ASTManager, enableLinear bool) (*NLWeightCache, error) {
	u, err := NewNonlinearArithmeticUtil(manager)
	if err != nil {
		return nil, err
	}
	if err = u.SetEnableLinear(enableLinear); err != nil {
		_ = u.Close()
		return nil, err
	}
	return &NLWeightCache{manager: manager, util: u, roots: make(map[ast.ASTHandle]ast.ASTRef), weights: make(map[ast.ASTHandle]map[ast.ASTHandle]uint32)}, nil
}
func (c *NLWeightCache) Weight(variable, formula ast.ASTHandle) (uint32, error) {
	if c == nil || c.closed {
		return 0, ast.ErrASTHandle
	}
	weights := c.weights[formula]
	if weights == nil {
		variables, err := c.util.ExtractNonlinear(formula)
		if err != nil {
			return 0, err
		}
		root, err := c.manager.NewRef(formula)
		if err != nil {
			return 0, err
		}
		weights = make(map[ast.ASTHandle]uint32)
		for _, v := range variables {
			weights[v] = 100
		}
		c.roots[formula] = root
		c.weights[formula] = weights
	}
	if weight, ok := weights[variable]; ok {
		return weight, nil
	}
	return math.MaxUint32, nil
}
func (c *NLWeightCache) Close() (result error) {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for h, r := range c.roots {
		result = errors.Join(result, c.manager.DelRef(&r))
		delete(c.roots, h)
	}
	result = errors.Join(result, c.util.Close())
	if result == nil {
		c.closed = true
		c.weights = nil
	}
	return result
}
