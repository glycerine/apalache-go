package smtcore

// Source: src/smt/smt_model_finder.cpp simple projections and witness constraints.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (g *smtModelFinderAUFGraph) exceptionValues(node *smtModelFinderAUFNode) ([]ast.ASTHandle, error) {
	var values []ast.ASTHandle
	for _, term := range node.exceptions {
		value, err := g.eval(term)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	for _, other := range node.avoid {
		root := other.root()
		if root.mono || root.elseValue.IsNull() {
			continue
		}
		value, err := g.eval(root.elseValue)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (g *smtModelFinderAUFGraph) pickInstance(node *smtModelFinderAUFNode, exceptions []ast.ASTHandle) (ast.ASTHandle, error) {
	var chosen ast.ASTHandle
	bestGeneration := ^uint32(0)
	for term, generation := range node.set.elements {
		value, err := g.eval(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eligible := true
		for _, exception := range exceptions {
			distinct, err := g.finder.manager.AreDistinct(value, exception)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !distinct {
				eligible = false
				break
			}
		}
		if eligible && (chosen.IsNull() || generation < bestGeneration) {
			chosen, bestGeneration = term, generation
		}
	}
	return chosen, nil
}

func (g *smtModelFinderAUFGraph) witness(sort ast.ASTHandle, exceptions []ast.ASTHandle) (ast.ASTHandle, error) {
	m := g.finder.manager
	if g.witnesses == nil {
		g.witnesses = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	witness := g.witnesses[sort]
	if witness.IsNull() {
		var err error
		witness, err = m.MkFreshConst("k", sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(witness)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		g.refs = append(g.refs, ref)
		declaration, err := m.GetDecl(witness)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = g.model.RegisterAuxDeclaration(declaration); err != nil {
			return ast.ASTHandle{}, err
		}
		g.witnesses[sort] = witness
	}
	declaration, err := m.GetDecl(witness)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := g.model.Core().ConstantInterpretation(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if value.IsNull() {
		value, err = g.model.FreshValue(sort)
		if err != nil || value.IsNull() {
			return ast.ASTHandle{}, err
		}
		if err = g.model.Core().RegisterConstant(declaration, value); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for _, exception := range exceptions {
		exceptionValue, err := g.eval(exception)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		distinct, err := m.AreDistinct(value, exceptionValue)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if distinct {
			continue
		}
		equality, err := m.MkEq(witness, exception)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		constraint, err := m.MkNot(equality)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(constraint)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		g.finder.pendingConstraints = append(g.finder.pendingConstraints, ref)
	}
	return witness, nil
}

func (g *smtModelFinderAUFGraph) setSimpleProjectionElse(node *smtModelFinderAUFNode) error {
	if len(node.set.elements) == 0 {
		return nil
	}
	if len(node.exceptions) != 0 || len(node.avoid) != 0 {
		values, err := g.exceptionValues(node)
		if err != nil {
			return err
		}
		term, err := g.pickInstance(node, values)
		if err != nil {
			return err
		}
		if !term.IsNull() {
			return node.setElse(term)
		}
		uninterpreted, err := g.finder.manager.IsUninterpretedSort(node.sort)
		if err != nil {
			return err
		}
		size, err := g.finder.manager.GetSortSize(node.sort)
		if err != nil {
			return err
		}
		if !uninterpreted && size.IsInfinite() {
			term, err := g.witness(node.sort, node.exceptions)
			if err != nil {
				return err
			}
			if !term.IsNull() {
				value, err := g.eval(term)
				if err != nil {
					return err
				}
				_, _, numeral, err := g.finder.manager.ArithmeticNumeralText(value)
				if err != nil {
					return err
				}
				if numeral {
					safe := true
					for _, exception := range values {
						distinct, err := g.finder.manager.AreDistinct(value, exception)
						if err != nil {
							return err
						}
						safe = safe && distinct
					}
					if safe {
						// A public numeral preserves this witness value when an
						// instance enters the context. A fresh constant could be
						// reassigned to an exception in the next candidate model.
						term = value
					}
				}
				if _, err = node.insert(term, 0); err != nil {
					return err
				}
				return node.setElse(term)
			}
		}
	}
	for term := range node.set.elements {
		return node.setElse(term)
	}
	return nil
}

func (g *smtModelFinderAUFGraph) makeSimpleProjection(node *smtModelFinderAUFNode) error {
	if node == nil || node.parent != nil || node.mono || node.set == nil || g.model == nil {
		return ast.ErrASTDomain
	}
	if err := g.setSimpleProjectionElse(node); err != nil {
		return err
	}
	values, err := g.instantiationValues(node)
	if err != nil {
		return err
	}
	m := g.finder.manager
	function, err := m.MkFreshFuncDecl(symbol.Symbol{}, symbol.Symbol{}, []ast.ASTHandle{node.sort}, node.sort)
	if err != nil {
		return err
	}
	interpretation, err := model.NewFunctionInterpretation(m, 1)
	if err != nil {
		return err
	}
	fail := func(err error) error { return errors.Join(err, interpretation.Close()) }
	if !node.elseValue.IsNull() {
		value, err := g.eval(node.elseValue)
		if err != nil {
			return fail(err)
		}
		if err = interpretation.SetElse(value); err != nil {
			return fail(err)
		}
	}
	for _, value := range values {
		if err = interpretation.InsertEntry([]ast.ASTHandle{value}, value); err != nil {
			return fail(err)
		}
	}
	if err = g.model.RegisterAuxFunction(function, interpretation); err != nil {
		return fail(err)
	}
	return node.setProjection(function)
}

// Restart applies model-finder witness constraints only after model construction,
// matching model_finder::restart_eh. The internalizer owns asserted expressions.
func (f *SMTModelFinder) Restart() error {
	if err := f.open(); err != nil {
		return err
	}
	for len(f.pendingConstraints) != 0 {
		ref := &f.pendingConstraints[0]
		constraint, err := f.manager.RefGet(*ref)
		if err != nil {
			return err
		}
		if err = f.state.Internalize(constraint, true); err != nil {
			return err
		}
		literal, err := f.state.Literal(constraint)
		if err != nil {
			return err
		}
		if variable := literal.Var(); variable >= 0 {
			if err = f.state.context.SetRelevant(f.state.boolExpr[variable], true); err != nil {
				return err
			}
		}
		if err = f.state.assignLiteral(literal); err != nil {
			return err
		}
		if err = f.manager.DelRef(ref); err != nil {
			return err
		}
		f.pendingConstraints = f.pendingConstraints[1:]
	}
	return nil
}
