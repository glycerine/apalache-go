package smtcore

// Source: src/smt/theory_pb.cpp pb_factory and init_model. The source PB
// build_models hook is false; Boolean roots use assignment-value wrappers
// before family dispatch in smt_model_generator.cpp, not pb_model_value_proc.

import "github.com/glycerine/g3/pkg/ast"

type smtPBValueFactory struct {
	manager *ast.ASTManager
	family  ast.FamilyID
}

func (f *smtPBValueFactory) FamilyID() ast.FamilyID { return f.family }

// The source ignores the requested sort and supplies canonical Booleans.
func (f *smtPBValueFactory) SomeValue(ast.ASTHandle) (ast.ASTHandle, error) {
	if f == nil || f.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return f.manager.MkTrue()
}

func (f *smtPBValueFactory) SomeValues(_ ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if f == nil || f.manager == nil {
		return false, ast.ErrASTHandle
	}
	value, err := f.manager.MkTrue()
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(first, value); err != nil {
		return false, err
	}
	value, err = f.manager.MkFalse()
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(second, value); err != nil {
		return false, err
	}
	return true, nil
}

func (*smtPBValueFactory) FreshValue(ast.ASTHandle) (ast.ASTHandle, error) {
	return ast.ASTHandle{}, nil
}
func (*smtPBValueFactory) RegisterValue(ast.ASTHandle) error { return nil }
func (f *smtPBValueFactory) Close() error {
	if f != nil {
		f.manager = nil
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) InitializeModel(generator *SMTModelGenerator) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if generator == nil || generator.manager != s.manager {
		return ast.ErrASTDomain
	}
	if err = generator.open(); err != nil {
		return err
	}
	return generator.RegisterFactory(&smtPBValueFactory{manager: s.manager, family: t.FamilyID()})
}
