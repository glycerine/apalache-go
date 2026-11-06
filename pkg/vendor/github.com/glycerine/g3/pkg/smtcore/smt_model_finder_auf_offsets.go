package smtcore

// Source: src/smt/smt_model_finder.cpp f_var_plus_offset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *smtModelFinderAUFGraph) collectOffset(quantifier, function ast.ASTHandle, position uint32, argument ast.ASTHandle) error {
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	inverse, variable, matched, err := g.finder.macroUtil.IsVarPlusGround(argument, ref)
	if err != nil || !matched || inverse {
		return errors.Join(err, m.DelRef(&ref))
	}
	offset, err := m.RefGet(ref)
	if err != nil {
		return errors.Join(err, m.DelRef(&ref))
	}
	index, err := m.GetVarIndex(variable)
	if err != nil {
		return errors.Join(err, m.DelRef(&ref))
	}
	g.refs = append(g.refs, ref)
	g.appendFVar(smtModelFinderFVar{
		quantifier: quantifier, function: function, position: position, variable: index, offset: offset,
	})
	return nil
}

func (g *smtModelFinderAUFGraph) insertShifted(to *smtModelFinderAUFNode, term, offset ast.ASTHandle, generation uint32, plus bool) (result error) {
	var shifted ast.ASTHandle
	var err error
	if plus {
		shifted, err = g.finder.macroUtil.MkAdd(term, offset)
	} else {
		shifted, err = g.finder.macroUtil.MkSub(term, offset)
	}
	if err != nil {
		return err
	}
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	manager := g.finder.state.quantifierManager
	if manager == nil || manager.rewriter == nil {
		return ast.ErrASTHandle
	}
	if err = manager.rewriter.Rewrite(shifted, ref); err != nil {
		return err
	}
	shifted, err = m.RefGet(ref)
	if err != nil {
		return err
	}
	_, err = to.insert(shifted, generation)
	return err
}

func (g *smtModelFinderAUFGraph) populateOffsets(quantifier ast.ASTHandle) error {
	for _, info := range g.fvars {
		if info.quantifier != quantifier || info.offset.IsNull() {
			continue
		}
		argument := g.arguments[smtModelFinderAUFKey{info.function, info.position}].root()
		variable := g.uvars[smtModelFinderAUFKey{info.quantifier, info.variable}].root()
		if argument == variable {
			// Source uses only ground-context arguments for this case: no
			// finite fixed point exists for a nonzero offset within a class.
			continue
		}
		// The second population pass enforces A_f_i-k subset S_j followed
		// by S_j+k subset A_f_i, preserving each source generation.
		for term, generation := range argument.set.elements {
			if err := g.insertShifted(variable, term, info.offset, generation, false); err != nil {
				return err
			}
		}
		for term, generation := range variable.set.elements {
			if err := g.insertShifted(argument, term, info.offset, generation, true); err != nil {
				return err
			}
		}
	}
	return nil
}
