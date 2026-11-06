package api

// Source: src/api/api_goal.h:25-30; src/api/api_goal.cpp:35-37.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// The original Z3_goal_ref wrapper retains a goal separately from the API
// object's reference count. Its base is cold; goal formulas remain scalar ASTs.

type apiGoalObject struct {
	object             apiObject
	goal, retiringGoal *tactic.Goal
}

func newAPIGoalObject(c *apiContextResults) (*apiGoalObject, error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(apiGoalObject{}).Size())
	if e := c.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	g := &apiGoalObject{}
	c.traceGoalAllocated(g, n)
	g.object = apiObject{context: c, destructor: g, ownerBytes: n}
	g.object.self = &g.object
	id, e := c.addObject(&g.object)
	if e != nil {
		c.manager.Budget().Release(n)
		return nil, e
	}
	g.object.id = id
	return g, nil
}

func (g *apiGoalObject) setGoal(value *tactic.Goal) error {
	if e := g.object.open(); e != nil {
		return e
	}
	if value != nil {
		if value.Manager() != g.object.context.manager {
			return ast.ErrASTHandle
		}
		if e := value.IncRef(); e != nil {
			return e
		}
	}
	if g.goal != nil {
		if e := g.goal.DecRef(); e != nil {
			return e
		}
	}
	g.goal = value
	return nil
}

func (g *apiGoalObject) apiDestroy() error {
	if g.retiringGoal != nil {
		if e := g.retiringGoal.Close(); e != nil {
			return e
		}
		g.retiringGoal = nil
	}
	if g.goal != nil {
		value := g.goal
		g.goal = nil
		if e := value.DecRef(); e != nil {
			g.retiringGoal = value
			return e
		}
	}
	return nil
}
