package core

// Source: src/tactic/core/solve_eqs_tactic.cpp live trivial/ITE/arithmetic and
// Boolean literal solvers. The #if 0 nonlinear path has no retained caller.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type solvedEquation struct{ variable, definition, proof ast.ASTHandle }

func (t *SolveEquationsTactic) op(term ast.ASTHandle, family ast.FamilyID, operation int32) (bool, error) {
	kind, err := t.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	fid, err := t.manager.GetFamilyID(term)
	if err != nil || fid != family {
		return false, err
	}
	op, err := t.manager.GetDeclKind(term)
	return op == operation, err
}
func (t *SolveEquationsTactic) uninterpretedConstant(term ast.ASTHandle) (bool, error) {
	kind, err := t.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := t.manager.GetNumArgs(term)
	if err != nil || count != 0 {
		return false, err
	}
	family, err := t.manager.GetFamilyID(term)
	return family == ast.NullFamilyID, err
}
func (t *SolveEquationsTactic) trivial(left, right ast.ASTHandle) (solvedEquation, bool, error) {
	for i, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		variable, err := t.uninterpretedConstant(pair[0])
		if err != nil {
			return solvedEquation{}, false, err
		}
		if !variable || t.candidateVars[pair[0]] {
			continue
		}
		occurs, err := t.manager.OccursExpr(pair[0], pair[1])
		if err != nil {
			return solvedEquation{}, false, err
		}
		if occurs {
			continue
		}
		acceptable, err := t.checkOccurrences(pair[0])
		if err != nil {
			return solvedEquation{}, false, err
		}
		if !acceptable {
			continue
		}
		solution := solvedEquation{variable: pair[0], definition: pair[1]}
		if i == 1 && t.proofs {
			eq, err := t.manager.MkEq(left, right)
			if err != nil {
				return solution, false, err
			}
			solution.proof, err = t.manager.MkProofCommutativity(eq)
			if err != nil {
				return solution, false, err
			}
		}
		return solution, true, nil
	}
	return solvedEquation{}, false, nil
}

func (t *SolveEquationsTactic) solve(term ast.ASTHandle, frame *solveEquationFrame) (solvedEquation, bool, error) {
	isEq, err := t.op(term, t.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil {
		return solvedEquation{}, false, err
	}
	if isEq {
		left, err := t.manager.GetArg(term, 0)
		if err != nil {
			return solvedEquation{}, false, err
		}
		right, err := t.manager.GetArg(term, 1)
		if err != nil {
			return solvedEquation{}, false, err
		}
		solution, found, err := t.trivial(left, right)
		if err != nil {
			return solution, false, err
		}
		if found {
			if err = frame.pin(solution.proof); err != nil {
				return solution, false, err
			}
			return solution, true, nil
		}
		if t.theorySolver {
			return t.solveArithmetic(left, right, term, frame)
		}
		return solvedEquation{}, false, nil
	}
	iff, err := t.op(term, t.manager.BasicFamilyID(), ast.ASTOpIff)
	if err != nil {
		return solvedEquation{}, false, err
	}
	if iff {
		left, e := t.manager.GetArg(term, 0)
		if e != nil {
			return solvedEquation{}, false, e
		}
		right, e := t.manager.GetArg(term, 1)
		if e != nil {
			return solvedEquation{}, false, e
		}
		solution, found, e := t.trivial(left, right)
		if e == nil && found {
			e = frame.pin(solution.proof)
		}
		return solution, found, e
	}
	ite, err := t.op(term, t.manager.BasicFamilyID(), ast.ASTOpIte)
	if err != nil {
		return solvedEquation{}, false, err
	}
	if ite && t.iteSolver {
		return t.solveITE(term, frame)
	}
	atom := term
	negative, child, err := t.manager.IsNot(term)
	if err != nil {
		return solvedEquation{}, false, err
	}
	if negative {
		atom = child
	}
	constant, err := t.uninterpretedConstant(atom)
	if err != nil {
		return solvedEquation{}, false, err
	}
	if !constant || t.candidateVars[atom] {
		return solvedEquation{}, false, nil
	}
	definition, err := t.manager.MkTrue()
	if negative {
		definition, err = t.manager.MkFalse()
	}
	if err != nil {
		return solvedEquation{}, false, err
	}
	solution := solvedEquation{variable: atom, definition: definition}
	if t.proofs {
		eq, err := t.manager.MkEq(atom, definition)
		if err != nil {
			return solution, false, err
		}
		proof, err := t.manager.MkProofRewrite(eq, term)
		if err != nil {
			return solution, false, err
		}
		if err = frame.pin(proof); err != nil {
			return solution, false, err
		}
		solution.proof, err = t.manager.MkProofSymmetry(proof)
		if err != nil {
			return solution, false, err
		}
		if err = frame.pin(solution.proof); err != nil {
			return solution, false, err
		}
	}
	if !negative {
		if err = t.traceExpression("solve_eqs_bug2", "solve", 369, "eliminating: ", term); err != nil {
			return solution, false, err
		}
	}
	return solution, true, nil
}

func (t *SolveEquationsTactic) solveITE(term ast.ASTHandle, frame *solveEquationFrame) (solvedEquation, bool, error) {
	m := t.manager
	condition, err := m.GetArg(term, 0)
	if err != nil {
		return solvedEquation{}, false, err
	}
	thenBranch, err := m.GetArg(term, 1)
	if err != nil {
		return solvedEquation{}, false, err
	}
	elseBranch, err := m.GetArg(term, 2)
	if err != nil {
		return solvedEquation{}, false, err
	}
	for _, branch := range []ast.ASTHandle{thenBranch, elseBranch} {
		eq, e := t.op(branch, m.BasicFamilyID(), ast.ASTOpEq)
		if e != nil || !eq {
			return solvedEquation{}, false, e
		}
	}
	a, err := m.GetArg(thenBranch, 0)
	if err != nil {
		return solvedEquation{}, false, err
	}
	b, err := m.GetArg(thenBranch, 1)
	if err != nil {
		return solvedEquation{}, false, err
	}
	c, err := m.GetArg(elseBranch, 0)
	if err != nil {
		return solvedEquation{}, false, err
	}
	d, err := m.GetArg(elseBranch, 1)
	if err != nil {
		return solvedEquation{}, false, err
	}
	for _, choice := range [][4]ast.ASTHandle{{a, b, c, d}, {b, a, c, d}, {a, b, d, c}, {b, a, d, c}} {
		variable, left, other, right := choice[0], choice[1], choice[2], choice[3]
		if variable != other {
			continue
		}
		constant, e := t.uninterpretedConstant(variable)
		if e != nil {
			return solvedEquation{}, false, e
		}
		if !constant || t.candidateVars[variable] {
			continue
		}
		occurs := false
		for _, part := range []ast.ASTHandle{condition, left, right} {
			found, e := m.OccursExpr(variable, part)
			if e != nil {
				return solvedEquation{}, false, e
			}
			occurs = occurs || found
			if occurs {
				break
			}
		}
		if occurs {
			continue
		}
		acceptable, e := t.checkOccurrences(variable)
		if e != nil {
			return solvedEquation{}, false, e
		}
		if !acceptable {
			continue
		}
		definition, e := m.MkIte(condition, left, right)
		if e != nil {
			return solvedEquation{}, false, e
		}
		if e = frame.pin(definition); e != nil {
			return solvedEquation{}, false, e
		}
		solution := solvedEquation{variable: variable, definition: definition}
		if t.proofs {
			eq, e := m.MkEq(variable, definition)
			if e != nil {
				return solution, false, e
			}
			solution.proof, e = m.MkProofRewrite(term, eq)
			if e != nil {
				return solution, false, e
			}
			if e = frame.pin(solution.proof); e != nil {
				return solution, false, e
			}
		}
		return solution, true, nil
	}
	return solvedEquation{}, false, nil
}

func (t *SolveEquationsTactic) occursElsewhere(variable ast.ASTHandle, args []ast.ASTHandle, index int, rhs ast.ASTHandle) (bool, error) {
	occurs, err := t.manager.OccursExpr(variable, rhs)
	if err != nil || occurs {
		return occurs, err
	}
	for i, arg := range args {
		if i == index {
			continue
		}
		occurs, err = t.manager.OccursExpr(variable, arg)
		if err != nil || occurs {
			return occurs, err
		}
	}
	return false, nil
}

func (t *SolveEquationsTactic) solveArithmetic(left, right, equation ast.ASTHandle, frame *solveEquationFrame) (solvedEquation, bool, error) {
	family, err := t.manager.ArithmeticFamilyID()
	if err != nil {
		return solvedEquation{}, false, err
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		add, e := t.op(pair[0], family, 6)
		if e != nil {
			return solvedEquation{}, false, e
		}
		if !add {
			continue
		}
		solution, found, e := t.solveArithmeticSum(pair[0], pair[1], equation, family, frame)
		if e != nil || found {
			return solution, found, e
		}
	}
	return solvedEquation{}, false, nil
}

func (t *SolveEquationsTactic) solveArithmeticSum(lhs, rhs, equation ast.ASTHandle, family ast.FamilyID, frame *solveEquationFrame) (solution solvedEquation, found bool, err error) {
	m := t.manager
	args, err := t.children(lhs)
	if err != nil {
		return solution, false, err
	}
	sort, err := m.GetSort(lhs)
	if err != nil {
		return solution, false, err
	}
	intSort, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		return solution, false, err
	}
	integer := sort == intSort
	numbers := m.NumeralService()
	coefficient, err := numbers.One()
	if err != nil {
		return solution, false, err
	}
	defer func() { err = errors.Join(err, numbers.Del(&coefficient)) }()
	selected := -1
	for i, arg := range args {
		variable, e := t.uninterpretedConstant(arg)
		if e != nil {
			return solution, false, e
		}
		candidate := arg
		if !variable {
			mul, e := t.op(arg, family, 9)
			if e != nil {
				return solution, false, e
			}
			if !mul {
				continue
			}
			count, e := m.GetNumArgs(arg)
			if e != nil {
				return solution, false, e
			}
			if count != 2 {
				continue
			}
			a, e := m.GetArg(arg, 0)
			if e != nil {
				return solution, false, e
			}
			candidate, e = m.GetArg(arg, 1)
			if e != nil {
				return solution, false, e
			}
			variable, e = t.uninterpretedConstant(candidate)
			if e != nil {
				return solution, false, e
			}
			if !variable {
				continue
			}
			text, _, numeral, e := m.ArithmeticNumeralText(a)
			if e != nil {
				return solution, false, e
			}
			if !numeral {
				continue
			}
			if e = numbers.SetString(&coefficient, text); e != nil {
				return solution, false, e
			}
			zero, e := numbers.IsZero(&coefficient)
			if e != nil {
				return solution, false, e
			}
			if zero {
				continue
			}
			minusOne, e := numbers.IsMinusOne(&coefficient)
			if e != nil {
				return solution, false, e
			}
			if integer && !minusOne {
				continue
			}
		} else {
			if err = numbers.SetInt64(&coefficient, 1); err != nil {
				return solution, false, err
			}
		}
		if t.candidateVars[candidate] {
			continue
		}
		acceptable, e := t.checkOccurrences(candidate)
		if e != nil {
			return solution, false, e
		}
		if !acceptable {
			continue
		}
		occurs, e := t.occursElsewhere(candidate, args, i, rhs)
		if e != nil {
			return solution, false, e
		}
		if occurs {
			continue
		}
		solution.variable = candidate
		selected = i
		break
	}
	if selected < 0 {
		return solution, false, nil
	}
	var inverse ast.ASTHandle
	one, err := numbers.IsOne(&coefficient)
	if err != nil {
		return solution, false, err
	}
	if !one {
		oneValue, e := numbers.One()
		if e != nil {
			return solution, false, e
		}
		inv, e := numbers.Div(&oneValue, &coefficient)
		if e = errors.Join(e, numbers.Del(&oneValue)); e != nil {
			return solution, false, e
		}
		inverse, e = m.ArithmeticNumeral(&inv, integer)
		if e = errors.Join(e, numbers.Del(&inv)); e != nil {
			return solution, false, e
		}
		if err = frame.pin(inverse); err != nil {
			return solution, false, err
		}
		rhs, err = m.MkArithmeticMul(inverse, rhs)
		if err != nil {
			return solution, false, err
		}
		if err = frame.pin(rhs); err != nil {
			return solution, false, err
		}
	}
	var others []ast.ASTHandle
	for i, arg := range args {
		if i == selected {
			continue
		}
		if !inverse.IsNull() {
			arg, err = m.MkArithmeticMul(inverse, arg)
			if err != nil {
				return solution, false, err
			}
			if err = frame.pin(arg); err != nil {
				return solution, false, err
			}
		}
		others = append(others, arg)
	}
	solution.definition = rhs
	if len(others) > 0 {
		sum := others[0]
		if len(others) > 1 {
			sum, err = m.MkFamilyApp(family, 6, nil, others)
			if err != nil {
				return solution, false, err
			}
			if err = frame.pin(sum); err != nil {
				return solution, false, err
			}
		}
		solution.definition, err = m.MkArithmeticSub(rhs, sum)
		if err != nil {
			return solution, false, err
		}
		if err = frame.pin(solution.definition); err != nil {
			return solution, false, err
		}
	}
	if t.proofs {
		eq, e := m.MkEq(solution.variable, solution.definition)
		if e != nil {
			return solution, false, e
		}
		solution.proof, err = m.MkProofRewrite(equation, eq)
		if err != nil {
			return solution, false, err
		}
		if err = frame.pin(solution.proof); err != nil {
			return solution, false, err
		}
	}
	return solution, true, nil
}
