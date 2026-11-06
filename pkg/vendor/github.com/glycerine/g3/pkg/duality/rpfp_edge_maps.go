package duality

// Source: src/duality/duality_rpfp.cpp:264-283 (SetEdgeMaps).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Edge) clearMaps() error {
	e.mapsReady = false
	for key, ref := range e.varMap {
		err := e.Owner.context.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(e.varMap, key)
		} else {
			e.varMap[key] = ref
		}
		if err != nil {
			return err
		}
	}
	e.relMap = nil
	return nil
}

// SetEdgeMaps maps relational parameters to child positions and individual
// parameters to the indexed variables of the parent node.
func (g *RPFP) SetEdgeMaps(edge *Edge) (result error) {
	if err := g.open(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	// A prefix remains owned by the edge if construction or cleanup fails,
	// but Localize must rebuild it before consuming any parameter mapping.
	// This defer runs after the timer and temporary-expression cleanup.
	defer func() { edge.mapsReady = result == nil }()
	timer, err := startProfile(g.context, "SetEdgeMaps")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err := edge.clearMaps(); err != nil {
		return err
	}
	edge.relMap = make(map[ast.ASTHandle]int)
	edge.varMap = make(map[ast.ASTHandle]ast.ASTRef)
	m := g.context.manager
	for n, ref := range edge.F.relationParams {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		edge.relMap[h] = n
	}
	b, vars, err := g.RedVars(edge.Parent)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	if len(edge.F.individualParams) > len(vars) {
		return ast.ErrASTDomain
	}
	for n, ref := range edge.F.individualParams {
		from, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		to, err := vars[n].Raw()
		if err != nil {
			return err
		}
		if old, found := edge.varMap[from]; found {
			if err = m.DelRef(&old); err != nil {
				edge.varMap[from] = old
				return err
			}
			delete(edge.varMap, from)
		}
		owned, err := m.NewRef(to)
		if err != nil {
			return err
		}
		edge.varMap[from] = owned
	}
	return nil
}
