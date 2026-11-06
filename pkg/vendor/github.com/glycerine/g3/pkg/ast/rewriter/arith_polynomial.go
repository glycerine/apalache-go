package rewriter

// Source: src/ast/rewriter/poly_rewriter.h;
// src/ast/rewriter/poly_rewriter_def.h.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/productiter"
)

const arithRelationEq int32 = -1

var ErrArithPolynomialMonomialBlowup = errors.New("sum of monomials blowup")

const (
	arithDivideExact = iota
	arithDivideFloor
	arithDivideCeil
)

func (r *ArithRewriter) integerTerm(h ast.ASTHandle) (bool, error) {
	s, err := r.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err := r.manager.GetFamilyID(s)
	if err != nil || family != r.family {
		return false, err
	}
	kind, err := r.manager.GetDeclKind(s)
	return err == nil && kind == 1, err
}

func (r *ArithRewriter) numeralForTerm(value *big.Rat, term ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := r.integerTerm(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeNumeral(value, integer)
}

func (r *ArithRewriter) makeAdd(args []ast.ASTHandle, term ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return r.numeralForTerm(new(big.Rat), term)
	case 1:
		return args[0], nil
	default:
		return r.app(arithOpAdd, args...)
	}
}

func (r *ArithRewriter) makeMul(args []ast.ASTHandle, term ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return r.numeralForTerm(big.NewRat(1, 1), term)
	case 1:
		return args[0], nil
	default:
		return r.app(arithOpMul, args...)
	}
}

// powerBody is the arithmetic specialization of get_power_body. Only an
// integer exponent greater than one participates in adjacent-factor folding.
func (r *ArithRewriter) powerBody(h ast.ASTHandle) (ast.ASTHandle, *big.Rat, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if kind != ast.ASTApp {
		return h, big.NewRat(1, 1), nil
	}
	_, args, power, err := r.operation(h, arithOpPower)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if power && len(args) == 2 {
		exponent, _, numeral, e := r.numeral(args[1])
		if e != nil {
			return ast.ASTHandle{}, nil, e
		}
		if numeral && exponent.IsInt() && exponent.Cmp(big.NewRat(1, 1)) > 0 {
			return args[0], exponent, nil
		}
	}
	return h, big.NewRat(1, 1), nil
}

func (r *ArithRewriter) sameArguments(args ast.AppArguments, values []ast.ASTHandle) (bool, error) {
	if uint32(len(values)) != args.Count() {
		return false, nil
	}
	for i := range values {
		h, err := args.At(uint32(i))
		if err != nil {
			return false, err
		}
		if h != values[i] {
			return false, nil
		}
	}
	return true, nil
}

func (r *ArithRewriter) monomialOrdinal(h ast.ASTHandle) (int64, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTVar {
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil {
		return 0, err
	}
	if family != r.family {
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	if _, factors, product, err := r.operation(h, arithOpMul); err != nil {
		return 0, err
	} else if product {
		if len(factors) != 0 {
			if _, _, numeral, err := r.numeral(factors[0]); err != nil {
				return 0, err
			} else if numeral && len(factors) > 1 {
				id, err := r.manager.GetASTID(factors[1])
				return int64(id), err
			}
		}
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	if _, _, numeral, err := r.numeral(h); err != nil {
		return 0, err
	} else if numeral {
		return -1, nil
	}
	if r.mulToPower && !r.expandPower {
		if _, args, power, err := r.operation(h, arithOpPower); err != nil {
			return 0, err
		} else if power && len(args) == 2 {
			exponent, _, numeral, err := r.numeral(args[1])
			if err != nil {
				return 0, err
			}
			if numeral && exponent.Cmp(big.NewRat(1, 1)) > 0 {
				id, err := r.manager.GetASTID(args[0])
				return int64(id), err
			}
		}
	}
	id, err := r.manager.GetASTID(h)
	return int64(id), err
}

func (r *ArithRewriter) monomialLess(left, right ast.ASTHandle) (bool, error) {
	if !r.arithIneqLHS {
		return r.manager.ASTLess(left, right)
	}
	leftOrdinal, err := r.monomialOrdinal(left)
	if err != nil {
		return false, err
	}
	rightOrdinal, err := r.monomialOrdinal(right)
	return leftOrdinal < rightOrdinal, err
}

func (r *ArithRewriter) expandSumOfMonomials(factors []ast.ASTHandle, coefficient *big.Rat, term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	choices := make([][]ast.ASTHandle, len(factors))
	hasSum := false
	for i, factor := range factors {
		_, children, sum, err := r.operation(factor, arithOpAdd)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if sum {
			choices[i] = children
			hasSum = true
		} else {
			choices[i] = []ast.ASTHandle{factor}
		}
	}
	if !hasSum {
		return ast.ASTHandle{}, false, nil
	}
	sizes := make([]uint32, len(choices))
	coordinates := make([]uint32, len(choices))
	for i := range choices {
		sizes[i] = uint32(len(choices[i]))
	}
	products := make([][]ast.ASTHandle, 0)
	for {
		if uint64(len(products)) > uint64(r.monomialLimit) {
			return ast.ASTHandle{}, false, ErrArithPolynomialMonomialBlowup
		}
		product := make([]ast.ASTHandle, len(choices))
		for i := range choices {
			product[i] = choices[i][coordinates[i]]
		}
		products = append(products, product)
		if !productiter.Next(sizes, coordinates) {
			break
		}
	}
	monomials := make([]ast.ASTHandle, len(products))
	for i, product := range products {
		if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			numeral, err := r.numeralForTerm(coefficient, term)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			product = append([]ast.ASTHandle{numeral}, product...)
		}
		var err error
		monomials[i], err = r.makeMul(product, term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeAdd(monomials, term)
	return result, err == nil, err
}

func (r *ArithRewriter) distributeCoefficient(coefficient *big.Rat, factor, term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if r.hoistCoeff || coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return ast.ASTHandle{}, false, nil
	}
	_, children, sum, err := r.operation(factor, arithOpAdd)
	if err != nil || !sum {
		return ast.ASTHandle{}, false, err
	}
	terms := make([]ast.ASTHandle, len(children))
	for i, child := range children {
		terms[i], err = r.coefficientProduct(coefficient, child, term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeAdd(terms, term)
	return result, err == nil, err
}

func (r *ArithRewriter) hoistEqualCoefficients(terms []ast.ASTHandle, term ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	if !r.hoistCoeff {
		return terms, false, nil
	}
	terms = append([]ast.ASTHandle(nil), terms...)
	var sortErr error
	sort.SliceStable(terms, func(i, j int) bool {
		leftCoefficient, leftProduct, err := r.monomial(terms[i])
		if err != nil {
			sortErr = err
			return false
		}
		rightCoefficient, rightProduct, err := r.monomial(terms[j])
		if err != nil {
			sortErr = err
			return false
		}
		leftMul, rightMul := leftProduct != terms[i], rightProduct != terms[j]
		if leftMul != rightMul {
			return !leftMul
		}
		if !leftMul {
			leftID, err := r.manager.GetASTID(terms[i])
			if err != nil {
				sortErr = err
				return false
			}
			rightID, err := r.manager.GetASTID(terms[j])
			if err != nil {
				sortErr = err
				return false
			}
			return leftID < rightID
		}
		if comparison := leftCoefficient.Cmp(rightCoefficient); comparison != 0 {
			return comparison < 0
		}
		leftID, err := r.manager.GetASTID(leftProduct)
		if err != nil {
			sortErr = err
			return false
		}
		rightID, err := r.manager.GetASTID(rightProduct)
		if err != nil {
			sortErr = err
			return false
		}
		return leftID < rightID
	})
	if sortErr != nil {
		return nil, false, sortErr
	}
	groups := make(map[string][]int)
	coefficients := make(map[string]*big.Rat)
	products := make([]ast.ASTHandle, len(terms))
	for i, item := range terms {
		coefficient, product, err := r.monomial(item)
		if err != nil {
			return nil, false, err
		}
		if product == item || coefficient.Cmp(big.NewRat(1, 1)) == 0 {
			continue
		}
		key := coefficient.RatString()
		groups[key] = append(groups[key], i)
		coefficients[key] = coefficient
		products[i] = product
	}
	consumed := make(map[int]bool)
	result := make([]ast.ASTHandle, 0, len(terms))
	changed := false
	for i, item := range terms {
		if consumed[i] {
			continue
		}
		coefficient, product, err := r.monomial(item)
		if err != nil {
			return nil, false, err
		}
		key := coefficient.RatString()
		indices := groups[key]
		if product == item || len(indices) < 2 || indices[0] != i {
			result = append(result, item)
			continue
		}
		parts := make([]ast.ASTHandle, len(indices))
		for j, index := range indices {
			consumed[index] = true
			parts[j] = products[index]
		}
		sum, err := r.makeAdd(parts, term)
		if err != nil {
			return nil, false, err
		}
		hoisted, err := r.coefficientProduct(coefficients[key], sum, term)
		if err != nil {
			return nil, false, err
		}
		result = append(result, hoisted)
		changed = true
	}
	return result, changed, nil
}

func (r *ArithRewriter) hoistCommonMultiplication(terms []ast.ASTHandle, term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if !r.hoistMul || len(terms) < 2 {
		return ast.ASTHandle{}, false, nil
	}
	factors := func(item ast.ASTHandle) ([]ast.ASTHandle, error) {
		_, values, product, err := r.operation(item, arithOpMul)
		if err != nil {
			return nil, err
		}
		if !product {
			values = []ast.ASTHandle{item}
		}
		return append([]ast.ASTHandle(nil), values...), nil
	}
	merge := func(left, right ast.ASTHandle) (ast.ASTHandle, error) {
		leftFactors, err := factors(left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rightFactors, err := factors(right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		common := make([]ast.ASTHandle, 0)
		leftRest := make([]ast.ASTHandle, 0, len(leftFactors))
		for _, candidate := range leftFactors {
			found := -1
			for i, other := range rightFactors {
				if candidate == other {
					found = i
					break
				}
			}
			if found < 0 {
				leftRest = append(leftRest, candidate)
				continue
			}
			common = append(common, candidate)
			rightFactors = append(rightFactors[:found], rightFactors[found+1:]...)
		}
		leftRemainder, err := r.makeMul(leftRest, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rightRemainder, err := r.makeMul(rightFactors, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sum, err := r.makeAdd([]ast.ASTHandle{leftRemainder, rightRemainder}, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		common = append(common, sum)
		return r.makeMul(common, term)
	}

	resultTerms := append([]ast.ASTHandle(nil), terms...)
	valid := make([]bool, len(terms))
	for i := range valid {
		valid[i] = true
	}
	seen := make(map[ast.ASTHandle]int)
	changed := false
	for i, item := range terms {
		values, err := factors(item)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		for _, factor := range values {
			if _, _, numeral, err := r.numeral(factor); err != nil {
				return ast.ASTHandle{}, false, err
			} else if numeral {
				continue
			}
			previous, found := seen[factor]
			if found && valid[previous] && previous != i {
				resultTerms[previous], err = merge(resultTerms[previous], resultTerms[i])
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				resultTerms[i], err = r.numeralForTerm(big.NewRat(0, 1), term)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				valid[previous] = false
				valid[i] = false
				changed = true
				break
			}
			seen[factor] = i
		}
	}
	if !changed {
		return ast.ASTHandle{}, false, nil
	}
	result, err := r.makeAdd(resultTerms, term)
	return result, err == nil, err
}

func (r *ArithRewriter) flattenArguments(args ast.AppArguments, kind int32, recursive bool) ([]ast.ASTHandle, bool, error) {
	values := make([]ast.ASTHandle, 0, args.Count())
	changed := false
	for i := uint32(0); i < args.Count(); i++ {
		h, err := args.At(i)
		if err != nil {
			return nil, false, err
		}
		_, children, matched, err := r.operation(h, kind)
		if err != nil {
			return nil, false, err
		}
		if !r.flat || !matched {
			values = append(values, h)
			continue
		}
		changed = true
		if !recursive {
			values = append(values, children...)
			continue
		}
		stack := append([]ast.ASTHandle(nil), children...)
		for len(stack) != 0 {
			current := stack[0]
			stack = stack[1:]
			_, nested, yes, e := r.operation(current, kind)
			if e != nil {
				return nil, false, e
			}
			if yes {
				stack = append(append([]ast.ASTHandle(nil), nested...), stack...)
			} else {
				values = append(values, current)
			}
		}
	}
	return values, changed, nil
}

func (r *ArithRewriter) monomial(h ast.ASTHandle) (*big.Rat, ast.ASTHandle, error) {
	_, args, matched, err := r.operation(h, arithOpMul)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if matched && len(args) == 2 {
		coefficient, _, numeral, e := r.numeral(args[0])
		if e != nil {
			return nil, ast.ASTHandle{}, e
		}
		if numeral {
			return coefficient, args[1], nil
		}
	}
	return big.NewRat(1, 1), h, nil
}

func (r *ArithRewriter) coefficientProduct(coefficient *big.Rat, powerProduct, term ast.ASTHandle) (ast.ASTHandle, error) {
	if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return powerProduct, nil
	}
	numeral, err := r.numeralForTerm(coefficient, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpMul, numeral, powerProduct)
}

func (r *ArithRewriter) negateMonomial(h ast.ASTHandle) (ast.ASTHandle, error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if product && len(factors) != 0 {
		coefficient, _, numeral, err := r.numeral(factors[0])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if numeral {
			result := make([]ast.ASTHandle, 0, len(factors))
			if coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
				negative, err := r.numeralForTerm(new(big.Rat).Neg(coefficient), h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				result = append(result, negative)
			}
			result = append(result, factors[1:]...)
			return r.makeMul(result, h)
		}
	}
	minusOne, err := r.numeralForTerm(big.NewRat(-1, 1), h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeMul([]ast.ASTHandle{minusOne, h}, h)
}

func (r *ArithRewriter) negativePolynomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if product && len(factors) != 0 {
		coefficient, _, numeral, err := r.numeral(factors[0])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if numeral && coefficient.Sign() < 0 {
			result, err := r.negateMonomial(h)
			return result, err == nil, err
		}
	}
	_, terms, sum, err := r.operation(h, arithOpAdd)
	if err != nil || !sum || len(terms) == 0 {
		return ast.ASTHandle{}, false, err
	}
	_, firstFactors, firstProduct, err := r.operation(terms[0], arithOpMul)
	if err != nil || !firstProduct || len(firstFactors) == 0 {
		return ast.ASTHandle{}, false, err
	}
	coefficient, _, numeral, err := r.numeral(firstFactors[0])
	if err != nil || !numeral || coefficient.Sign() >= 0 {
		return ast.ASTHandle{}, false, err
	}
	negated := make([]ast.ASTHandle, len(terms))
	for i, item := range terms {
		negated[i], err = r.negateMonomial(item)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeAdd(negated, h)
	return result, err == nil, err
}

func (r *ArithRewriter) rewriteAdd(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	if args.Count() == 1 {
		h, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	values, flattened, err := r.flattenArguments(args, arithOpAdd, false)
	if err != nil {
		return RewriteFailed, err
	}
	first, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	constant := new(big.Rat)
	order := make([]ast.ASTHandle, 0, len(values))
	coefficients := make(map[ast.ASTHandle]*big.Rat)
	counts := make(map[ast.ASTHandle]int)
	for _, value := range values {
		v, _, numeral, e := r.numeral(value)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			constant.Add(constant, v)
			continue
		}
		coefficient, product, e := r.monomial(value)
		if e != nil {
			return RewriteFailed, e
		}
		if counts[product] == 0 {
			order = append(order, product)
			coefficients[product] = new(big.Rat)
		}
		counts[product]++
		coefficients[product].Add(coefficients[product], coefficient)
	}
	resultArgs := make([]ast.ASTHandle, 0, len(order)+1)
	if constant.Sign() != 0 {
		h, e := r.numeralForTerm(constant, first)
		if e != nil {
			return RewriteFailed, e
		}
		resultArgs = append(resultArgs, h)
	}
	duplicate := false
	for _, product := range order {
		duplicate = duplicate || counts[product] > 1
		coefficient := coefficients[product]
		if coefficient.Sign() == 0 {
			continue
		}
		h, e := r.coefficientProduct(coefficient, product, first)
		if e != nil {
			return RewriteFailed, e
		}
		resultArgs = append(resultArgs, h)
	}
	if r.sortSums && len(resultArgs) > 1 {
		start := 0
		if constant.Sign() != 0 {
			start = 1
		}
		var cause error
		sort.SliceStable(resultArgs[start:], func(i, j int) bool {
			less, e := r.monomialLess(resultArgs[start+i], resultArgs[start+j])
			if cause == nil {
				cause = e
			}
			return e == nil && less
		})
		if cause != nil {
			return RewriteFailed, cause
		}
	}
	coefficientHoisted := false
	resultArgs, coefficientHoisted, err = r.hoistEqualCoefficients(resultArgs, first)
	if err != nil {
		return RewriteFailed, err
	}
	if result, hoisted, e := r.hoistCommonMultiplication(resultArgs, first); e != nil {
		return RewriteFailed, e
	} else if hoisted {
		return RewriteFull, r.manager.SetRef(out, result)
	}
	result, err := r.makeAdd(resultArgs, first)
	if err != nil {
		return RewriteFailed, err
	}
	same, err := r.sameArguments(args, resultArgs)
	if err != nil {
		return RewriteFailed, err
	}
	if !flattened && !duplicate && !coefficientHoisted && same {
		return RewriteFailed, nil
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteMul(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	if args.Count() == 1 {
		h, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	allowFlatten := true
	if args.Count() == 2 {
		left, e := args.At(0)
		if e != nil {
			return RewriteFailed, e
		}
		_, _, numeral, e := r.numeral(left)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			right, e := args.At(1)
			if e != nil {
				return RewriteFailed, e
			}
			_, nested, product, e := r.operation(right, arithOpMul)
			if e != nil {
				return RewriteFailed, e
			}
			allowFlatten = product && len(nested) != 0
			if allowFlatten {
				_, _, nestedNumeral, e := r.numeral(nested[0])
				if e != nil {
					return RewriteFailed, e
				}
				allowFlatten = nestedNumeral
			}
		}
	}
	var values []ast.ASTHandle
	var flattened bool
	var err error
	if allowFlatten {
		values, flattened, err = r.flattenArguments(args, arithOpMul, true)
	} else {
		values = make([]ast.ASTHandle, args.Count())
		for i := range values {
			values[i], err = args.At(uint32(i))
			if err != nil {
				return RewriteFailed, err
			}
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	first, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	coefficient := big.NewRat(1, 1)
	factors := make([]ast.ASTHandle, 0, len(values))
	for _, value := range values {
		v, _, numeral, e := r.numeral(value)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			coefficient.Mul(coefficient, v)
		} else {
			factors = append(factors, value)
		}
	}
	if coefficient.Sign() == 0 || len(factors) == 0 {
		h, e := r.numeralForTerm(coefficient, first)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if len(factors) == 1 {
		if result, distributed, e := r.distributeCoefficient(coefficient, factors[0], first); e != nil {
			return RewriteFailed, e
		} else if distributed {
			return Rewrite2, r.manager.SetRef(out, result)
		}
	}
	if r.sumOfMonomials && len(factors) > 1 {
		if result, expanded, e := r.expandSumOfMonomials(factors, coefficient, first); e != nil {
			return RewriteFailed, e
		} else if expanded {
			return Rewrite2, r.manager.SetRef(out, result)
		}
	}
	var cause error
	sort.SliceStable(factors, func(i, j int) bool {
		less, e := r.monomialLess(factors[i], factors[j])
		if cause == nil {
			cause = e
		}
		return e == nil && less
	})
	if cause != nil {
		return RewriteFailed, cause
	}
	combined := make([]ast.ASTHandle, 0, len(factors)+1)
	if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		n, e := r.numeralForTerm(coefficient, first)
		if e != nil {
			return RewriteFailed, e
		}
		combined = append(combined, n)
	}
	if r.mulToPower && !r.expandPower {
		for i := 0; i < len(factors); {
			body, exponent, e := r.powerBody(factors[i])
			if e != nil {
				return RewriteFailed, e
			}
			j := i + 1
			for j < len(factors) {
				nextBody, nextExponent, e := r.powerBody(factors[j])
				if e != nil {
					return RewriteFailed, e
				}
				if nextBody != body {
					break
				}
				exponent.Add(exponent, nextExponent)
				j++
			}
			if exponent.Cmp(big.NewRat(1, 1)) == 0 {
				combined = append(combined, body)
			} else {
				exponentNode, e := r.numeralForTerm(exponent, first)
				if e != nil {
					return RewriteFailed, e
				}
				power, e := r.app(arithOpPower, body, exponentNode)
				if e != nil {
					return RewriteFailed, e
				}
				combined = append(combined, power)
			}
			i = j
		}
	} else {
		combined = append(combined, factors...)
	}
	result, err := r.makeMul(combined, first)
	if err != nil {
		return RewriteFailed, err
	}
	same, err := r.sameArguments(args, combined)
	if err != nil {
		return RewriteFailed, err
	}
	if !flattened && same {
		return RewriteFailed, nil
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteSub(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	first, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	if args.Count() == 1 {
		return RewriteDone, r.manager.SetRef(out, first)
	}
	minusOne, err := r.numeralForTerm(big.NewRat(-1, 1), first)
	if err != nil {
		return RewriteFailed, err
	}
	values := []ast.ASTHandle{first}
	for i := uint32(1); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		v, _, numeral, e := r.numeral(h)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral && v.Sign() == 0 {
			continue
		}
		negative, e := r.app(arithOpMul, minusOne, h)
		if e != nil {
			return RewriteFailed, e
		}
		values = append(values, negative)
	}
	result, err := r.makeAdd(values, first)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) monomials(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil {
		return nil, err
	}
	if add {
		return args, nil
	}
	return []ast.ASTHandle{h}, nil
}

func (r *ArithRewriter) cancelMonomials(left, right ast.ASTHandle, move bool) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	sides := [2][]ast.ASTHandle{}
	var err error
	sides[0], err = r.monomials(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	sides[1], err = r.monomials(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	constant := new(big.Rat)
	constantCount := 0
	leftProducts := make(map[ast.ASTHandle]bool)
	shared := make(map[ast.ASTHandle]bool)
	for _, term := range sides[0] {
		value, _, numeral, e := r.numeral(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if numeral {
			constant.Add(constant, value)
			constantCount++
			continue
		}
		_, product, e := r.monomial(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		leftProducts[product] = true
	}
	if move && constantCount == 0 {
		if _, _, numeral, e := r.numeral(right); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
		}
	}
	for _, term := range sides[1] {
		value, _, numeral, e := r.numeral(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if numeral {
			constant.Sub(constant, value)
			constantCount++
			continue
		}
		_, product, e := r.monomial(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if leftProducts[product] {
			shared[product] = true
		}
	}
	if len(shared) == 0 && constantCount <= 1 {
		_, _, rightNumeral, e := r.numeral(right)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if move && rightNumeral || !move && (constantCount == 0 || rightNumeral) {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
		}
	}
	sharedCoefficients := make(map[ast.ASTHandle]*big.Rat)
	for side := range sides {
		for _, term := range sides[side] {
			if _, _, numeral, e := r.numeral(term); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			} else if numeral {
				continue
			}
			coefficient, product, e := r.monomial(term)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			if !shared[product] {
				continue
			}
			if sharedCoefficients[product] == nil {
				sharedCoefficients[product] = new(big.Rat)
			}
			if side == 0 {
				sharedCoefficients[product].Add(sharedCoefficients[product], coefficient)
			} else {
				sharedCoefficients[product].Sub(sharedCoefficients[product], coefficient)
			}
		}
	}
	lhs := make([]ast.ASTHandle, 0, len(sides[0])+len(sides[1]))
	rhs := make([]ast.ASTHandle, 0, len(sides[1]))
	seen := make(map[ast.ASTHandle]bool)
	for _, term := range sides[0] {
		if _, _, numeral, e := r.numeral(term); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			continue
		}
		_, product, e := r.monomial(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if !shared[product] {
			lhs = append(lhs, term)
			continue
		}
		if seen[product] {
			continue
		}
		seen[product] = true
		coefficient := sharedCoefficients[product]
		if coefficient.Sign() != 0 {
			h, e := r.coefficientProduct(coefficient, product, left)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			lhs = append(lhs, h)
		}
	}
	for _, term := range sides[1] {
		if _, _, numeral, e := r.numeral(term); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			continue
		}
		coefficient, product, e := r.monomial(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if shared[product] {
			continue
		}
		if !move {
			rhs = append(rhs, term)
			continue
		}
		coefficient.Neg(coefficient)
		h, e := r.coefficientProduct(coefficient, product, left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		lhs = append(lhs, h)
	}
	constantAtRight := move || len(rhs) == 0 || len(lhs) != 0
	if constantAtRight {
		constant.Neg(constant)
	}
	if constant.Sign() != 0 || constantAtRight && len(rhs) == 0 || !constantAtRight && len(lhs) == 0 {
		h, e := r.numeralForTerm(constant, left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if constantAtRight {
			rhs = append([]ast.ASTHandle{h}, rhs...)
		} else {
			lhs = append([]ast.ASTHandle{h}, lhs...)
		}
	}
	newLeft, err := r.makeAdd(lhs, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	newRight, err := r.makeAdd(rhs, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return newLeft, newRight, newLeft != left || newRight != right, nil
}

func invertArithmeticRelation(kind int32) int32 {
	if kind == arithOpLE {
		return arithOpGE
	}
	if kind == arithOpGE {
		return arithOpLE
	}
	return kind
}

func (r *ArithRewriter) rewriteBound(left, right ast.ASTHandle, kind int32, out ast.ASTRef) (bool, error) {
	_, _, add, err := r.operation(left, arithOpAdd)
	if err != nil {
		return false, err
	}
	_, _, mod, err := r.operation(left, arithOpMod)
	if err != nil {
		return false, err
	}
	bound, _, rightNumeral, err := r.numeral(right)
	if err != nil {
		return false, err
	}
	if !add && !mod && rightNumeral {
		coefficient, product, e := r.monomial(left)
		if e != nil {
			return false, e
		}
		relation := kind
		changed := false
		if coefficient.Sign() < 0 {
			coefficient.Neg(coefficient)
			bound.Neg(bound)
			relation = invertArithmeticRelation(relation)
			changed = true
		}
		if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			bound.Quo(bound, coefficient)
			changed = true
		}
		if changed {
			integer, e := r.integerTerm(left)
			if e != nil {
				return false, e
			}
			if integer && !bound.IsInt() {
				switch relation {
				case arithOpLE:
					bound.SetInt(arithFloor(bound))
				case arithOpGE:
					floor := arithFloor(bound)
					floor.Add(floor, big.NewInt(1))
					bound.SetInt(floor)
				default:
					return true, r.setBool(out, false)
				}
			}
			value, e := r.makeNumeral(bound, integer)
			if e != nil {
				return false, e
			}
			var result ast.ASTHandle
			if relation == arithRelationEq {
				result, e = r.manager.MkEq(product, value)
			} else {
				result, e = r.app(relation, product, value)
			}
			if e != nil {
				return false, e
			}
			return true, r.manager.SetRef(out, result)
		}
	}
	if !rightNumeral {
		return false, nil
	}
	_, modArgs, mod, err := r.operation(left, arithOpMod)
	if err != nil || !mod || len(modArgs) != 2 {
		return false, err
	}
	modulus, integer, numeral, err := r.numeral(modArgs[1])
	if err != nil || !numeral || !integer || modulus.Sign() == 0 {
		return false, err
	}
	truth, decided := false, false
	if bound.Sign() < 0 {
		decided = true
		truth = kind == arithOpGE
	} else if bound.Sign() == 0 && kind == arithOpGE {
		decided, truth = true, true
	} else {
		absolute := new(big.Rat).Abs(modulus)
		if bound.Sign() > 0 && bound.Cmp(absolute) >= 0 {
			decided = true
			truth = kind == arithOpLE
		} else if kind == arithOpLE && new(big.Rat).Add(bound, big.NewRat(1, 1)).Cmp(absolute) == 0 {
			decided, truth = true, true
		}
	}
	if !decided {
		return false, nil
	}
	return true, r.setBool(out, truth)
}

func (r *ArithRewriter) zero(h ast.ASTHandle) (bool, error) {
	value, _, numeral, err := r.numeral(h)
	return err == nil && numeral && value.Sign() == 0, err
}

func (r *ArithRewriter) reducePowerFactors(h ast.ASTHandle, equality bool) (ast.ASTHandle, bool, error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !product {
		factors = []ast.ASTHandle{h}
	}
	changed := false
	result := make([]ast.ASTHandle, len(factors))
	for i, factor := range factors {
		result[i] = factor
		_, powerArgs, power, e := r.operation(factor, arithOpPower)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !power || len(powerArgs) != 2 {
			continue
		}
		exponent, _, numeral, e := r.numeral(powerArgs[1])
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		threshold := int64(2)
		if equality {
			threshold = 1
		}
		if !numeral || !exponent.IsInt() || !exponent.Num().IsInt64() || exponent.Num().Int64() <= threshold {
			continue
		}
		changed = true
		if equality || exponent.Num().Bit(0) == 1 {
			result[i] = powerArgs[0]
			continue
		}
		two, e := r.numeralForTerm(big.NewRat(2, 1), factor)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		result[i], e = r.app(arithOpPower, powerArgs[0], two)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	if !changed {
		return h, false, nil
	}
	reduced, err := r.makeMul(result, h)
	return reduced, true, err
}

func (r *ArithRewriter) reduceComparisonPowers(left, right ast.ASTHandle, equality bool) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	leftZero, err := r.zero(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	rightZero, err := r.zero(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if !leftZero && !rightZero {
		return left, right, false, nil
	}
	newLeft, leftChanged, err := r.reducePowerFactors(left, equality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	newRight, rightChanged, err := r.reducePowerFactors(right, equality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return newLeft, newRight, leftChanged || rightChanged, nil
}

func (r *ArithRewriter) eliminateToRealVariable(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	value, _, numeral, err := r.numeral(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if numeral {
		if !value.IsInt() {
			return ast.ASTHandle{}, false, nil
		}
		result, e := r.makeNumeral(value, true)
		return result, e == nil, e
	}
	if yes, inner, e := r.unary(h, arithOpToReal); e != nil {
		return ast.ASTHandle{}, false, e
	} else if yes {
		return inner, true, nil
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) eliminateToRealMonomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !product {
		return r.eliminateToRealVariable(h)
	}
	converted := make([]ast.ASTHandle, len(factors))
	for i, factor := range factors {
		var ok bool
		converted[i], ok, err = r.eliminateToRealVariable(factor)
		if err != nil || !ok {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeMul(converted, converted[0])
	return result, err == nil, err
}

func (r *ArithRewriter) eliminateToRealPolynomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, monomials, sum, err := r.operation(h, arithOpAdd)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !sum {
		return r.eliminateToRealMonomial(h)
	}
	converted := make([]ast.ASTHandle, len(monomials))
	for i, monomial := range monomials {
		var ok bool
		converted[i], ok, err = r.eliminateToRealMonomial(monomial)
		if err != nil || !ok {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeAdd(converted, converted[0])
	return result, err == nil, err
}

func (r *ArithRewriter) eliminateToRealComparison(left, right ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	integer, err := r.integerTerm(left)
	if err != nil || integer {
		return left, right, false, err
	}
	newLeft, leftOK, err := r.eliminateToRealPolynomial(left)
	if err != nil || !leftOK {
		return left, right, false, err
	}
	newRight, rightOK, err := r.eliminateToRealPolynomial(right)
	if err != nil || !rightOK {
		return left, right, false, err
	}
	return newLeft, newRight, true, nil
}

func arithCeil(value *big.Rat) *big.Int {
	negative := new(big.Rat).Neg(value)
	return new(big.Int).Neg(arithFloor(negative))
}

func (r *ArithRewriter) polynomialCoefficientGCD(h ast.ASTHandle, gcd *big.Int, first *bool, constants *int) error {
	terms, err := r.monomials(h)
	if err != nil {
		return err
	}
	for _, term := range terms {
		value, _, numeral, e := r.numeral(term)
		if e != nil {
			return e
		}
		if numeral {
			if value.Sign() != 0 {
				*constants++
			}
			continue
		}
		coefficient, _, e := r.monomial(term)
		if e != nil {
			return e
		}
		if !coefficient.IsInt() {
			return ast.ErrASTDomain
		}
		absolute := new(big.Int).Abs(coefficient.Num())
		if *first {
			gcd.Set(absolute)
			*first = false
		} else {
			gcd.GCD(nil, nil, gcd, absolute)
		}
		if gcd.Cmp(big.NewInt(1)) == 0 {
			return nil
		}
	}
	return nil
}

func (r *ArithRewriter) dividePolynomial(h ast.ASTHandle, divisor *big.Int, treatment int) (ast.ASTHandle, bool, error) {
	terms, err := r.monomials(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result := make([]ast.ASTHandle, 0, len(terms))
	denominator := new(big.Rat).SetInt(divisor)
	for _, term := range terms {
		value, _, numeral, e := r.numeral(term)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if numeral {
			value.Quo(value, denominator)
			if !value.IsInt() {
				switch treatment {
				case arithDivideFloor:
					value.SetInt(arithFloor(value))
				case arithDivideCeil:
					value.SetInt(arithCeil(value))
				default:
					return ast.ASTHandle{}, false, nil
				}
			}
			if value.Sign() != 0 {
				n, e := r.numeralForTerm(value, h)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				result = append(result, n)
			}
			continue
		}
		coefficient, product, e := r.monomial(term)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		coefficient.Quo(coefficient, denominator)
		if !coefficient.IsInt() {
			return ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		if coefficient.Sign() != 0 {
			monomial, e := r.coefficientProduct(coefficient, product, h)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			result = append(result, monomial)
		}
	}
	quotient, err := r.makeAdd(result, h)
	return quotient, err == nil, err
}

func (r *ArithRewriter) reducePolynomialGCD(left, right ast.ASTHandle, relation int32) (ast.ASTHandle, ast.ASTHandle, bool, bool, error) {
	integer, err := r.integerTerm(left)
	if err != nil || !integer {
		return left, right, false, false, err
	}
	gcd := new(big.Int)
	first, constants := true, 0
	if err = r.polynomialCoefficientGCD(left, gcd, &first, &constants); err != nil {
		return left, right, false, false, err
	}
	if (first || gcd.Cmp(big.NewInt(1)) != 0) && constants <= 1 {
		if err = r.polynomialCoefficientGCD(right, gcd, &first, &constants); err != nil {
			return left, right, false, false, err
		}
	}
	if first || gcd.Cmp(big.NewInt(1)) == 0 || constants > 1 {
		return left, right, false, false, nil
	}
	leftTreatment, rightTreatment := arithDivideExact, arithDivideExact
	if relation == arithOpLE {
		leftTreatment, rightTreatment = arithDivideCeil, arithDivideFloor
	} else if relation == arithOpGE {
		leftTreatment, rightTreatment = arithDivideFloor, arithDivideCeil
	}
	newLeft, possible, err := r.dividePolynomial(left, gcd, leftTreatment)
	if err != nil || !possible {
		return left, right, false, !possible, err
	}
	newRight, possible, err := r.dividePolynomial(right, gcd, rightTreatment)
	if err != nil || !possible {
		return left, right, false, !possible, err
	}
	return newLeft, newRight, true, false, nil
}
