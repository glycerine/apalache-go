package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type forwardTrigPair struct {
	theta    ast.ASTHandle
	thetaRef ast.ASTRef
	sin, cos ast.ASTRef
}
type forwardTrigEntry struct{ term, fresh, proof ast.ASTRef }
type forwardTrigInvocation struct {
	owner                  *ForwardTrigPurifyTactic
	goal                   *tactic.Goal
	proofs, models         bool
	unsafe                 map[ast.ASTHandle]bool
	pairs                  map[ast.ASTHandle]*forwardTrigPair
	pairOrder              []*forwardTrigPair
	cache                  map[ast.ASTHandle]*forwardTrigEntry
	entries                []*forwardTrigEntry
	constraints, proofRefs []ast.ASTRef
}

func newForwardTrigInvocation(t *ForwardTrigPurifyTactic, g *tactic.Goal) (*forwardTrigInvocation, error) {
	p, e := g.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	models, e := g.ModelsEnabled()
	if e != nil {
		return nil, e
	}
	return &forwardTrigInvocation{owner: t, goal: g, proofs: p, models: models, unsafe: make(map[ast.ASTHandle]bool), pairs: make(map[ast.ASTHandle]*forwardTrigPair), cache: make(map[ast.ASTHandle]*forwardTrigEntry)}, nil
}
func (i *forwardTrigInvocation) close() (err error) {
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
	for j := len(i.pairOrder) - 1; j >= 0; j-- {
		p := i.pairOrder[j]
		err = errors.Join(err, m.DelRef(&p.cos), m.DelRef(&p.sin), m.DelRef(&p.thetaRef))
	}
	return err
}

type forwardTrigTemps struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (t *forwardTrigTemps) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, e := t.manager.NewRef(h)
	if e == nil {
		t.refs = append(t.refs, r)
	}
	return h, e
}
func (t *forwardTrigTemps) close() (err error) {
	for j := len(t.refs) - 1; j >= 0; j-- {
		err = errors.Join(err, t.manager.DelRef(&t.refs[j]))
	}
	return err
}
