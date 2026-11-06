package smtcore

// Source: src/smt/smt_model_finder.cpp cleanup_instantiation_sets,
// add_elem_to_empty_inst_sets, mk_sorts_finite and add_mono_exceptions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *smtModelFinderAUFGraph) eval(term ast.ASTHandle) (ast.ASTHandle, error) {
	if g.model == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = g.model.Eval(term, ref, true)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
	}
	g.refs = append(g.refs, ref)
	return m.RefGet(ref)
}

func (g *smtModelFinderAUFGraph) cleanInstantiationSets() error {
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		for term := range node.set.elements {
			value, err := g.eval(term)
			if err != nil {
				return err
			}
			isValue, err := g.finder.manager.IsValue(value)
			if err != nil {
				return err
			}
			if !isValue {
				if err := node.set.Remove(term); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) fillEmptyInstantiationSets() error {
	m := g.finder.manager
	bySort := make(map[ast.ASTHandle]ast.ASTHandle)
	var fresh []*smtModelFinderAUFNode
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		if len(node.set.elements) != 0 {
			for term := range node.set.elements {
				bySort[node.sort] = term
				break
			}
			continue
		}
		interpreted, err := m.IsFullyInterpreted(node.sort)
		if err != nil {
			return err
		}
		if !interpreted {
			fresh = append(fresh, node)
			continue
		}
		value, err := g.model.SomeValue(node.sort)
		if err != nil {
			return err
		}
		if _, err := node.insert(value, 0); err != nil {
			return err
		}
	}
	for _, node := range fresh {
		term := bySort[node.sort]
		if term.IsNull() {
			var err error
			term, err = m.MkFreshConst("elem", node.sort)
			if err != nil {
				return err
			}
			bySort[node.sort] = term
		}
		// Insert pins the term. Private model values must never escape
		// into instantiations for sorts with an arbitrary interpretation.
		if _, err := node.insert(term, 0); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) freezeUniverses() error {
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		uninterpreted, err := g.finder.manager.IsUninterpretedSort(node.sort)
		if err != nil {
			return err
		}
		if !uninterpreted {
			continue
		}
		finite, err := g.model.IsFinite(node.sort)
		if err != nil {
			return err
		}
		if !finite {
			if err := g.model.FreezeUniverse(node.sort); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) addMonotoneExceptions() error {
	m := g.finder.manager
	for _, node := range g.nodes {
		if node.parent != nil || !node.mono || len(node.exceptions) == 0 {
			continue
		}
		bv, err := m.IsBitVectorSort(node.sort)
		if err != nil {
			return err
		}
		if !bv {
			family, err := m.GetFamilyID(node.sort)
			if err != nil {
				return err
			}
			arithmetic, err := m.ArithmeticFamilyID()
			if err != nil {
				return err
			}
			op, err := m.GetDeclKind(node.sort)
			if err != nil {
				return err
			}
			// Integer sort kind is one. Reals deliberately receive no
			// neighboring exceptions in the source algorithm.
			if family != arithmetic || op != 1 {
				continue
			}
		}
		one, err := m.MkDatalogNumeral(1, node.sort)
		if err != nil {
			return err
		}
		for _, exception := range node.exceptions {
			if err := g.insertShifted(node, exception, one, 0, true); err != nil {
				return err
			}
			if err := g.insertShifted(node, exception, one, 0, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) prepareInstantiationSets() error {
	if err := g.cleanInstantiationSets(); err != nil {
		return err
	}
	if err := g.fillEmptyInstantiationSets(); err != nil {
		return err
	}
	if err := g.freezeUniverses(); err != nil {
		return err
	}
	return g.addMonotoneExceptions()
}
