package base

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Pin first, as in context::register_predicate. A later classifier allocation
// failure may leave a harmless pinned declaration, but must not make it a
// recursive predicate before the context publishes membership.
func (c *DatalogContext) validatePredicate(declaration ast.ASTHandle) error {
	kind, err := c.manager.GetKind(declaration)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return fmt.Errorf("muz/base: predicate is not a function declaration")
	}
	return nil
}

func (c *DatalogContext) pinPredicate(declaration ast.ASTHandle) error {
	if err := c.ruleManager.open(); err != nil {
		return err
	}
	if err := c.validatePredicate(declaration); err != nil {
		return err
	}
	if err := c.retain(declaration); err != nil {
		return err
	}
	if reference, ok := c.inactivePredicates[declaration]; ok {
		if _, active := c.ruleManager.predicates[declaration]; !active {
			c.ruleManager.predicates[declaration] = reference
			delete(c.inactivePredicates, declaration)
			return nil
		}
	}
	return c.ruleManager.RegisterPredicate(declaration)
}

func (c *DatalogContext) RegisterPredicate(declaration ast.ASTHandle, named bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.predicates[declaration] {
		return nil
	}
	var name usymbol.Symbol
	if named {
		var err error
		name, err = c.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
	}
	if err := c.pinPredicate(declaration); err != nil {
		return err
	}
	c.predicates[declaration] = true
	c.predicateOrder = append(c.predicateOrder, declaration)
	if named {
		c.named[name] = declaration
	}
	return nil
}

func (c *DatalogContext) IsPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.predicates[declaration], nil
}
func (c *DatalogContext) Predicate(name usymbol.Symbol) (ast.ASTHandle, bool, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	d, ok := c.named[name]
	return d, ok, nil
}
func (c *DatalogContext) Predicates() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), c.predicateOrder...), nil
}

func (c *DatalogContext) RestrictPredicates(keep map[ast.ASTHandle]bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.ruleManager.open(); err != nil {
		return err
	}
	// Stage ownership without publishing classifier entries. On a later
	// failure, staged references remain parked for retry or context Close.
	var restored []ast.ASTHandle
	for d, present := range keep {
		if present && !c.predicates[d] {
			if err := c.validatePredicate(d); err != nil {
				return err
			}
			if err := c.retain(d); err != nil {
				return err
			}
			if _, active := c.ruleManager.predicates[d]; !active {
				if _, parked := c.inactivePredicates[d]; !parked {
					reference, err := c.manager.NewRef(d)
					if err != nil {
						return err
					}
					if c.inactivePredicates == nil {
						c.inactivePredicates = make(map[ast.ASTHandle]ast.ASTRef)
					}
					c.inactivePredicates[d] = reference
				}
			}
			restored = append(restored, d)
		}
	}
	order := c.predicateOrder[:0]
	for _, d := range c.predicateOrder {
		if keep[d] {
			order = append(order, d)
			continue
		}
		delete(c.predicates, d)
	}
	for d, reference := range c.ruleManager.predicates {
		if keep[d] {
			continue
		}
		if c.inactivePredicates == nil {
			c.inactivePredicates = make(map[ast.ASTHandle]ast.ASTRef)
		}
		if previous, exists := c.inactivePredicates[d]; exists {
			c.references = append(c.references, previous)
		}
		c.inactivePredicates[d] = reference
		delete(c.ruleManager.predicates, d)
	}
	c.predicateOrder = order
	// Scoped queries restore a previously saved set, which can contain
	// declarations no longer in the current membership set. The name table
	// is independent and remains unchanged by this operation.
	for _, d := range restored {
		if reference, ok := c.inactivePredicates[d]; ok {
			if _, active := c.ruleManager.predicates[d]; !active {
				c.ruleManager.predicates[d] = reference
				delete(c.inactivePredicates, d)
			}
		}
		c.predicates[d] = true
		c.predicateOrder = append(c.predicateOrder, d)
	}
	return nil
}

func (c *DatalogContext) FreshHeadPredicate(prefix, suffix usymbol.Symbol, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	b, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := c.manager.MkFreshFuncDecl(prefix, suffix, domain, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.RegisterPredicate(d, true); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}
