package smtcore

// Source: src/smt/theory_seq.cpp canonize and expand1.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Every recursive result owns its expression until its parent has adopted it.
// Expansion consults the scoped solution map, never unexplained egraph roots.
func (t *SMTSequenceTheory) expandSequenceExpression(expression ast.ASTHandle, dependency *smtSequenceDependency) (out ast.ASTRef, updated *smtSequenceDependency, result error) {
	state, err := t.State()
	if err != nil {
		return out, dependency, err
	}
	m := state.manager
	representative, evidence, err := t.solutions.Find(expression)
	if err != nil {
		return out, dependency, err
	}
	dependency = t.dependencies.Join(dependency, evidence)
	if representative != expression {
		return t.expandSequenceExpression(representative, dependency)
	}
	nodeKind, err := m.GetKind(expression)
	if err != nil {
		return out, dependency, err
	}
	if nodeKind != ast.ASTApp {
		out, err = m.NewRef(expression)
		return out, dependency, err
	}
	family, err := m.GetFamilyID(expression)
	if err != nil {
		return out, dependency, err
	}
	kind, err := m.GetDeclKind(expression)
	if err != nil {
		return out, dependency, err
	}
	if family == 0 && kind == ast.ASTOpIte {
		condition, err := m.GetArg(expression, 0)
		if err != nil {
			return out, dependency, err
		}
		for i := uint32(1); i <= 2; i++ {
			branch, err := m.GetArg(expression, i)
			if err != nil {
				return out, dependency, err
			}
			node, branchNode := state.context.enodes[expression], state.context.enodes[branch]
			if node != nil && branchNode != nil && node.Root() == branchNode.Root() {
				dependency = t.dependencies.Join(dependency, t.dependencies.Leaf(smtSequenceAssumption{left: node, right: branchNode, literal: SMTNullLiteral}))
				return t.expandSequenceExpression(branch, dependency)
			}
		}
		if err = state.Internalize(condition, true); err != nil {
			return out, dependency, err
		}
		literal, err := state.Literal(condition)
		if err != nil {
			return out, dependency, err
		}
		value, err := state.Assignment(literal)
		if err != nil {
			return out, dependency, err
		}
		if value != lbool.Undef {
			index := uint32(1)
			if value == lbool.False {
				index, literal = 2, literal.Negated()
			}
			dependency = t.dependencies.Join(dependency, t.dependencies.Leaf(smtSequenceAssumption{literal: literal}))
			branch, err := m.GetArg(expression, index)
			if err != nil {
				return out, dependency, err
			}
			return t.expandSequenceExpression(branch, dependency)
		}
	}
	if family == t.FamilyID() && kind == 27 {
		return t.expandSequenceIntegerString(expression, dependency)
	}
	if family != t.FamilyID() || (kind != ast.ASTSequenceOpConcat && kind != ast.ASTSequenceOpUnit && kind != 3 && kind != 4 && kind != 5 && kind != 10) {
		out, err = m.NewRef(expression)
		return out, dependency, err
	}
	count, err := m.GetNumArgs(expression)
	if err != nil {
		return out, dependency, err
	}
	arguments := make([]ast.ASTHandle, count)
	refs := make([]ast.ASTRef, 0, count)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	for i := range arguments {
		argument, err := m.GetArg(expression, uint32(i))
		if err != nil {
			return out, dependency, err
		}
		// Native index expansion leaves the offset unchanged.
		if kind == 10 && i == 2 {
			arguments[i] = argument
			continue
		}
		ref, next, err := t.expandSequenceExpression(argument, dependency)
		if err != nil {
			return out, dependency, err
		}
		refs = append(refs, ref)
		dependency = next
		arguments[i], err = m.RefGet(ref)
		if err != nil {
			return out, dependency, err
		}
	}
	decl, err := m.GetDecl(expression)
	if err != nil {
		return out, dependency, err
	}
	expanded, err := m.MkApp(decl, arguments)
	if err != nil {
		return out, dependency, err
	}
	out, err = m.NewRef(expanded)
	return out, dependency, err
}

// Source get_num_value and expand1's itos branch. A numerical model value is
// guarded by an equality; only a merged numeral supplies expansion evidence.
func (t *SMTSequenceTheory) expandSequenceIntegerString(expression ast.ASTHandle, dependency *smtSequenceDependency) (out ast.ASTRef, updated *smtSequenceDependency, result error) {
	state := t.state
	m := state.manager
	argument, _, err := m.SequenceIntToStringArgument(expression)
	if err != nil {
		return out, dependency, err
	}
	node := state.context.enodes[argument]
	var value *big.Rat
	if node != nil {
		family, err := m.ArithmeticFamilyID()
		if err != nil {
			return out, dependency, err
		}
		arithmetic, _ := state.Theory(family).(*SMTLinearArithmeticTheory)
		for current := node; ; current = current.Next() {
			text, integer, numeral, err := m.ArithmeticNumeralText(current.Owner())
			if err != nil {
				return out, dependency, err
			}
			if numeral && integer {
				value, _ = new(big.Rat).SetString(text)
			} else if arithmetic != nil && arithmetic.solver != nil && arithmetic.IsAttachedToVariable(current) {
				variable := current.TheoryVar(family)
				column := arithmetic.simplexVariables[variable]
				if column < arithmetic.solver.NumVariables() {
					if err = arithmetic.computeModelDelta(); err != nil {
						return out, dependency, err
					}
					candidate := arithmetic.arithmeticColumnValue(column)
					if candidate.IsInt() {
						value = candidate
					}
				}
			}
			if value != nil || current.Next() == node {
				break
			}
		}
	}
	if value == nil || value.Sign() < 0 {
		out, err = m.NewRef(expression)
		return out, dependency, err
	}
	numeral, err := m.ArithmeticNumeralRat(value, true)
	if err != nil {
		return out, dependency, err
	}
	numeralRef, err := m.NewRef(numeral)
	if err != nil {
		return out, dependency, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&numeralRef)) }()
	if err = state.Internalize(numeral, false); err != nil {
		return out, dependency, err
	}
	text, err := m.MkString(value.Num().String())
	if err != nil {
		return out, dependency, err
	}
	textRef, err := m.NewRef(text)
	if err != nil {
		return out, dependency, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&textRef)) }()
	numeralNode := state.context.enodes[numeral]
	if numeralNode.Root() == node.Root() {
		dependency = t.dependencies.Join(dependency, t.dependencies.Leaf(smtSequenceAssumption{left: numeralNode, right: node, literal: SMTNullLiteral}))
		out, err = m.NewRef(text)
		return out, dependency, err
	}
	numeralEquality, err := m.MkEq(numeral, argument)
	if err != nil {
		return out, dependency, err
	}
	equalityRef, err := m.NewRef(numeralEquality)
	if err != nil {
		return out, dependency, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&equalityRef)) }()
	stringEquality, err := m.MkEq(expression, text)
	if err != nil {
		return out, dependency, err
	}
	stringEqualityRef, err := m.NewRef(stringEquality)
	if err != nil {
		return out, dependency, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&stringEqualityRef)) }()
	if err = t.pushSequenceImplication([]ast.ASTHandle{numeralEquality}, stringEquality); err != nil {
		return out, dependency, err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{stringEquality}, numeralEquality); err != nil {
		return out, dependency, err
	}
	out, err = m.NewRef(expression)
	return out, dependency, err
}

func (t *SMTSequenceTheory) canonizeSequenceExpression(expression ast.ASTHandle, dependency *smtSequenceDependency) (out ast.ASTRef, updated *smtSequenceDependency, result error) {
	out, updated, result = t.expandSequenceExpression(expression, dependency)
	if result != nil {
		if out != (ast.ASTRef{}) {
			result = errors.Join(result, t.state.manager.DelRef(&out))
		}
		return out, updated, result
	}
	rw, err := t.equalityRewriter()
	if err == nil {
		expanded, getErr := t.state.manager.RefGet(out)
		err = getErr
		if err == nil {
			err = rw.Rewrite(expanded, out)
		}
	}
	if err != nil {
		return ast.ASTRef{}, updated, errors.Join(err, t.state.manager.DelRef(&out))
	}
	return out, updated, nil
}
