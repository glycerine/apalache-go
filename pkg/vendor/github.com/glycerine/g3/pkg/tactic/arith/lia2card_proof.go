package arith

// Rewriting a bounded integer as a Boolean indicator is valid under the
// original goal, rather than as an unconditional arithmetic rewrite.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func lia2cardOriginalProofs(m *ast.ASTManager, goal *tactic.Goal, size uint32) (refs []ast.ASTRef, err error) {
	defer func() {
		if err != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, m.DelRef(&refs[i]))
			}
			refs = nil
		}
	}()
	for i := uint32(0); i < size; i++ {
		proof, e := goal.Proof(i)
		if e != nil {
			return refs, e
		}
		ref, e := m.NewRef(proof)
		if e != nil {
			return refs, e
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func lia2cardGoalLemma(m *ast.ASTManager, family ast.FamilyID, conclusion ast.ASTHandle, refs []ast.ASTRef) (ast.ASTHandle, error) {
	premises := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		var err error
		premises[i], err = m.RefGet(refs[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.MkProofTheoryLemma(family, conclusion, premises, nil)
}
