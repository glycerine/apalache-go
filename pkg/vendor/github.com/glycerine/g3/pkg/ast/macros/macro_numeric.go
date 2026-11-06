package macros

// Source: src/ast/macros/macro_util.cpp:36-353,748-829.

import (
	"github.com/glycerine/g3/pkg/ast"
)

const (
	macroArithLE  int32 = 2
	macroArithGE  int32 = 3
	macroArithAdd int32 = 6
	macroArithSub int32 = 7
	macroArithMul int32 = 9
)

func (u *Util) IsBitVector(term ast.ASTHandle) (bool, error) {
	return u.manager.IsBitVector(term)
}

func (u *Util) IsBitVectorSort(sort ast.ASTHandle) (bool, error) {
	return u.manager.IsBitVectorSort(sort)
}

func (u *Util) arithmeticOperation(term ast.ASTHandle, operation int32) (bool, error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := u.manager.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	arithmetic, err := u.manager.ArithmeticFamilyID()
	if err != nil || family != arithmetic {
		return false, err
	}
	op, err := u.manager.GetDeclKind(term)
	return op == operation, err
}

func (u *Util) IsAdd(term ast.ASTHandle) (bool, error) {
	if yes, err := u.arithmeticOperation(term, macroArithAdd); err != nil || yes {
		return yes, err
	}
	return u.manager.IsBitVectorOperation(term, 4)
}

func (u *Util) IsTimesMinusOne(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if product, err := u.arithmeticOperation(term, macroArithMul); err != nil {
		return ast.ASTHandle{}, false, err
	} else if product {
		n, err := u.manager.GetNumArgs(term)
		if err != nil || n != 2 {
			return ast.ASTHandle{}, false, err
		}
		coefficient, err := u.manager.GetArg(term, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		text, _, numeral, err := u.manager.ArithmeticNumeralText(coefficient)
		if err != nil || !numeral || text != "-1" {
			return ast.ASTHandle{}, false, err
		}
		arg, err := u.manager.GetArg(term, 1)
		return arg, err == nil, err
	}
	product, err := u.manager.IsBitVectorOperation(term, 6)
	if err != nil || !product {
		return ast.ASTHandle{}, false, err
	}
	n, err := u.manager.GetNumArgs(term)
	if err != nil || n != 2 {
		return ast.ASTHandle{}, false, err
	}
	coefficient, err := u.manager.GetArg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	minusOne, err := u.manager.IsBitVectorAllOne(coefficient)
	if err != nil || !minusOne {
		return ast.ASTHandle{}, false, err
	}
	arg, err := u.manager.GetArg(term, 1)
	return arg, err == nil, err
}

func (u *Util) IsLE(term ast.ASTHandle) (bool, error) {
	if yes, err := u.arithmeticOperation(term, macroArithLE); err != nil || yes {
		return yes, err
	}
	if yes, err := u.manager.IsBitVectorOperation(term, 22); err != nil || yes {
		return yes, err
	}
	return u.manager.IsBitVectorOperation(term, 23)
}

func (u *Util) IsLEGE(term ast.ASTHandle) (bool, error) {
	if yes, err := u.IsLE(term); err != nil || yes {
		return yes, err
	}
	return u.arithmeticOperation(term, macroArithGE)
}

func (u *Util) IsZero(term ast.ASTHandle) (bool, error) {
	bv, err := u.manager.IsBitVector(term)
	if err != nil {
		return false, err
	}
	if bv {
		return u.manager.IsBitVectorZero(term)
	}
	text, _, numeral, err := u.manager.ArithmeticNumeralText(term)
	return numeral && text == "0", err
}

func (u *Util) MkZero(sort ast.ASTHandle) (ast.ASTHandle, error) {
	return u.manager.MkDatalogNumeral(0, sort)
}

func (u *Util) MkSub(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	bv, err := u.manager.IsBitVector(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bv {
		return u.manager.MkBitVectorSub(left, right)
	}
	family, err := u.manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.manager.MkFamilyApp(family, macroArithSub, nil, []ast.ASTHandle{left, right})
}

func (u *Util) MkAdd(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	sort, err := u.manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.mkAdd([]ast.ASTHandle{left, right}, sort)
}

func (u *Util) mkAdd(arguments []ast.ASTHandle, sort ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(arguments) {
	case 0:
		return u.MkZero(sort)
	case 1:
		return arguments[0], nil
	}
	bv, err := u.manager.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bv {
		result := arguments[0]
		for _, argument := range arguments[1:] {
			result, err = u.manager.MkBitVectorAdd(result, argument)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return result, nil
	}
	family, err := u.manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.manager.MkFamilyApp(family, macroArithAdd, nil, arguments)
}

func (u *Util) IsVarPlusGround(term ast.ASTHandle, ground ast.ASTRef) (inverse bool, variable ast.ASTHandle, matched bool, err error) {
	add, err := u.IsAdd(term)
	if err != nil || !add {
		return false, variable, false, err
	}
	closed, err := u.manager.IsGround(term)
	if err != nil || closed {
		return false, variable, false, err
	}
	current := term
	groundTerms := make([]ast.ASTHandle, 0, 2)
	for {
		argument := current
		add, e := u.IsAdd(current)
		if e != nil {
			return false, variable, false, e
		}
		if add {
			n, e := u.manager.GetNumArgs(current)
			if e != nil || n < 2 {
				return false, variable, false, e
			}
			argument, e = u.manager.GetArg(current, 0)
			if e != nil {
				return false, variable, false, e
			}
			current, e = u.manager.GetArg(current, 1)
			if e != nil {
				return false, variable, false, e
			}
		}
		isGround, e := u.manager.IsGround(argument)
		if e != nil {
			return false, variable, false, e
		}
		if isGround {
			groundTerms = append(groundTerms, argument)
		} else {
			kind, e := u.manager.GetKind(argument)
			if e != nil {
				return false, variable, false, e
			}
			candidate, negative := argument, false
			if kind != ast.ASTVar {
				candidate, negative, e = u.IsTimesMinusOne(argument)
				if e != nil {
					return false, variable, false, e
				}
				if negative {
					kind, e = u.manager.GetKind(candidate)
					if e != nil {
						return false, variable, false, e
					}
				}
			}
			if kind != ast.ASTVar || !variable.IsNull() {
				return false, ast.ASTHandle{}, false, nil
			}
			variable, inverse = candidate, negative
		}
		if !add {
			break
		}
	}
	if variable.IsNull() || len(groundTerms) == 0 {
		return false, ast.ASTHandle{}, false, nil
	}
	sort, err := u.manager.GetSort(term)
	if err != nil {
		return false, ast.ASTHandle{}, false, err
	}
	value, err := u.mkAdd(groundTerms, sort)
	if err != nil {
		return false, ast.ASTHandle{}, false, err
	}
	if err = u.manager.SetRef(ground, value); err != nil {
		return false, ast.ASTHandle{}, false, err
	}
	return inverse, variable, true, nil
}

func (u *Util) IsPseudoHead(term ast.ASTHandle, declarations uint32) (head, ground ast.ASTHandle, matched bool, err error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return head, ground, false, err
	}
	family, err := u.manager.GetFamilyID(term)
	if err != nil {
		return head, ground, false, err
	}
	op, err := u.manager.GetDeclKind(term)
	if err != nil || family != u.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return head, ground, false, err
	}
	lhs, err := u.manager.GetArg(term, 0)
	if err != nil {
		return head, ground, false, err
	}
	rhs, err := u.manager.GetArg(term, 1)
	if err != nil {
		return head, ground, false, err
	}
	lg, err := u.manager.IsGround(lhs)
	if err != nil {
		return head, ground, false, err
	}
	rg, err := u.manager.IsGround(rhs)
	if err != nil || !lg && !rg {
		return head, ground, false, err
	}
	sort, err := u.manager.GetSort(lhs)
	if err != nil {
		return head, ground, false, err
	}
	sortFamily, err := u.manager.GetFamilyID(sort)
	if err != nil || sortFamily == ast.NullFamilyID {
		return head, ground, false, err
	}
	size, err := u.manager.GetSortSize(sort)
	if err != nil {
		return head, ground, false, err
	}
	if size.IsFinite() {
		n, e := size.Size()
		if e != nil || n == 1 {
			return head, ground, false, e
		}
	}
	if yes, e := u.IsMacroHead(lhs, declarations); e != nil || yes {
		return lhs, rhs, yes, e
	}
	if yes, e := u.IsMacroHead(rhs, declarations); e != nil || yes {
		return rhs, lhs, yes, e
	}
	return head, ground, false, nil
}

// IsArithmeticMacro retains the source's legacy arithmetic-only recognizer.
func (u *Util) IsArithmeticMacro(term ast.ASTHandle, declarations uint32) (head, definition ast.ASTHandle, inverse, matched bool, err error) {
	family, err := u.manager.GetFamilyID(term)
	if err != nil {
		return head, definition, false, false, err
	}
	op, err := u.manager.GetDeclKind(term)
	if err != nil {
		return head, definition, false, false, err
	}
	arithmetic, err := u.manager.ArithmeticFamilyID()
	if err != nil {
		return head, definition, false, false, err
	}
	if !(family == u.manager.BasicFamilyID() && op == ast.ASTOpEq) && !(family == arithmetic && (op == macroArithLE || op == macroArithGE)) {
		return head, definition, false, false, nil
	}
	lhs, err := u.manager.GetArg(term, 0)
	if err != nil {
		return head, definition, false, false, err
	}
	rhs, err := u.manager.GetArg(term, 1)
	if err != nil {
		return head, definition, false, false, err
	}
	_, _, numeral, err := u.manager.ArithmeticNumeralText(rhs)
	if err != nil || !numeral {
		return head, definition, false, false, err
	}
	arguments, err := u.polynomialArguments(lhs)
	if err != nil {
		return head, definition, false, false, err
	}
	rest := make([]ast.ASTHandle, 0, len(arguments))
	foundArgument := ast.ASTHandle{}
	for _, argument := range arguments {
		candidate, negative := argument, false
		yes, e := u.IsMacroHead(candidate, declarations)
		if e != nil {
			return head, definition, false, false, e
		}
		if !yes {
			candidate, negative, e = u.IsTimesMinusOne(argument)
			if e != nil {
				return head, definition, false, false, e
			}
			if negative {
				yes, e = u.IsMacroHead(candidate, declarations)
				if e != nil {
					return head, definition, false, false, e
				}
			}
		}
		if foundArgument.IsNull() && yes {
			declaration, e := u.manager.GetDecl(candidate)
			if e != nil {
				return head, definition, false, false, e
			}
			contained, e := u.polynomialContains(lhs, declaration, argument)
			if e != nil {
				return head, definition, false, false, e
			}
			if !u.forbiddenDecl(declaration) && !contained {
				head, foundArgument, inverse = candidate, argument, negative
				continue
			}
		}
		rest = append(rest, argument)
	}
	if foundArgument.IsNull() {
		return head, definition, false, false, nil
	}
	sort, err := u.manager.GetSort(lhs)
	if err != nil {
		return head, definition, false, false, err
	}
	restTerm, err := u.mkAdd(rest, sort)
	if err != nil {
		return head, definition, false, false, err
	}
	if inverse {
		definition, err = u.MkSub(restTerm, rhs)
	} else {
		definition, err = u.MkSub(rhs, restTerm)
	}
	return head, definition, inverse, err == nil, err
}

func (u *Util) IsPseudoPredicateMacro(term ast.ASTHandle) (head, ground, definition ast.ASTHandle, matched bool, err error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTQuantifier {
		return head, ground, definition, false, err
	}
	forall, err := u.manager.IsForall(term)
	if err != nil || !forall {
		return head, ground, definition, false, err
	}
	body, err := u.manager.GetQuantifierBody(term)
	if err != nil {
		return head, ground, definition, false, err
	}
	kind, err = u.manager.GetKind(body)
	if err != nil || kind != ast.ASTApp {
		return head, ground, definition, false, err
	}
	family, err := u.manager.GetFamilyID(body)
	if err != nil {
		return head, ground, definition, false, err
	}
	op, err := u.manager.GetDeclKind(body)
	if err != nil || family != u.manager.BasicFamilyID() || op != ast.ASTOpIff {
		return head, ground, definition, false, err
	}
	declarations, err := u.manager.GetNumDeclarations(term)
	if err != nil {
		return head, ground, definition, false, err
	}
	lhs, err := u.manager.GetArg(body, 0)
	if err != nil {
		return head, ground, definition, false, err
	}
	rhs, err := u.manager.GetArg(body, 1)
	if err != nil {
		return head, ground, definition, false, err
	}
	for _, pair := range [][2]ast.ASTHandle{{lhs, rhs}, {rhs, lhs}} {
		h, value, yes, e := u.IsPseudoHead(pair[0], declarations)
		if e != nil {
			return head, ground, definition, false, e
		}
		if !yes {
			continue
		}
		declaration, e := u.manager.GetDecl(h)
		if e != nil {
			return head, ground, definition, false, e
		}
		occurs, e := u.manager.OccursDecl(declaration, pair[1])
		if e != nil {
			return head, ground, definition, false, e
		}
		if !u.forbiddenDecl(declaration) && !occurs {
			return h, value, pair[1], true, nil
		}
	}
	return head, ground, definition, false, nil
}

func (u *Util) polynomialArguments(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	add, err := u.IsAdd(term)
	if err != nil || !add {
		return []ast.ASTHandle{term}, err
	}
	n, err := u.manager.GetNumArgs(term)
	if err != nil {
		return nil, err
	}
	arguments := make([]ast.ASTHandle, n)
	for i := range arguments {
		arguments[i], err = u.manager.GetArg(term, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

func (u *Util) polynomialContains(term, declaration, exception ast.ASTHandle) (bool, error) {
	arguments, err := u.polynomialArguments(term)
	if err != nil {
		return false, err
	}
	for _, argument := range arguments {
		if argument != exception {
			yes, e := u.manager.OccursDecl(declaration, argument)
			if e != nil || yes {
				return yes, e
			}
		}
	}
	return false, nil
}

func (u *Util) isPolyHint(term, head, exception ast.ASTHandle) (bool, error) {
	variables, yes, err := u.hintHead(head)
	if err != nil || !yes {
		return false, err
	}
	declaration, err := u.manager.GetDecl(head)
	if err != nil {
		return false, err
	}
	arguments, err := u.polynomialArguments(term)
	if err != nil {
		return false, err
	}
	for _, argument := range arguments {
		if argument == exception {
			continue
		}
		occurs, e := u.manager.OccursDecl(declaration, argument)
		if e != nil || occurs {
			return false, e
		}
		subset, e := u.variablesSubset(argument, variables)
		if e != nil || !subset {
			return false, e
		}
	}
	return true, nil
}

func (u *Util) collectArithmeticSide(lhs, rhs, atom ast.ASTHandle, declarations uint32, inequality bool, candidates *Candidates) error {
	add, err := u.IsAdd(lhs)
	if err != nil {
		return err
	}
	family, err := u.manager.GetFamilyID(atom)
	if err != nil {
		return err
	}
	op, err := u.manager.GetDeclKind(atom)
	if err != nil {
		return err
	}
	if !add && family == u.manager.BasicFamilyID() && op == ast.ASTOpEq {
		return nil
	}
	arguments, err := u.polynomialArguments(lhs)
	if err != nil {
		return err
	}
	for _, argument := range arguments {
		head, negative := argument, false
		kind, e := u.manager.GetKind(head)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			continue
		}
		declaration, e := u.manager.GetDecl(head)
		if e != nil {
			return e
		}
		quasi, e := u.IsQuasiMacroHead(head, declarations)
		if e != nil {
			return e
		}
		contained, e := u.polynomialContains(lhs, declaration, argument)
		if e != nil {
			return e
		}
		inRight, e := u.manager.OccursDecl(declaration, rhs)
		if e != nil {
			return e
		}
		inRest, e := u.restContains(declaration, atom)
		if e != nil {
			return e
		}
		macro := quasi && !u.forbiddenDecl(declaration) && !contained && !inRight && !inRest
		hint := false
		if !macro {
			hint, e = u.isPolyHint(lhs, head, argument)
			if e != nil {
				return e
			}
		}
		if !macro && !hint {
			head, negative, e = u.IsTimesMinusOne(argument)
			if e != nil || !negative {
				if e != nil {
					return e
				}
				continue
			}
			kind, e = u.manager.GetKind(head)
			if e != nil || kind != ast.ASTApp {
				if e != nil {
					return e
				}
				continue
			}
			declaration, e = u.manager.GetDecl(head)
			if e != nil {
				return e
			}
			quasi, e = u.IsQuasiMacroHead(head, declarations)
			if e != nil {
				return e
			}
			contained, e = u.polynomialContains(lhs, declaration, argument)
			if e != nil {
				return e
			}
			inRight, e = u.manager.OccursDecl(declaration, rhs)
			if e != nil {
				return e
			}
			inRest, e = u.restContains(declaration, atom)
			if e != nil {
				return e
			}
			macro = quasi && !u.forbiddenDecl(declaration) && !contained && !inRight && !inRest
			if !macro {
				hint, e = u.isPolyHint(lhs, head, argument)
				if e != nil {
					return e
				}
			}
		}
		if !macro && !hint {
			continue
		}
		rest := make([]ast.ASTHandle, 0, len(arguments)-1)
		for _, value := range arguments {
			if value != argument {
				rest = append(rest, value)
			}
		}
		sort, e := u.manager.GetSort(argument)
		if e != nil {
			return e
		}
		restTerm, e := u.mkAdd(rest, sort)
		if e != nil {
			return e
		}
		definition := ast.ASTHandle{}
		if negative {
			definition, e = u.MkSub(restTerm, rhs)
		} else {
			definition, e = u.MkSub(rhs, restTerm)
		}
		if e != nil {
			return e
		}
		if hint {
			valid, e := u.isPolyHint(definition, head, ast.ASTHandle{})
			if e != nil {
				return e
			}
			if !valid {
				continue
			}
		}
		condition := ast.ASTHandle{}
		if !hint {
			condition, e = u.restCondition(atom)
			if e != nil {
				return e
			}
		}
		if e = u.insertCandidate(head, declarations, definition, condition, inequality, true, hint, candidates); e != nil {
			return e
		}
	}
	return nil
}

func (u *Util) collectArithmeticCandidates(atom ast.ASTHandle, declarations uint32, candidates *Candidates) error {
	family, err := u.manager.GetFamilyID(atom)
	if err != nil {
		return err
	}
	op, err := u.manager.GetDeclKind(atom)
	if err != nil {
		return err
	}
	equality := family == u.manager.BasicFamilyID() && op == ast.ASTOpEq
	comparison, err := u.IsLEGE(atom)
	if err != nil || !equality && !comparison {
		return err
	}
	lhs, err := u.manager.GetArg(atom, 0)
	if err != nil {
		return err
	}
	rhs, err := u.manager.GetArg(atom, 1)
	if err != nil {
		return err
	}
	if err = u.collectArithmeticSide(lhs, rhs, atom, declarations, !equality, candidates); err != nil {
		return err
	}
	return u.collectArithmeticSide(rhs, lhs, atom, declarations, !equality, candidates)
}
