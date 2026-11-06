package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type rootObjectEntry struct{ term, fresh, proof ast.ASTRef }
type rootObjectInvocation struct {
	owner                  *RootObjectPurifyTactic
	goal                   *tactic.Goal
	proofs, models         bool
	cache                  map[ast.ASTHandle]*rootObjectEntry
	entries                []*rootObjectEntry
	constraints, proofRefs []ast.ASTRef
	freshDecls             []ast.ASTHandle
}

func newRootObjectInvocation(t *RootObjectPurifyTactic, g *tactic.Goal) (*rootObjectInvocation, error) {
	p, e := g.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	models, e := g.ModelsEnabled()
	if e != nil {
		return nil, e
	}
	return &rootObjectInvocation{owner: t, goal: g, proofs: p, models: models, cache: make(map[ast.ASTHandle]*rootObjectEntry)}, nil
}
func (i *rootObjectInvocation) close() (err error) {
	m := i.owner.manager
	for j := len(i.proofRefs) - 1; j >= 0; j-- {
		err = errors.Join(err, m.DelRef(&i.proofRefs[j]))
	}
	for j := len(i.constraints) - 1; j >= 0; j-- {
		err = errors.Join(err, m.DelRef(&i.constraints[j]))
	}
	for j := len(i.entries) - 1; j >= 0; j-- {
		e := i.entries[j]
		if e.proof != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&e.proof))
		}
		err = errors.Join(err, m.DelRef(&e.fresh), m.DelRef(&e.term))
	}
	return err
}

type rootObjectTemps struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (t *rootObjectTemps) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, e := t.manager.NewRef(h)
	if e == nil {
		t.refs = append(t.refs, r)
	}
	return h, e
}
func (t *rootObjectTemps) close() (err error) {
	for j := len(t.refs) - 1; j >= 0; j-- {
		err = errors.Join(err, t.manager.DelRef(&t.refs[j]))
	}
	return err
}
