package transforms

// Source: src/muz/transforms/dl_mk_interp_tail_simplifier.cpp normalizer_cfg
// and normalizer_rw. This is the source-specific Boolean pass between the two
// general theory-rewriter invocations.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type booleanNormalizer struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	memo    map[ast.ASTHandle]ast.ASTHandle
}

func newBooleanNormalizer(manager *ast.ASTManager) *booleanNormalizer {
	return &booleanNormalizer{manager: manager, memo: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (n *booleanNormalizer) close() (result error) {
	for index := len(n.refs) - 1; index >= 0; index-- {
		result = errors.Join(result, n.manager.DelRef(&n.refs[index]))
	}
	n.refs = nil
	return result
}

func (n *booleanNormalizer) keep(handle ast.ASTHandle) (ast.ASTHandle, error) {
	reference, err := n.manager.NewRef(handle)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n.refs = append(n.refs, reference)
	return handle, nil
}

func (n *booleanNormalizer) operation(handle ast.ASTHandle) (int32, bool, error) {
	kind, err := n.manager.GetKind(handle)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := n.manager.GetFamilyID(handle)
	if errors.Is(err, ast.ErrASTDomain) {
		// Plain uninterpreted declarations have no family metadata.
		return -1, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	if family != n.manager.BasicFamilyID() {
		return -1, true, nil
	}
	operation, err := n.manager.GetDeclKind(handle)
	return operation, err == nil, err
}

func (n *booleanNormalizer) compare(left, right ast.ASTHandle, depth int) (int, error) {
	if left == right {
		return 0, nil
	}
	leftNegative, leftBare, err := n.manager.IsNot(left)
	if err != nil {
		return 0, err
	}
	if leftNegative {
		left = leftBare
	}
	rightNegative, rightBare, err := n.manager.IsNot(right)
	if err != nil {
		return 0, err
	}
	if rightNegative {
		right = rightBare
	}
	if left == right {
		if leftNegative == rightNegative {
			return 0, nil
		}
		if !leftNegative {
			return -1, nil
		}
		return 1, nil
	}
	leftKind, err := n.manager.GetKind(left)
	if err != nil {
		return 0, err
	}
	rightKind, err := n.manager.GetKind(right)
	if err != nil {
		return 0, err
	}
	if leftKind != ast.ASTApp || rightKind != ast.ASTApp {
		if leftKind != ast.ASTApp && rightKind == ast.ASTApp {
			return -1, nil
		}
		if leftKind == ast.ASTApp && rightKind != ast.ASTApp {
			return 1, nil
		}
		leftID, e := n.manager.GetASTID(left)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(right)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	leftDecl, err := n.manager.GetDecl(left)
	if err != nil {
		return 0, err
	}
	rightDecl, err := n.manager.GetDecl(right)
	if err != nil {
		return 0, err
	}
	if leftDecl != rightDecl {
		leftID, e := n.manager.GetASTID(leftDecl)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(rightDecl)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	leftCount, err := n.manager.GetNumArgs(left)
	if err != nil {
		return 0, err
	}
	rightCount, err := n.manager.GetNumArgs(right)
	if err != nil {
		return 0, err
	}
	if leftCount != rightCount {
		return compareUint32(leftCount, rightCount), nil
	}
	if depth == 0 {
		leftID, e := n.manager.GetASTID(left)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(right)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	negationComparison := 0
	for index := uint32(0); index < leftCount; index++ {
		leftArg, e := n.manager.GetArg(left, index)
		if e != nil {
			return 0, e
		}
		rightArg, e := n.manager.GetArg(right, index)
		if e != nil {
			return 0, e
		}
		leftNegated, leftArgBare, e := n.manager.IsNot(leftArg)
		if e != nil {
			return 0, e
		}
		if leftNegated {
			leftArg = leftArgBare
		}
		rightNegated, rightArgBare, e := n.manager.IsNot(rightArg)
		if e != nil {
			return 0, e
		}
		if rightNegated {
			rightArg = rightArgBare
		}
		if negationComparison == 0 && leftNegated != rightNegated {
			if leftNegated {
				negationComparison = -1
			} else {
				negationComparison = 1
			}
		}
		comparison, e := n.compare(leftArg, rightArg, depth-1)
		if e != nil || comparison != 0 {
			return comparison, e
		}
	}
	if negationComparison != 0 {
		return negationComparison, nil
	}
	leftID, err := n.manager.GetASTID(left)
	if err != nil {
		return 0, err
	}
	rightID, err := n.manager.GetASTID(right)
	if err != nil {
		return 0, err
	}
	return compareUint32(leftID, rightID), nil
}

func compareUint32(left, right uint32) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

type booleanPair struct {
	first  ast.ASTHandle
	second ast.ASTHandle
}

func (n *booleanNormalizer) pair(expression ast.ASTHandle, operation int32) (booleanPair, bool, error) {
	op, app, err := n.operation(expression)
	if err != nil || !app || op != operation {
		return booleanPair{}, false, err
	}
	count, err := n.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return booleanPair{}, false, err
	}
	first, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return booleanPair{}, false, err
	}
	second, err := n.manager.GetArg(expression, 1)
	return booleanPair{first: first, second: second}, err == nil, err
}

func (n *booleanNormalizer) equivalence(first, second booleanPair, insideDisjunction bool) (ast.ASTHandle, bool, error) {
	firstLeftNegated, firstLeft, err := n.manager.IsNot(first.first)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !firstLeftNegated {
		firstLeft = first.first
	}
	secondLeftNegated, secondLeft, err := n.manager.IsNot(second.first)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !secondLeftNegated {
		secondLeft = second.first
	}
	firstRightNegated, firstRight, err := n.manager.IsNot(first.second)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !firstRightNegated {
		firstRight = first.second
	}
	secondRightNegated, secondRight, err := n.manager.IsNot(second.second)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !secondRightNegated {
		secondRight = second.second
	}
	if firstLeftNegated == secondLeftNegated || firstRightNegated == secondRightNegated || firstLeft != secondLeft || firstRight != secondRight {
		return ast.ASTHandle{}, false, nil
	}
	boolean, err := n.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	leftSort, err := n.manager.GetSort(firstLeft)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rightSort, err := n.manager.GetSort(firstRight)
	if err != nil || leftSort != boolean || rightSort != boolean {
		return ast.ASTHandle{}, false, err
	}
	negsTogether := firstLeftNegated == firstRightNegated
	right := firstRight
	if negsTogether != insideDisjunction {
		right, err = n.manager.MkNotSimplified(right)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if right, err = n.keep(right); err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	equivalence, err := n.manager.MkEq(firstLeft, right)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	equivalence, err = n.keep(equivalence)
	return equivalence, err == nil, err
}

func (n *booleanNormalizer) normalize(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if cached, found := n.memo[expression]; found {
		return cached, nil
	}
	operation, app, err := n.operation(expression)
	if err != nil || !app {
		return expression, err
	}
	count, err := n.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, count)
	changed := false
	for index := uint32(0); index < count; index++ {
		argument, e := n.manager.GetArg(expression, index)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[index], e = n.normalize(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || arguments[index] != argument
	}
	if operation == ast.ASTOpNot && len(arguments) == 1 {
		childOperation, childApp, e := n.operation(arguments[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if childApp && (childOperation == ast.ASTOpAnd || childOperation == ast.ASTOpOr) {
			childCount, e := n.manager.GetNumArgs(arguments[0])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			negated := make([]ast.ASTHandle, childCount)
			for index := uint32(0); index < childCount; index++ {
				child, x := n.manager.GetArg(arguments[0], index)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				negated[index], x = n.manager.MkNotSimplified(child)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				if negated[index], x = n.keep(negated[index]); x != nil {
					return ast.ASTHandle{}, x
				}
			}
			var demorgan ast.ASTHandle
			if childOperation == ast.ASTOpAnd {
				demorgan, e = n.manager.MkOrArgs(negated)
			} else {
				demorgan, e = n.manager.MkAndArgs(negated)
			}
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if demorgan, e = n.keep(demorgan); e != nil {
				return ast.ASTHandle{}, e
			}
			result, e := n.normalize(demorgan)
			if e == nil {
				n.memo[expression] = result
			}
			return result, e
		}
	}
	if operation != ast.ASTOpAnd && operation != ast.ASTOpOr {
		if !changed {
			n.memo[expression] = expression
			return expression, nil
		}
		declaration, e := n.manager.GetDecl(expression)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e := n.manager.MkApp(declaration, arguments)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e = n.keep(result)
		if e == nil {
			n.memo[expression] = result
		}
		return result, e
	}
	var compareErr error
	sort.Slice(arguments, func(left, right int) bool {
		if compareErr != nil {
			return false
		}
		comparison, e := n.compare(arguments[left], arguments[right], 4)
		if e != nil {
			compareErr = e
			return false
		}
		return comparison < 0
	})
	if compareErr != nil {
		return ast.ASTHandle{}, compareErr
	}
	unique := arguments[:0]
	for _, argument := range arguments {
		if len(unique) == 0 || unique[len(unique)-1] != argument {
			unique = append(unique, argument)
		}
	}
	arguments = unique
	pairOperation := int32(ast.ASTOpOr)
	insideDisjunction := operation == ast.ASTOpOr
	if insideDisjunction {
		pairOperation = ast.ASTOpAnd
	}
	resultArguments := make([]ast.ASTHandle, 0, len(arguments))
	havePair := false
	var priorPair booleanPair
	priorIndex := 0
	for _, argument := range arguments {
		currentPair, matched, e := n.pair(argument, pairOperation)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if matched && havePair {
			equivalence, found, x := n.equivalence(priorPair, currentPair, insideDisjunction)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if found {
				resultArguments[priorIndex] = equivalence
				havePair = false
				continue
			}
		}
		if matched {
			havePair = true
			priorPair = currentPair
			priorIndex = len(resultArguments)
		} else {
			havePair = false
		}
		resultArguments = append(resultArguments, argument)
	}
	arguments = resultArguments
	var result ast.ASTHandle
	switch len(arguments) {
	case 0:
		result, err = n.manager.MkBoolVal(operation == ast.ASTOpAnd)
	case 1:
		result = arguments[0]
	default:
		if operation == ast.ASTOpAnd {
			result, err = n.manager.MkAndArgs(arguments)
		} else {
			result, err = n.manager.MkOrArgs(arguments)
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if result != expression {
		if result, err = n.keep(result); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	n.memo[expression] = result
	return result, nil
}

func normalizeBooleanExpression(manager *ast.ASTManager, expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	normalizer := newBooleanNormalizer(manager)
	defer func() { resultErr = errors.Join(resultErr, normalizer.close()) }()
	result, err := normalizer.normalize(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(result)
}
