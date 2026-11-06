package protomodel

import (
	"github.com/glycerine/g3/pkg/ast"
)

// ValueFactory is the concrete theory-value interface used while constructing
// an SMT model.
type ValueFactory interface {
	FamilyID() ast.FamilyID
	SomeValue(ast.ASTHandle) (ast.ASTHandle, error)
	// SomeValues assigns caller-owned outputs at their source publication points.
	// The caller retains both reference objects on success, refusal, and failure.
	SomeValues(ast.ASTHandle, ast.ASTRef, ast.ASTRef) (bool, error)
	FreshValue(ast.ASTHandle) (ast.ASTHandle, error)
	RegisterValue(ast.ASTHandle) error
	Close() error
}

type basicFactory struct {
	manager *ast.ASTManager
	family  ast.FamilyID
}

func newBasicFactory(m *ast.ASTManager) *basicFactory {
	return &basicFactory{manager: m, family: m.BasicFamilyID()}
}
func (f *basicFactory) FamilyID() ast.FamilyID { return f.family }
func (f *basicFactory) SomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := f.manager.GetFamilyID(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family != f.manager.BasicFamilyID() {
		return ast.ASTHandle{}, nil
	}
	k, err := f.manager.GetDeclKind(s)
	if err != nil || k != 0 {
		return ast.ASTHandle{}, err
	}
	return f.manager.MkFalse()
}
func (f *basicFactory) SomeValues(s ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	value, err := f.SomeValue(s)
	if err != nil || value.IsNull() {
		return false, err
	}
	if err = f.manager.SetRef(first, value); err != nil {
		return false, err
	}
	value, err = f.manager.MkTrue()
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(second, value); err != nil {
		return false, err
	}
	return true, nil
}
func (f *basicFactory) FreshValue(ast.ASTHandle) (ast.ASTHandle, error) {
	return ast.ASTHandle{}, nil
}
func (f *basicFactory) RegisterValue(ast.ASTHandle) error { return nil }
func (f *basicFactory) Close() error                      { return nil }

// Structured factories borrow the same concrete handle collection shape;
// their enumeration policy is separate from numeric/unsigned simple_factory.
type factoryValueSet struct {
	values []ast.ASTHandle
	seen   map[ast.ASTHandle]struct{}
}
