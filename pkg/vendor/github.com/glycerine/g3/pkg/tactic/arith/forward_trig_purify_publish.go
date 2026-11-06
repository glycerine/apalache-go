package arith

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (i *forwardTrigInvocation) publishConstraints() error {
	for j := range i.constraints {
		h, e := i.owner.manager.RefGet(i.constraints[j])
		if e != nil {
			return e
		}
		proof := ast.ASTHandle{}
		if i.proofs {
			proof, e = i.owner.manager.RefGet(i.proofRefs[j])
			if e != nil {
				return e
			}
		}
		if e = i.goal.AssertWithProof(h, proof, ast.ASTDependency{}); e != nil {
			return e
		}
	}
	return nil
}
