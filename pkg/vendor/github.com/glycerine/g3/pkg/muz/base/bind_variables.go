package base

// Source: src/muz/base/bind_variables.cpp and bind_variables.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrBindVariablesOwner = errors.New("muz/base: invalid or closed variable binder")

// VariableBinder owns the constant applications declared as implicit Datalog
// variables and abstracts them into a forall or exists prefix on demand.
type VariableBinder struct {
	manager    *ast.ASTManager
	variables  []ast.ASTHandle
	references []ast.ASTRef
	closing    bool
	closed     bool
}

func NewVariableBinder(manager *ast.ASTManager) (*VariableBinder, error) {
	if manager == nil {
		return nil, ErrBindVariablesOwner
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &VariableBinder{manager: manager}, nil
}

func (b *VariableBinder) open() error {
	if b == nil || b.closed || b.closing || b.manager == nil {
		return ErrBindVariablesOwner
	}
	return nil
}

// AddVariable retains a zero-arity constant application. Re-adding a variable
// is a no-op, matching the source map's single bound index per application.
func (b *VariableBinder) AddVariable(variable ast.ASTHandle) error {
	if err := b.open(); err != nil {
		return err
	}
	kind, err := b.manager.GetKind(variable)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("muz/base: implicit variable must be a constant application")
	}
	count, err := b.manager.GetNumArgs(variable)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("muz/base: implicit variable must be zero-arity")
	}
	for _, current := range b.variables {
		if current == variable {
			return nil
		}
	}
	reference, err := b.manager.NewRef(variable)
	if err != nil {
		return err
	}
	b.variables = append(b.variables, variable)
	b.references = append(b.references, reference)
	return nil
}

// Bind abstracts registered constants and returns one owning expression
// reference. With no registered variables it returns an owning reference to
// the original expression.
func (b *VariableBinder) Bind(expression ast.ASTHandle, forall bool) (result ast.ASTRef, resultErr error) {
	if err := b.open(); err != nil {
		return result, err
	}
	root, err := b.manager.NewRef(expression)
	if err != nil {
		return result, err
	}
	defer func() {
		if e := b.manager.DelRef(&root); e != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("muz/base: release binding input: %w", e))
		}
	}()
	result, err = b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	failed := true
	defer func() {
		if failed || resultErr != nil {
			if e := b.manager.DelRef(&result); e != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("muz/base: release failed binding result: %w", e))
			}
		}
	}()
	var used []ast.ASTHandle
	if len(b.variables) != 0 {
		used, err = b.usedVariables(expression)
		if err != nil {
			return result, err
		}
	}
	if len(used) == 0 {
		err = b.manager.SetRef(result, expression)
	} else {
		abstractor, e := ast.NewExprAbstractor(b.manager)
		if abstractor != nil {
			defer func() {
				if x := abstractor.Close(); x != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("muz/base: close variable abstractor: %w", x))
				}
			}()
		}
		if e != nil {
			return result, e
		}
		if forall {
			err = abstractor.MkForall(used, expression, result)
		} else {
			err = abstractor.MkExists(used, expression, result)
		}
		if err != nil {
			return result, fmt.Errorf("muz/base: abstract implicit variables: %w", err)
		}
	}
	if err != nil {
		return result, err
	}
	failed = false
	return result, nil
}

func (b *VariableBinder) Close() (result error) {
	if b == nil || b.manager == nil {
		return ErrBindVariablesOwner
	}
	if b.closed {
		return nil
	}
	b.closing = true
	for len(b.references) != 0 {
		last := len(b.references) - 1
		var err error
		if b.references[last] != (ast.ASTRef{}) {
			err = b.manager.DelRef(&b.references[last])
		}
		if b.references[last] == (ast.ASTRef{}) {
			// Native destruction may consume a reference before reporting an
			// error. Never replay that decrement when Close is retried.
			b.references = b.references[:last]
			b.variables = b.variables[:last]
		}
		if err != nil {
			return err
		}
	}
	b.references = nil
	b.variables = nil
	b.closing = false
	b.closed = true
	return nil
}
