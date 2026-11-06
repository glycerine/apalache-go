package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (i *rootObjectInvocation) publish(out *tactic.TacticResult) (err error) {
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
	if !i.models || len(i.freshDecls) == 0 {
		return nil
	}
	filter, e := tactic.NewFilterModel(i.owner.manager)
	if e != nil {
		return e
	}
	if e = filter.IncRef(); e != nil {
		return errors.Join(e, filter.Close())
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	for _, d := range i.freshDecls {
		if e = filter.Insert(d); e != nil {
			return e
		}
	}
	return out.SetModelConverter(filter)
}
