package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type NLBranch struct {
	Branch, Constraint, Definition, A, B, C ast.ASTRef
	Substitutions                           []ast.ASTRef
}
type NLBranchConditions struct {
	manager    *ast.ASTManager
	Predicates []ast.ASTRef
	Branches   []*NLBranch
	closed     bool
}

func NewNLBranchConditions(m *ast.ASTManager) (*NLBranchConditions, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &NLBranchConditions{manager: m}, nil
}
func (b *NLBranchConditions) AddPredicate(p ast.ASTHandle) error {
	if b == nil || b.closed {
		return ast.ErrASTHandle
	}
	r, err := b.manager.NewRef(p)
	if err == nil {
		b.Predicates = append(b.Predicates, r)
	}
	return err
}
func (b *NLBranchConditions) AddBranch(branch, constraint ast.ASTHandle, substitutions []ast.ASTHandle, definition, a, bb, c ast.ASTHandle) error {
	if b == nil || b.closed {
		return ast.ErrASTHandle
	}
	values := []ast.ASTHandle{branch, constraint, definition, a, bb, c}
	row := &NLBranch{}
	refs := []*ast.ASTRef{&row.Branch, &row.Constraint, &row.Definition, &row.A, &row.B, &row.C}
	for i := range refs {
		r, err := b.manager.NewRef(values[i])
		if err != nil {
			_ = row.close(b.manager)
			return err
		}
		*refs[i] = r
	}
	for _, h := range substitutions {
		r, err := b.manager.NewRef(h)
		if err != nil {
			_ = row.close(b.manager)
			return err
		}
		row.Substitutions = append(row.Substitutions, r)
	}
	b.Branches = append(b.Branches, row)
	return nil
}
func (r *NLBranch) close(m *ast.ASTManager) (result error) {
	for i := len(r.Substitutions) - 1; i >= 0; i-- {
		result = errors.Join(result, m.DelRef(&r.Substitutions[i]))
	}
	for _, v := range []*ast.ASTRef{&r.C, &r.B, &r.A, &r.Definition, &r.Constraint, &r.Branch} {
		if *v != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(v))
		}
	}
	return result
}
func (b *NLBranchConditions) Reset() (result error) {
	if b == nil || b.closed {
		return ast.ErrASTHandle
	}
	for i := len(b.Branches) - 1; i >= 0; i-- {
		result = errors.Join(result, b.Branches[i].close(b.manager))
	}
	for i := len(b.Predicates) - 1; i >= 0; i-- {
		result = errors.Join(result, b.manager.DelRef(&b.Predicates[i]))
	}
	if result == nil {
		b.Branches = nil
		b.Predicates = nil
	}
	return result
}
func (b *NLBranchConditions) Close() error {
	if b == nil || b.manager == nil {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	err := b.Reset()
	if err == nil {
		b.closed = true
	}
	return err
}
