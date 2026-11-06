package pdr

// Source: src/muz/pdr/pdr_util.cpp arith_normalizer and normalize_arithmetic.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// NormalizeArithmeticRef matches normalize_arithmetic(expr_ref&): publish the
// rewritten formula into the caller's owning reference before releasing the
// previous formula. On rewrite failure the caller keeps its original formula.
func NormalizeArithmeticRef(manager *ast.ASTManager, formula ast.ASTRef) (resultErr error) {
	if manager == nil {
		return ast.ErrASTHandle
	}
	input, err := manager.RefGet(formula)
	if err != nil {
		return err
	}
	scope, err := ast.NewScopedNoProof(manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	n := arithmeticNormalization{manager: manager}
	defer func() {
		if cleanup := n.close(); cleanup != nil {
			n.cause, n.cleanupErr = errors.Join(resultErr, cleanup), cleanup
			resultErr = &n
		}
	}()
	result, err := n.tree(input, make(map[ast.ASTHandle]ast.ASTHandle))
	if err != nil {
		return err
	}
	return manager.SetRef(formula, result)
}

// Generated nodes stay alive across sibling rewrites and are released only
// after the caller owns the completed root, or after an interrupted rewrite.
type arithmeticNormalization struct {
	manager           *ast.ASTManager
	refs              []ast.ASTHandle
	cause, cleanupErr error
}

func (n *arithmeticNormalization) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	err = n.manager.IncRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n.refs = append(n.refs, h)
	return h, nil
}

func (n *arithmeticNormalization) close() (result error) {
	for len(n.refs) != 0 {
		i := len(n.refs) - 1
		consumed, err := n.manager.DecRefConsumed(n.refs[i])
		result = errors.Join(result, err)
		if !consumed {
			return result
		}
		n.refs = n.refs[:i]
	}
	return result
}

// An interrupted decrement keeps its real ownership roots available for retry.
func (n *arithmeticNormalization) Error() string { return n.cause.Error() }
func (n *arithmeticNormalization) Unwrap() error { return n.cause }
func (n *arithmeticNormalization) Cleanup() error {
	n.cleanupErr = n.close()
	return n.cleanupErr
}
func (n *arithmeticNormalization) RuntimeOwner() *memory.Runtime {
	return n.manager.NumeralService().RuntimeOwner()
}
func (n *arithmeticNormalization) OriginalCause() error { return n.cause }
func (n *arithmeticNormalization) CleanupCause() error  { return n.cleanupErr }

// The source rewriter visits application children and quantifier bodies while
// leaving patterns unchanged. Rebuild with the original declaration so that
// parameters and uninterpreted symbols keep their identities.
func (n *arithmeticNormalization) tree(formula ast.ASTHandle, cache map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	manager := n.manager
	canceled, err := manager.Canceled()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if canceled {
		return ast.ASTHandle{}, rewriter.NewRewriterException(manager.ResourceLimit().CancelMessage())
	}
	if result, found := cache[formula]; found {
		return result, nil
	}
	kind, err := manager.GetKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := formula
	if kind == ast.ASTQuantifier {
		body, err := manager.GetQuantifierBody(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rewritten, err := n.tree(body, cache)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if rewritten != body {
			result, err = n.keep(manager.UpdateQuantifierBody(formula, rewritten))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	} else if kind == ast.ASTApp {
		count, err := manager.GetNumArgs(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, count)
		changed := false
		for i := range args {
			arg, err := manager.GetArg(formula, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i], err = n.tree(arg, cache)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed = changed || args[i] != arg
		}
		if changed {
			decl, err := manager.GetDecl(formula)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = n.keep(manager.MkApp(decl, args))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		result, err = n.comparison(result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	cache[formula] = result
	return result, nil
}

func (n *arithmeticNormalization) comparison(formula ast.ASTHandle) (ast.ASTHandle, error) {
	manager := n.manager
	kind, err := manager.GetKind(formula)
	if err != nil || kind != ast.ASTApp {
		return formula, err
	}
	family, err := manager.GetFamilyID(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := manager.GetDeclKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negateResult := false
	if family == arithmetic && op == 4 {
		op, negateResult = 3, true
	} else if family == arithmetic && op == 5 {
		op, negateResult = 2, true
	}
	if (family == arithmetic && op != 2 && op != 3) || (family == 0 && op != ast.ASTOpEq) || (family != arithmetic && family != 0) {
		return formula, nil
	}
	left, err := manager.GetArg(formula, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := manager.GetArg(formula, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftSort, err := manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSort, err := manager.GetSort(right)
	if err != nil || leftSort != rightSort {
		return formula, err
	}
	realSort, err := n.keep(manager.MkFamilySort(arithmetic, 0, nil))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftSort != realSort {
		if !negateResult {
			return formula, nil
		}
		return n.negatedRelation(family, op, left, right)
	}
	leftTerms, err := n.terms(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightTerms, err := n.terms(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var divisor *big.Rat
	for _, terms := range [][]ast.ASTHandle{leftTerms, rightTerms} {
		for _, term := range terms {
			coefficient, _, err := n.coefficient(term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			coefficient.Abs(coefficient)
			if divisor == nil || divisor.Sign() == 0 || coefficient.Cmp(divisor) < 0 {
				divisor = coefficient
			}
		}
	}
	if divisor == nil || divisor.Sign() == 0 || divisor.Cmp(big.NewRat(1, 1)) == 0 {
		if !negateResult {
			return formula, nil
		}
		return n.negatedRelation(family, op, left, right)
	}
	left, err = n.divide(leftSort, leftTerms, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = n.divide(rightSort, rightTerms, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := []ast.ASTHandle{left, right}
	var result ast.ASTHandle
	if family == 0 {
		result, err = n.keep(manager.MkFamilyApp(0, ast.ASTOpEq, nil, args))
	} else {
		result, err = n.keep(manager.MkFamilyApp(family, op, nil, args))
	}
	if err != nil || !negateResult {
		return result, err
	}
	not, err := n.keep(manager.MkFamilyApp(0, ast.ASTOpNot, nil, []ast.ASTHandle{result}))
	return not, err
}

func (n *arithmeticNormalization) negatedRelation(family ast.FamilyID, op int32, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	manager := n.manager
	relation, err := n.keep(manager.MkFamilyApp(family, op, nil, []ast.ASTHandle{left, right}))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return n.keep(manager.MkFamilyApp(0, ast.ASTOpNot, nil, []ast.ASTHandle{relation}))
}

func (n *arithmeticNormalization) terms(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	manager := n.manager
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return nil, err
	}
	op, err := manager.GetDeclKind(expression)
	if err != nil || family != arithmetic || op != 6 {
		return []ast.ASTHandle{expression}, err
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	terms := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		term, err := manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

func (n *arithmeticNormalization) coefficient(term ast.ASTHandle) (*big.Rat, ast.ASTHandle, error) {
	manager := n.manager
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if text, _, numeral, err := manager.ArithmeticNumeralText(term); err != nil {
		return nil, ast.ASTHandle{}, err
	} else if numeral {
		coefficient, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, ast.ASTHandle{}, ast.ErrASTDomain
		}
		return coefficient, ast.ASTHandle{}, nil
	}
	kind, err := manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return big.NewRat(1, 1), term, err
	}
	family, err := manager.GetFamilyID(term)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	op, err := manager.GetDeclKind(term)
	if err != nil || family != arithmetic || op != 9 {
		return big.NewRat(1, 1), term, err
	}
	count, err := manager.GetNumArgs(term)
	if err != nil || count != 2 {
		return big.NewRat(1, 1), term, err
	}
	coefficientTerm, err := manager.GetArg(term, 0)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	text, _, numeral, err := manager.ArithmeticNumeralText(coefficientTerm)
	if err != nil || !numeral {
		return big.NewRat(1, 1), term, err
	}
	coefficient, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ASTHandle{}, ast.ErrASTDomain
	}
	powerProduct, err := manager.GetArg(term, 1)
	return coefficient, powerProduct, err
}

func (n *arithmeticNormalization) divide(sort ast.ASTHandle, terms []ast.ASTHandle, divisor *big.Rat) (ast.ASTHandle, error) {
	manager := n.manager
	args := make([]ast.ASTHandle, 0, len(terms))
	for _, term := range terms {
		coefficient, powerProduct, err := n.coefficient(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		coefficient.Quo(coefficient, divisor)
		if powerProduct == (ast.ASTHandle{}) {
			if coefficient.Sign() == 0 {
				continue
			}
			text := coefficient.RatString()
			constant, err := n.keep(manager.ArithmeticRealNumeralText(text))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args = append(args, constant)
			continue
		}
		if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
			args = append(args, powerProduct)
			continue
		}
		coefficientHandle, err := n.keep(manager.ArithmeticRealNumeralText(coefficient.RatString()))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		product, err := n.keep(manager.MkArithmeticMul(coefficientHandle, powerProduct))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, product)
	}
	if len(args) == 0 {
		return n.keep(manager.ArithmeticRealNumeralText("0"))
	}
	if len(args) == 1 {
		return args[0], nil
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return n.keep(manager.MkFamilyApp(family, 6, nil, args))
}
