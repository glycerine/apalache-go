package smtcore

// Source: src/smt/smt_model_finder.cpp quantifier_analyzer AUF classification.

import "github.com/glycerine/g3/pkg/ast"

type smtModelFinderAUFClassifier struct {
	finder *SMTModelFinder
	seen   map[ast.ASTHandle]uint8
}

const (
	aufSeenFormulaPositive uint8 = 1 << iota
	aufSeenFormulaNegative
	aufSeenTerm
)

func (c *smtModelFinderAUFClassifier) varPlusGround(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	ground, err := c.finder.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	defer func() { _ = c.finder.manager.DelRef(&ground) }()
	inverse, variable, matched, err := c.finder.macroUtil.IsVarPlusGround(term, ground)
	return variable, matched && !inverse, err
}

func (c *smtModelFinderAUFClassifier) varOrPositiveOffset(term ast.ASTHandle) (bool, error) {
	kind, err := c.finder.manager.GetKind(term)
	if err != nil || kind == ast.ASTVar {
		return kind == ast.ASTVar, err
	}
	_, matched, err := c.varPlusGround(term)
	return matched, err
}

func (c *smtModelFinderAUFClassifier) varOrOffset(term ast.ASTHandle) (bool, error) {
	kind, err := c.finder.manager.GetKind(term)
	if err != nil || kind == ast.ASTVar {
		return kind == ast.ASTVar, err
	}
	ground, err := c.finder.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { _ = c.finder.manager.DelRef(&ground) }()
	_, _, matched, err := c.finder.macroUtil.IsVarPlusGround(term, ground)
	return matched, err
}

func (c *smtModelFinderAUFClassifier) varAndGround(left, right ast.ASTHandle) (bool, error) {
	leftVar, err := c.varOrOffset(left)
	if err != nil {
		return false, err
	}
	rightGround, err := c.finder.manager.IsGround(right)
	if err != nil {
		return false, err
	}
	if leftVar && rightGround {
		return true, nil
	}
	rightVar, err := c.varOrOffset(right)
	if err != nil {
		return false, err
	}
	leftGround, err := c.finder.manager.IsGround(left)
	return rightVar && leftGround, err
}

func (c *smtModelFinderAUFClassifier) varMinusVar(term ast.ASTHandle) (bool, error) {
	add, err := c.finder.macroUtil.IsAdd(term)
	if err != nil || !add {
		return false, err
	}
	n, err := c.finder.manager.GetNumArgs(term)
	if err != nil || n != 2 {
		return false, err
	}
	left, err := c.finder.manager.GetArg(term, 0)
	if err != nil {
		return false, err
	}
	right, err := c.finder.manager.GetArg(term, 1)
	if err != nil {
		return false, err
	}
	leftKind, err := c.finder.manager.GetKind(left)
	if err != nil {
		return false, err
	}
	if leftKind != ast.ASTVar {
		left, right = right, left
		leftKind, err = c.finder.manager.GetKind(left)
		if err != nil {
			return false, err
		}
	}
	if leftKind != ast.ASTVar {
		return false, nil
	}
	argument, negative, err := c.finder.macroUtil.IsTimesMinusOne(right)
	if err != nil || !negative {
		return false, err
	}
	kind, err := c.finder.manager.GetKind(argument)
	return kind == ast.ASTVar, err
}

func (c *smtModelFinderAUFClassifier) varAndVar(left, right ast.ASTHandle) (bool, error) {
	lk, err := c.finder.manager.GetKind(left)
	if err != nil {
		return false, err
	}
	rk, err := c.finder.manager.GetKind(right)
	if err != nil {
		return false, err
	}
	if lk == ast.ASTVar && rk == ast.ASTVar {
		return true, nil
	}
	leftDifference, err := c.varMinusVar(left)
	if err != nil {
		return false, err
	}
	rightZero, err := c.finder.macroUtil.IsZero(right)
	if err != nil {
		return false, err
	}
	if leftDifference && rightZero {
		return true, nil
	}
	rightDifference, err := c.varMinusVar(right)
	if err != nil {
		return false, err
	}
	leftZero, err := c.finder.macroUtil.IsZero(left)
	return rightDifference && leftZero, err
}

func (c *smtModelFinderAUFClassifier) recognizedLiteral(atom ast.ASTHandle, negative bool) (bool, error) {
	family, err := c.finder.manager.GetFamilyID(atom)
	if err != nil {
		return false, err
	}
	op, err := c.finder.manager.GetDeclKind(atom)
	if err != nil {
		return false, err
	}
	if family == c.finder.manager.BasicFamilyID() && op == ast.ASTOpEq {
		left, err := c.finder.manager.GetArg(atom, 0)
		if err != nil {
			return false, err
		}
		right, err := c.finder.manager.GetArg(atom, 1)
		if err != nil {
			return false, err
		}
		if matched, err := c.varAndGround(left, right); err != nil || matched {
			return matched, err
		}
		return c.varAndVar(left, right)
	}
	comparison, err := c.finder.macroUtil.IsLEGE(atom)
	if err != nil || !comparison {
		return false, err
	}
	left, err := c.finder.manager.GetArg(atom, 0)
	if err != nil {
		return false, err
	}
	right, err := c.finder.manager.GetArg(atom, 1)
	if err != nil {
		return false, err
	}
	if negative {
		if matched, err := c.varAndVar(left, right); err != nil || matched {
			return matched, err
		}
	}
	return c.varAndGround(left, right)
}

func (c *smtModelFinderAUFClassifier) isAUFSelect(term ast.ASTHandle) (bool, error) {
	kind, err := c.finder.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.finder.manager.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	arrayFamily, err := c.finder.manager.ArrayFamilyID()
	if err != nil || family != arrayFamily {
		return false, err
	}
	op, err := c.finder.manager.GetDeclKind(term)
	if err != nil || op != ast.ASTArrayOpSelect {
		return false, err
	}
	n, err := c.finder.manager.GetNumArgs(term)
	if err != nil || n == 0 {
		return false, err
	}
	array, err := c.finder.manager.GetArg(term, 0)
	if err != nil {
		return false, err
	}
	ground, err := c.finder.manager.IsGround(array)
	if err != nil {
		return false, err
	}
	if !ground {
		selectArray, err := c.isAUFSelect(array)
		if err != nil || !selectArray {
			return false, err
		}
	}
	for i := uint32(1); i < n; i++ {
		index, err := c.finder.manager.GetArg(term, i)
		if err != nil {
			return false, err
		}
		ground, err := c.finder.manager.IsGround(index)
		if err != nil {
			return false, err
		}
		kind, err := c.finder.manager.GetKind(index)
		if err != nil || !ground && kind != ast.ASTVar {
			return false, err
		}
	}
	return true, nil
}

func (c *smtModelFinderAUFClassifier) term(term ast.ASTHandle) (bool, error) {
	ground, err := c.finder.manager.IsGround(term)
	if err != nil || ground {
		return true, err
	}
	if c.seen[term]&aufSeenTerm != 0 {
		return true, nil
	}
	c.seen[term] |= aufSeenTerm
	sort, err := c.finder.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	boolean, err := c.finder.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	if sort == boolean {
		if ok, err := c.formulaWithPolarity(term, false); err != nil || !ok {
			return ok, err
		}
		return c.formulaWithPolarity(term, true)
	}
	kind, err := c.finder.manager.GetKind(term)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		return false, nil
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return c.application(term)
}

func (c *smtModelFinderAUFClassifier) application(term ast.ASTHandle) (bool, error) {
	family, err := c.finder.manager.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	n, err := c.finder.manager.GetNumArgs(term)
	if err != nil {
		return false, err
	}
	if family == ast.NullFamilyID {
		for i := uint32(0); i < n; i++ {
			argument, err := c.finder.manager.GetArg(term, i)
			if err != nil {
				return false, err
			}
			kind, err := c.finder.manager.GetKind(argument)
			if err != nil {
				return false, err
			}
			if kind == ast.ASTVar {
				continue
			}
			if _, matched, err := c.varPlusGround(argument); err != nil {
				return false, err
			} else if matched {
				continue
			}
			if ok, err := c.term(argument); err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	if selectTerm, err := c.isAUFSelect(term); err != nil {
		return false, err
	} else if selectTerm {
		array, err := c.finder.manager.GetArg(term, 0)
		if err != nil {
			return false, err
		}
		return c.term(array)
	}
	for i := uint32(0); i < n; i++ {
		argument, err := c.finder.manager.GetArg(term, i)
		if err != nil {
			return false, err
		}
		if ok, err := c.term(argument); err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func (c *smtModelFinderAUFClassifier) formula(formula ast.ASTHandle) (bool, error) {
	return c.formulaWithPolarity(formula, false)
}

func (c *smtModelFinderAUFClassifier) formulaWithPolarity(formula ast.ASTHandle, negative bool) (bool, error) {
	ground, err := c.finder.manager.IsGround(formula)
	if err != nil || ground {
		return true, err
	}
	flag := aufSeenFormulaPositive
	if negative {
		flag = aufSeenFormulaNegative
	}
	if c.seen[formula]&flag != 0 {
		return true, nil
	}
	c.seen[formula] |= flag
	kind, err := c.finder.manager.GetKind(formula)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	family, err := c.finder.manager.GetFamilyID(formula)
	if err != nil {
		return false, err
	}
	op, err := c.finder.manager.GetDeclKind(formula)
	if err != nil {
		return false, err
	}
	n, err := c.finder.manager.GetNumArgs(formula)
	if err != nil {
		return false, err
	}
	if family == c.finder.manager.BasicFamilyID() {
		switch op {
		case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpNot, ast.ASTOpIte, ast.ASTOpIff:
			for i := uint32(0); i < n; i++ {
				argument, err := c.finder.manager.GetArg(formula, i)
				if err != nil {
					return false, err
				}
				childNegative := negative
				if op == ast.ASTOpNot {
					childNegative = !negative
				}
				if ok, err := c.formulaWithPolarity(argument, childNegative); err != nil || !ok {
					return ok, err
				}
				if op == ast.ASTOpIff || op == ast.ASTOpIte && i == 0 {
					if ok, err := c.formulaWithPolarity(argument, !childNegative); err != nil || !ok {
						return ok, err
					}
				}
			}
			return true, nil
		case ast.ASTOpEq:
			left, err := c.finder.manager.GetArg(formula, 0)
			if err != nil {
				return false, err
			}
			sort, err := c.finder.manager.GetSort(left)
			if err != nil {
				return false, err
			}
			boolean, err := c.finder.manager.MkBoolSort()
			if err != nil {
				return false, err
			}
			if sort == boolean {
				for i := uint32(0); i < 2; i++ {
					argument, err := c.finder.manager.GetArg(formula, i)
					if err != nil {
						return false, err
					}
					if ok, err := c.formulaWithPolarity(argument, false); err != nil || !ok {
						return ok, err
					}
					if ok, err := c.formulaWithPolarity(argument, true); err != nil || !ok {
						return ok, err
					}
				}
				return true, nil
			}
		}
	}
	if recognized, err := c.recognizedLiteral(formula, negative); err != nil || recognized {
		return recognized, err
	}
	return c.application(formula)
}

func (f *SMTModelFinder) classifyAUF(quantifier ast.ASTHandle) (bool, error) {
	body, err := f.manager.GetQuantifierBody(f.flatQuantifier(quantifier))
	if err != nil {
		return false, err
	}
	classifier := smtModelFinderAUFClassifier{finder: f, seen: make(map[ast.ASTHandle]uint8)}
	return classifier.formula(body)
}
