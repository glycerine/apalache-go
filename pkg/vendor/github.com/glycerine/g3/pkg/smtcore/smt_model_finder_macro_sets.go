package smtcore

// Source: src/smt/smt_model_finder.cpp f_var macro-based population and
// quantifier_info::populate_macro_based_inst_sets.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (f *SMTModelFinder) populateMacroInstantiationSets(quantifier ast.ASTHandle) (result error) {
	chosen := f.chosen[quantifier]
	if chosen.IsNull() {
		return nil
	}
	graph := newSMTModelFinderAUFGraph(f)
	defer func() { result = errors.Join(result, graph.close()) }()
	if err := graph.collectFVars(quantifier); err != nil {
		return err
	}
	flatCount, err := f.manager.GetNumDeclarations(f.flatQuantifier(quantifier))
	if err != nil {
		return err
	}
	originalCount, err := f.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return err
	}
	for _, info := range graph.fvars {
		if info.function != chosen || !info.offset.IsNull() {
			continue
		}
		variable := flatCount - info.variable - 1
		if variable >= originalCount {
			continue
		}
		for _, application := range f.state.context.byDeclaration[chosen] {
			if application == nil || !f.state.isRelevant(application.owner) {
				continue
			}
			if info.position >= uint32(len(application.arguments)) {
				return ast.ErrASTDomain
			}
			argument := application.arguments[info.position]
			if _, err := f.InsertInstantiationTerm(quantifier, variable, argument.owner, argument.Generation()); err != nil {
				return err
			}
		}
	}
	return nil
}
