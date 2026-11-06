package smtcore

// Source: src/smt/smt_model_finder.cpp get_auf_arrays and select_var qinfo.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtModelFinderSelectVar struct {
	quantifier, selectTerm ast.ASTHandle
	position, variable     uint32
}

func (g *smtModelFinderAUFGraph) collectSelectVars(quantifier, term ast.ASTHandle) error {
	classifier := smtModelFinderAUFClassifier{finder: g.finder}
	selected, err := classifier.isAUFSelect(term)
	if err != nil || !selected {
		return err
	}
	m := g.finder.manager
	n, err := m.GetNumArgs(term)
	if err != nil {
		return err
	}
	for position := uint32(1); position < n; position++ {
		argument, err := m.GetArg(term, position)
		if err != nil {
			return err
		}
		kind, err := m.GetKind(argument)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, err := m.GetVarIndex(argument)
		if err != nil {
			return err
		}
		g.selects = append(g.selects, smtModelFinderSelectVar{quantifier: quantifier, selectTerm: term, position: position, variable: variable})
	}
	return nil
}

func (g *smtModelFinderAUFGraph) matchingArrays(array ast.ASTHandle) ([]*SMTEnode, error) {
	m, context := g.finder.manager, g.finder.state.context
	ground, err := m.IsGround(array)
	if err != nil {
		return nil, err
	}
	if ground {
		if node := context.enodes[array]; node != nil && g.finder.state.isRelevant(array) {
			return []*SMTEnode{node}, nil
		}
		return nil, nil
	}
	nested, err := m.GetArg(array, 0)
	if err != nil {
		return nil, err
	}
	arrays, err := g.matchingArrays(nested)
	if err != nil {
		return nil, err
	}
	declaration, err := m.GetDecl(array)
	if err != nil {
		return nil, err
	}
	var matches []*SMTEnode
	for _, node := range arrays {
		for _, parent := range node.parents {
			if g.finder.state.isRelevant(parent.owner) && parent.declaration == declaration {
				matches = append(matches, parent)
			}
		}
	}
	return matches, nil
}

func (g *smtModelFinderAUFGraph) arrayFunction(array ast.ASTHandle) (ast.ASTHandle, error) {
	if g.model == nil {
		return ast.ASTHandle{}, nil
	}
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = g.model.Eval(array, ref, false)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
	}
	g.refs = append(g.refs, ref)
	value, err := m.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := m.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, err
	}
	arrayFamily, err := m.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := m.GetFamilyID(value)
	if err != nil || family != arrayFamily {
		return ast.ASTHandle{}, err
	}
	op, err := m.GetDeclKind(value)
	if err != nil || op != ast.ASTArrayOpAsArray {
		return ast.ASTHandle{}, err
	}
	return m.AsArrayFunction(value)
}

func (g *smtModelFinderAUFGraph) processSelectVars() error {
	for _, info := range g.selects {
		array, err := g.finder.manager.GetArg(info.selectTerm, 0)
		if err != nil {
			return err
		}
		arrays, err := g.matchingArrays(array)
		if err != nil {
			return err
		}
		variable, err := g.uvar(info.quantifier, info.variable)
		if err != nil {
			return err
		}
		for _, node := range arrays {
			function, err := g.arrayFunction(node.owner)
			if err != nil {
				return err
			}
			if function.IsNull() {
				continue
			}
			argument, err := g.argument(function, info.position-1)
			if err != nil {
				return err
			}
			if err = variable.merge(argument); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) populateSelectVars(quantifier ast.ASTHandle) error {
	m := g.finder.manager
	for _, info := range g.selects {
		if info.quantifier != quantifier {
			continue
		}
		array, err := m.GetArg(info.selectTerm, 0)
		if err != nil {
			return err
		}
		arrays, err := g.matchingArrays(array)
		if err != nil {
			return err
		}
		declaration, err := m.GetDecl(info.selectTerm)
		if err != nil {
			return err
		}
		for _, node := range arrays {
			function, err := g.arrayFunction(node.owner)
			if err != nil {
				return err
			}
			if function.IsNull() {
				continue
			}
			argument := g.arguments[smtModelFinderAUFKey{function, info.position - 1}]
			if argument == nil {
				return ast.ErrASTDomain
			}
			for _, parent := range node.parents {
				if !g.finder.state.isRelevant(parent.owner) || parent.declaration != declaration {
					continue
				}
				if info.position >= uint32(len(parent.arguments)) {
					return ast.ErrASTDomain
				}
				index := parent.arguments[info.position]
				if _, err := argument.insert(index.owner, index.Generation()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
