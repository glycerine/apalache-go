package protomodel

// Source: src/smt/theory_fpa.h fpa_value_factory.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// FloatingPointFactory supplies numeric zero and one for FP sorts.
type FloatingPointFactory struct {
	manager *ast.ASTManager
	family  ast.FamilyID
}

func NewFloatingPointFactory(manager *ast.ASTManager, family ast.FamilyID) (*FloatingPointFactory, error) {
	if manager == nil || family == ast.NullFamilyID {
		return nil, ast.ErrASTDomain
	}
	return &FloatingPointFactory{manager: manager, family: family}, nil
}

func (f *FloatingPointFactory) FamilyID() ast.FamilyID { return f.family }

func (f *FloatingPointFactory) SomeValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if f == nil || f.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return f.manager.MkFPAModelFactoryValue(sort)
}

func (f *FloatingPointFactory) SomeValues(sort ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if f == nil || f.manager == nil {
		return false, ast.ErrASTHandle
	}
	err := f.manager.MkFPAModelFactoryPair(sort, first, second)
	return err == nil, err
}

// Source get_fresh_value is a fatal NOT_IMPLEMENTED_YET, not a null value.
func (*FloatingPointFactory) FreshValue(ast.ASTHandle) (ast.ASTHandle, error) {
	return ast.ASTHandle{}, diagnostic.BorrowProcessDebugContext().NotImplementedYet("src/smt/theory_fpa.h", 65)
}

func (*FloatingPointFactory) RegisterValue(ast.ASTHandle) error { return nil }
func (f *FloatingPointFactory) Close() error {
	if f != nil {
		f.manager = nil
	}
	return nil
}
