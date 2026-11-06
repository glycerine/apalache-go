package rewriter

// Source: src/ast/rewriter/ast_counter.cpp; src/ast/rewriter/ast_counter.h.

import "github.com/glycerine/g3/pkg/ast"

// Counter is the source unsigned-keyed signed counter.
type Counter struct{ data map[uint32]int32 }

func NewCounter() *Counter { return &Counter{data: make(map[uint32]int32)} }

func (c *Counter) ensure() {
	if c.data == nil {
		c.data = make(map[uint32]int32)
	}
}

func (c *Counter) Reset() { c.data = make(map[uint32]int32) }

func (c *Counter) Update(element uint32, delta int32) {
	c.ensure()
	c.data[element] += delta
}

func (c *Counter) Get(element uint32) int32 {
	c.ensure()
	if _, found := c.data[element]; !found {
		c.data[element] = 0
	}
	return c.data[element]
}

func (c *Counter) Set(element uint32, value int32) {
	c.ensure()
	c.data[element] = value
}

func (c *Counter) Entries() map[uint32]int32 {
	result := make(map[uint32]int32, len(c.data))
	for element, count := range c.data {
		result[element] = count
	}
	return result
}

func (c *Counter) Count(elements []uint32, delta int32) *Counter {
	for _, element := range elements {
		c.Update(element, delta)
	}
	return c
}

func (c *Counter) CollectPositive(acc map[uint32]struct{}) {
	for element, count := range c.data {
		if count > 0 {
			acc[element] = struct{}{}
		}
	}
}

func (c *Counter) PositiveCount() uint32 {
	var result uint32
	for _, count := range c.data {
		if count > 0 {
			result++
		}
	}
	return result
}

func (c *Counter) MaxPositive() (uint32, bool) {
	var result uint32
	found := false
	for element, count := range c.data {
		if count > 0 && (!found || element > result) {
			result, found = element, true
		}
	}
	return result, found
}

func (c *Counter) MaxPositiveValue() (uint32, error) {
	result, found := c.MaxPositive()
	if !found {
		return 0, ast.ErrASTDomain
	}
	return result, nil
}

func (c *Counter) MaxCounterValue() int32 {
	var result int32
	for _, count := range c.data {
		if count > result {
			result = count
		}
	}
	return result
}

// VarCounter counts variables in predicate arguments and finds the largest
// free de Bruijn index while accounting for quantifier binders.
type VarCounter struct {
	Counter
	manager *ast.ASTManager
	visited map[ast.ASTHandle]struct{}
	free    *ExprFreeVars
	todo    []ast.ASTHandle
}

func NewVarCounter(m *ast.ASTManager) (*VarCounter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	free, err := NewExprFreeVars(m)
	if err != nil {
		return nil, err
	}
	return &VarCounter{Counter: *NewCounter(), manager: m, visited: make(map[ast.ASTHandle]struct{}), free: free}, nil
}

func (c *VarCounter) CountVars(predicate ast.ASTHandle, coefficient int32) error {
	if c == nil || c.manager == nil || c.free == nil {
		return ast.ErrASTHandle
	}
	kind, err := c.manager.GetKind(predicate)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	n, err := c.manager.GetNumArgs(predicate)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		arg, e := c.manager.GetArg(predicate, i)
		if e != nil {
			return e
		}
		if e = c.free.Collect(arg); e != nil {
			return e
		}
		for j := uint32(0); j < c.free.Size(); j++ {
			if c.free.Contains(j) {
				c.Update(j, coefficient)
			}
		}
	}
	return c.free.Reset()
}

func (c *VarCounter) maxVar(hasVar *bool) (uint32, error) {
	*hasVar = false
	var maxVar uint32
	quantifiers := make([]ast.ASTHandle, 0)
	for len(c.todo) != 0 {
		last := len(c.todo) - 1
		h := c.todo[last]
		c.todo = c.todo[:last]
		if _, found := c.visited[h]; found {
			continue
		}
		c.visited[h] = struct{}{}
		kind, err := c.manager.GetKind(h)
		if err != nil {
			return 0, err
		}
		switch kind {
		case ast.ASTQuantifier:
			quantifiers = append(quantifiers, h)
		case ast.ASTVar:
			index, e := c.manager.GetVarIndex(h)
			if e != nil {
				return 0, e
			}
			if index >= maxVar {
				*hasVar = true
				maxVar = index
			}
		case ast.ASTApp:
			n, e := c.manager.GetNumArgs(h)
			if e != nil {
				return 0, e
			}
			for i := uint32(0); i < n; i++ {
				child, childErr := c.manager.GetArg(h, i)
				if childErr != nil {
					return 0, childErr
				}
				c.todo = append(c.todo, child)
			}
		default:
			return 0, ast.ErrASTDomain
		}
	}
	clear(c.visited)

	for len(quantifiers) != 0 {
		last := len(quantifiers) - 1
		q := quantifiers[last]
		quantifiers = quantifiers[:last]
		aux, err := NewVarCounter(c.manager)
		if err != nil {
			return 0, err
		}
		body, err := c.manager.GetQuantifierBody(q)
		if err != nil {
			return 0, err
		}
		aux.todo = append(aux.todo, body)
		hasNested := false
		nestedMax, err := aux.maxVar(&hasNested)
		if err != nil {
			return 0, err
		}
		declarations, err := c.manager.GetNumDeclarations(q)
		if err != nil {
			return 0, err
		}
		if nestedMax >= maxVar+declarations {
			maxVar = nestedMax - declarations
			*hasVar = *hasVar || hasNested
		}
	}
	return maxVar, nil
}

func (c *VarCounter) MaxVar(expression ast.ASTHandle) (uint32, error) {
	if c == nil || c.manager == nil {
		return 0, ast.ErrASTHandle
	}
	c.todo = append(c.todo, expression)
	hasVar := false
	return c.maxVar(&hasVar)
}

func (c *VarCounter) NextVar(expression ast.ASTHandle) (uint32, error) {
	if c == nil || c.manager == nil {
		return 0, ast.ErrASTHandle
	}
	c.todo = append(c.todo, expression)
	hasVar := false
	result, err := c.maxVar(&hasVar)
	if err == nil && hasVar {
		result++
	}
	return result, err
}

// ASTCounter is the source AST-identity signed counter.
type ASTCounter struct{ data map[ast.ASTHandle]int32 }

func NewASTCounter() *ASTCounter { return &ASTCounter{data: make(map[ast.ASTHandle]int32)} }

func (c *ASTCounter) Get(h ast.ASTHandle) int32 {
	if c.data == nil {
		c.data = make(map[ast.ASTHandle]int32)
	}
	if _, found := c.data[h]; !found {
		c.data[h] = 0
	}
	return c.data[h]
}

func (c *ASTCounter) Update(h ast.ASTHandle, delta int32) {
	if c.data == nil {
		c.data = make(map[ast.ASTHandle]int32)
	}
	c.data[h] += delta
}

func (c *ASTCounter) Set(h ast.ASTHandle, value int32) {
	if c.data == nil {
		c.data = make(map[ast.ASTHandle]int32)
	}
	c.data[h] = value
}

func (c *ASTCounter) Inc(h ast.ASTHandle) { c.Update(h, 1) }
func (c *ASTCounter) Dec(h ast.ASTHandle) { c.Update(h, -1) }
