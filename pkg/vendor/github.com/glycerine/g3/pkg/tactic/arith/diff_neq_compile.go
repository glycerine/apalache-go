package arith

// Source: src/tactic/arith/diff_neq_tactic.cpp compile and bound/atom processing.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func diffNeqUnsupported() error { return tactic.NewTacticException("goal is not diff neq") }
func (t *DiffNeqTactic) diffNeqApp(term ast.ASTHandle) (family ast.FamilyID, op int32, args []ast.ASTHandle, app bool, err error) {
	kind, err := t.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return family, op, nil, false, err
	}
	family, err = t.manager.GetFamilyID(term)
	if err != nil {
		return family, op, nil, false, err
	}
	op, err = t.manager.GetDeclKind(term)
	if err != nil {
		return family, op, nil, false, err
	}
	count, err := t.manager.GetNumArgs(term)
	if err != nil {
		return family, op, nil, false, err
	}
	args = make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = t.manager.GetArg(term, uint32(i))
		if err != nil {
			return family, op, nil, false, err
		}
	}
	return family, op, args, true, nil
}
func (t *DiffNeqTactic) diffNeqConstant(term ast.ASTHandle) (bool, error) {
	family, _, args, app, err := t.diffNeqApp(term)
	return app && family == ast.NullFamilyID && len(args) == 0, err
}
func (t *DiffNeqTactic) diffNeqInteger(term ast.ASTHandle) (bool, error) {
	sort, err := t.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	family, err := t.manager.GetFamilyID(sort)
	if err != nil || family != t.family {
		return false, err
	}
	kind, err := t.manager.GetDeclKind(sort)
	return kind == 1, err
}
func (t *DiffNeqTactic) diffNeqNumeral(term ast.ASTHandle) (value big.Rat, found bool, err error) {
	text, _, found, err := t.manager.ArithmeticNumeralText(term)
	if err != nil || !found {
		return value, found, err
	}
	if _, ok := value.SetString(text); !ok {
		return value, false, ast.ErrASTDomain
	}
	return value, true, nil
}
func (t *DiffNeqTactic) diffNeqOffset(term ast.ASTHandle) (int32, bool, error) {
	n, found, err := t.diffNeqNumeral(term)
	if err != nil || !found {
		return 0, false, err
	}
	if n.Cmp(big.NewRat(t.current.maxNegK, 1)) < 0 || n.Cmp(big.NewRat(t.current.maxK, 1)) > 0 {
		return 0, false, nil
	}
	if !n.IsInt() || !n.Num().IsInt64() {
		return 0, false, ast.ErrASTDomain
	}
	return int32(n.Num().Int64()), true, nil
}
func (t *DiffNeqTactic) mkDiffNeqVariable(term ast.ASTHandle) (uint32, error) {
	if x, found := t.current.indices[term]; found {
		return x, nil
	}
	constant, err := t.diffNeqConstant(term)
	if err != nil {
		return 0, err
	}
	if !constant {
		return 0, ast.ErrASTDomain
	}
	if uint64(len(t.current.variables)) >= uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	if err = t.manager.IncRef(term); err != nil {
		return 0, err
	}
	x := uint32(len(t.current.variables))
	t.current.indices[term] = x
	t.current.variables = append(t.current.variables, diffNeqVariable{term: term, lower: -2147483648, upper: 2147483647})
	return x, nil
}
func (t *DiffNeqTactic) processDiffNeqLE(lhs, rhs ast.ASTHandle) error {
	integer, err := t.diffNeqInteger(lhs)
	if err != nil {
		return err
	}
	if !integer {
		return diffNeqUnsupported()
	}
	constant, err := t.diffNeqConstant(lhs)
	if err != nil {
		return err
	}
	if constant {
		k, found, err := t.diffNeqOffset(rhs)
		if err != nil {
			return err
		}
		if found {
			x, err := t.mkDiffNeqVariable(lhs)
			if err != nil {
				return err
			}
			if k < t.current.variables[x].upper {
				t.current.variables[x].upper = k
			}
			return nil
		}
	}
	constant, err = t.diffNeqConstant(rhs)
	if err != nil {
		return err
	}
	if constant {
		k, found, err := t.diffNeqOffset(lhs)
		if err != nil {
			return err
		}
		if found {
			x, err := t.mkDiffNeqVariable(rhs)
			if err != nil {
				return err
			}
			if k > t.current.variables[x].lower {
				t.current.variables[x].lower = k
			}
			return nil
		}
	}
	return diffNeqUnsupported()
}
func (t *DiffNeqTactic) processDiffNeqCore(lhs, rhs ast.ASTHandle, offset int32) error {
	x, err := t.mkDiffNeqVariable(lhs)
	if err != nil {
		return err
	}
	y, err := t.mkDiffNeqVariable(rhs)
	if err != nil {
		return err
	}
	if x == y {
		return diffNeqUnsupported()
	}
	if x < y {
		x, y = y, x
		offset = -offset
	}
	t.current.variables[x].disequalities = append(t.current.variables[x].disequalities, diffNeqDisequality{y, offset})
	return nil
}
func (t *DiffNeqTactic) diffNeqMinusOne(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	family, op, args, app, err := t.diffNeqApp(term)
	if err != nil || !app || family != t.family || op != 9 || len(args) != 2 {
		return ast.ASTHandle{}, false, err
	}
	n, found, err := t.diffNeqNumeral(args[0])
	if err != nil || !found || n.Cmp(big.NewRat(-1, 1)) != 0 {
		return ast.ASTHandle{}, false, err
	}
	return args[1], true, nil
}
func (t *DiffNeqTactic) processDiffNeqNE(lhs, rhs ast.ASTHandle) error {
	integer, err := t.diffNeqInteger(lhs)
	if err != nil {
		return err
	}
	if !integer {
		return diffNeqUnsupported()
	}
	a, err := t.diffNeqConstant(lhs)
	if err != nil {
		return err
	}
	b, err := t.diffNeqConstant(rhs)
	if err != nil {
		return err
	}
	if a && b {
		return t.processDiffNeqCore(lhs, rhs, 0)
	}
	_, number, err := t.diffNeqNumeral(lhs)
	if err != nil {
		return err
	}
	if number {
		lhs, rhs = rhs, lhs
	}
	k, found, err := t.diffNeqOffset(rhs)
	if err != nil {
		return err
	}
	if !found {
		return diffNeqUnsupported()
	}
	family, op, args, app, err := t.diffNeqApp(lhs)
	if err != nil {
		return err
	}
	if !app || family != t.family || op != 6 || len(args) != 2 {
		return diffNeqUnsupported()
	}
	for _, pair := range [][2]int{{0, 1}, {1, 0}} {
		first, second := args[pair[0]], args[pair[1]]
		constant, err := t.diffNeqConstant(first)
		if err != nil {
			return err
		}
		if !constant {
			continue
		}
		negative, minus, err := t.diffNeqMinusOne(second)
		if err != nil {
			return err
		}
		if !minus {
			continue
		}
		constant, err = t.diffNeqConstant(negative)
		if err != nil {
			return err
		}
		if constant {
			return t.processDiffNeqCore(first, negative, k)
		}
	}
	return diffNeqUnsupported()
}
func (t *DiffNeqTactic) compileDiffNeq(goal *tactic.Goal) error {
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		form, err := goal.Form(i)
		if err != nil {
			return err
		}
		if err = t.traceDiffNeqProcessing(form); err != nil {
			return err
		}
		family, op, args, app, err := t.diffNeqApp(form)
		if err != nil {
			return err
		}
		if app && family == t.family && (op == 2 || op == 3) {
			if len(args) != 2 {
				return ast.ErrASTDomain
			}
			if op == 3 {
				args[0], args[1] = args[1], args[0]
			}
			if err = t.processDiffNeqLE(args[0], args[1]); err != nil {
				return err
			}
			continue
		}
		if app && family == t.manager.BasicFamilyID() && op == ast.ASTOpNot && len(args) == 1 {
			family, op, args, app, err = t.diffNeqApp(args[0])
			if err != nil {
				return err
			}
			if app && family == t.manager.BasicFamilyID() && op == ast.ASTOpEq && len(args) == 2 {
				if err = t.processDiffNeqNE(args[0], args[1]); err != nil {
					return err
				}
				continue
			}
		}
		return diffNeqUnsupported()
	}
	for _, v := range t.current.variables {
		if v.lower == -2147483648 || v.upper == 2147483647 || v.lower != 0 {
			return diffNeqUnsupported()
		}
	}
	return nil
}
