package duality

import "github.com/glycerine/g3/pkg/ast"

// backgroundLiteralProofs preserves the asserted axioms and derives each
// conjunction member by and-elimination, with all proof references owned by
// the caller until the resulting hyper-resolution node has acquired them.
func (g *RPFP) backgroundLiteralProofs(background []*Expr, refs *[]ast.ASTRef) (map[ast.ASTHandle]ast.ASTHandle, error) {
	m := g.context.manager
	result := make(map[ast.ASTHandle]ast.ASTHandle)
	var visit func(ast.ASTHandle, ast.ASTHandle) error
	visit = func(formula, proof ast.ASTHandle) error {
		result[formula] = proof
		family, err := m.GetFamilyID(formula)
		if err != nil {
			return err
		}
		op, err := m.GetDeclKind(formula)
		if err != nil {
			return err
		}
		if family != m.BasicFamilyID() || op != ast.ASTOpAnd {
			return nil
		}
		n, err := m.GetNumArgs(formula)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			child, err := m.GetArg(formula, i)
			if err != nil {
				return err
			}
			derived, err := m.MkProofAndElim(proof, i)
			if err != nil {
				return err
			}
			ref, err := m.NewRef(derived)
			if err != nil {
				return err
			}
			*refs = append(*refs, ref)
			if err := visit(child, derived); err != nil {
				return err
			}
		}
		return nil
	}
	for _, axiom := range background {
		if err := axiom.open(); err != nil {
			return nil, err
		}
		if axiom.value.object.context != g.context {
			return nil, ErrContextMismatch
		}
		h, err := axiom.Raw()
		if err != nil {
			return nil, err
		}
		proof, err := m.MkProofAsserted(h)
		if err != nil {
			return nil, err
		}
		ref, err := m.NewRef(proof)
		if err != nil {
			return nil, err
		}
		*refs = append(*refs, ref)
		if err := visit(h, proof); err != nil {
			return nil, err
		}
	}
	return result, nil
}
