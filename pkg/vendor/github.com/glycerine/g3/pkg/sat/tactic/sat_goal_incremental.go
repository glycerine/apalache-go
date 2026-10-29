package tactic

// Source: src/sat/tactic/goal2sat.cpp:547-559 and atom2bool_var/expr2var.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
)

// NewGoalTranslatorWithAtomMap borrows the actual scoped source mapping. Each
// translation has fresh traversal caches and dependency/interpreted-atom lists;
// the caller keeps mapping alive across translations and owns its push/pop.
// A nonnil partial translator returned with an error must be closed.
func NewGoalTranslatorWithAtomMap(m *ast.ASTManager, s *sat.Solver, mapping *ast.Expr2Var, defaultExternal bool) (*GoalTranslator, error) {
	if mapping == nil {
		return nil, ast.ErrASTHandle
	}
	owner, err := mapping.Manager()
	if err != nil {
		return nil, err
	}
	if owner != m {
		return nil, ast.ErrASTHandle
	}
	tr, err := NewGoalTranslator(m, s)
	if err != nil {
		return tr, err
	}
	tr.sharedAtoms = mapping
	tr.defaultExternal = defaultExternal
	return tr, nil
}
func (tr *GoalTranslator) atomVariable(h ast.ASTHandle) (uint32, bool, error) {
	if tr.sharedAtoms != nil {
		v, err := tr.sharedAtoms.ToVar(h)
		return v, v != ^uint32(0), err // expr2var uses UINT_MAX, not SAT's literal sentinel.
	}
	v, found := tr.Atoms.Get2(h)
	return v, found, nil
}
func (tr *GoalTranslator) insertAtom(h ast.ASTHandle, v uint32) error {
	if tr.sharedAtoms != nil {
		return tr.sharedAtoms.Insert(h, v)
	}
	if err := tr.manager.IncRef(h); err != nil {
		return err
	}
	tr.Atoms.Set(h, v)
	return nil
}
func (tr *GoalTranslator) atomEntries() ([]ast.ExprVarBinding, error) {
	if tr.sharedAtoms != nil {
		return tr.sharedAtoms.Entries()
	}
	entries := make([]ast.ExprVarBinding, 0, tr.Atoms.Len())
	for h, v := range tr.Atoms.All() {
		entries = append(entries, ast.ExprVarBinding{Expr: h, Variable: v})
	}
	return entries, nil
}

// InterpretedAtoms returns borrowed handles discovered by this compilation,
// matching goal2sat::get_interpreted_atoms rather than a cumulative map flag.
func (tr *GoalTranslator) InterpretedAtoms() ([]ast.ASTHandle, error) {
	if err := tr.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), tr.interpreted...), nil
}

type DependencyLiteralBinding struct {
	Expression ast.ASTHandle
	Literal    sat.Literal
}

// DependencyLiterals copies the source dep2asm map with borrowed expression
// keys. The caller retains its original dependencies while using these keys.
func (tr *GoalTranslator) DependencyLiterals() ([]DependencyLiteralBinding, error) {
	if err := tr.open(); err != nil {
		return nil, err
	}
	result := make([]DependencyLiteralBinding, 0, tr.dependencies.Len())
	for h, v := range tr.dependencies.All() {
		result = append(result, DependencyLiteralBinding{h, sat.Literal(v)})
	}
	return result, nil
}
