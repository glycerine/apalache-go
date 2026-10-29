package transforms

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp/.h. The retained Go
// implementation evaluates Horn clauses directly over the concrete Karr
// relation instead of recreating the generic C++ relation-plugin framework.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type KarrInvariantsPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	priority   uint32
	cancelled  bool
	closed     bool
	invariants map[ast.ASTHandle]rel.KarrMatrix
	model      map[ast.ASTHandle]rel.KarrMatrix
}

func NewKarrInvariantsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *KarrInvariantsPlugin {
	if priority == 0 {
		priority = 36010
	}
	return &KarrInvariantsPlugin{manager: manager, rules: rules, priority: priority}
}

func (p *KarrInvariantsPlugin) Priority() uint32            { return p.priority }
func (p *KarrInvariantsPlugin) CanDestratifyNegation() bool { return false }
func (p *KarrInvariantsPlugin) Cancel()                     { p.cancelled = true }

func (p *KarrInvariantsPlugin) proceed() bool { return p != nil && !p.cancelled && !p.closed }

func (p *KarrInvariantsPlugin) ModelInvariants() map[ast.ASTHandle]rel.KarrMatrix {
	result := make(map[ast.ASTHandle]rel.KarrMatrix, len(p.model))
	for declaration, matrix := range p.model {
		result[declaration] = rel.CloneKarrMatrix(matrix)
	}
	return result
}

func (p *KarrInvariantsPlugin) predicateOrder(source *base.RuleSet) ([]ast.ASTHandle, error) {
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	seen := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		declaration, declarationErr := rule.HeadDeclaration()
		getErr = errors.Join(declarationErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if !seen[declaration] {
			seen[declaration] = true
			result = append(result, declaration)
		}
	}
	return result, nil
}

func (p *KarrInvariantsPlugin) relationMap(source *base.RuleSet) (map[ast.ASTHandle]*rel.KarrRelation, []ast.ASTHandle, error) {
	order, err := p.predicateOrder(source)
	if err != nil {
		return nil, nil, err
	}
	result := make(map[ast.ASTHandle]*rel.KarrRelation, len(order))
	for _, declaration := range order {
		arity, arityErr := p.manager.GetArity(declaration)
		if arityErr != nil {
			return nil, nil, arityErr
		}
		result[declaration] = rel.NewKarrEmpty(arity, p.proceed)
	}
	return result, order, nil
}

func (p *KarrInvariantsPlugin) addPredicateApplication(target *rel.KarrRelation, application ast.ASTHandle, matrix rel.KarrMatrix) error {
	arguments, err := p.manager.GetNumArgs(application)
	if err != nil {
		return err
	}
	if arguments != uint32(len(matrix.A[0])) {
		return rel.ErrKarrDimension
	}
	for i := range matrix.A {
		row := make([]big.Rat, target.Dimension())
		constant := new(big.Rat).Set(&matrix.B[i])
		processed := true
		for column := uint32(0); column < arguments && processed; column++ {
			if matrix.A[i][column].Sign() == 0 {
				continue
			}
			argument, getErr := p.manager.GetArg(application, column)
			if getErr != nil {
				return getErr
			}
			processed, getErr = p.addLinear(argument, &matrix.A[i][column], row, constant)
			if getErr != nil {
				return getErr
			}
		}
		if processed {
			if err = target.AddConstraint(row, constant, matrix.Eq[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *KarrInvariantsPlugin) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	text, integer, matched, err := p.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched || !integer {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (p *KarrInvariantsPlugin) isIntegerExpression(expression ast.ASTHandle) (bool, error) {
	sort, err := p.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	arithmetic, err := p.manager.RegisterArithmeticPlugin()
	if err != nil {
		return false, err
	}
	kind, err := p.manager.GetDeclKind(sort)
	return family == arithmetic && kind == 1, err
}

func addRatAt(row []big.Rat, index uint32, value *big.Rat) bool {
	if uint64(index) >= uint64(len(row)) {
		return false
	}
	row[index].Add(&row[index], value)
	return true
}

func (p *KarrInvariantsPlugin) addLinear(expression ast.ASTHandle, multiplier *big.Rat, row []big.Rat, constant *big.Rat) (bool, error) {
	integer, err := p.isIntegerExpression(expression)
	if err != nil || !integer {
		return false, err
	}
	kind, err := p.manager.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		index, indexErr := p.manager.GetVarIndex(expression)
		return addRatAt(row, index, multiplier), indexErr
	}
	if value, matched, numeralErr := p.numeral(expression); numeralErr != nil || matched {
		if numeralErr == nil {
			constant.Add(constant, new(big.Rat).Mul(multiplier, value))
		}
		return matched, numeralErr
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	arithmetic, err := p.manager.RegisterArithmeticPlugin()
	if err != nil || family != arithmetic {
		return false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	switch op {
	case 6: // add
		for i := uint32(0); i < count; i++ {
			argument, getErr := p.manager.GetArg(expression, i)
			if getErr != nil {
				return false, getErr
			}
			matched, getErr := p.addLinear(argument, multiplier, row, constant)
			if getErr != nil || !matched {
				return false, getErr
			}
		}
		return true, nil
	case 7: // sub
		if count != 2 {
			return false, nil
		}
		left, leftErr := p.manager.GetArg(expression, 0)
		right, rightErr := p.manager.GetArg(expression, 1)
		if err = errors.Join(leftErr, rightErr); err != nil {
			return false, err
		}
		matched, err := p.addLinear(left, multiplier, row, constant)
		if err != nil || !matched {
			return false, err
		}
		return p.addLinear(right, new(big.Rat).Neg(multiplier), row, constant)
	case 8: // unary minus
		if count != 1 {
			return false, nil
		}
		argument, getErr := p.manager.GetArg(expression, 0)
		if getErr != nil {
			return false, getErr
		}
		return p.addLinear(argument, new(big.Rat).Neg(multiplier), row, constant)
	case 9: // multiplication by a numeral
		if count != 2 {
			return false, nil
		}
		left, leftErr := p.manager.GetArg(expression, 0)
		right, rightErr := p.manager.GetArg(expression, 1)
		if err = errors.Join(leftErr, rightErr); err != nil {
			return false, err
		}
		coefficient, matched, numeralErr := p.numeral(left)
		term := right
		if numeralErr != nil {
			return false, numeralErr
		}
		if !matched {
			coefficient, matched, numeralErr = p.numeral(right)
			term = left
		}
		if numeralErr != nil || !matched {
			return false, numeralErr
		}
		return p.addLinear(term, new(big.Rat).Mul(multiplier, coefficient), row, constant)
	default:
		return false, nil
	}
}

func (p *KarrInvariantsPlugin) basicOperation(expression ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != 0 {
		return false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	return op == operation, err
}

func (p *KarrInvariantsPlugin) arithmeticOperation(expression ast.ASTHandle, operations ...int32) (int32, bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return 0, false, err
	}
	arithmetic, err := p.manager.RegisterArithmeticPlugin()
	if err != nil || family != arithmetic {
		return 0, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil {
		return 0, false, err
	}
	for _, expected := range operations {
		if op == expected {
			return op, true, nil
		}
	}
	return op, false, nil
}

func (p *KarrInvariantsPlugin) addComparison(target *rel.KarrRelation, left, right ast.ASTHandle, leftMultiplier, rightMultiplier, adjustment int64, equal bool) (bool, error) {
	row := make([]big.Rat, target.Dimension())
	constant := new(big.Rat).SetInt64(adjustment)
	matched, err := p.addLinear(left, big.NewRat(leftMultiplier, 1), row, constant)
	if err != nil || !matched {
		return false, err
	}
	matched, err = p.addLinear(right, big.NewRat(rightMultiplier, 1), row, constant)
	if err != nil || !matched {
		return false, err
	}
	return true, target.AddConstraint(row, constant, equal)
}

func (p *KarrInvariantsPlugin) flattenAnd(expression ast.ASTHandle, destination *[]ast.ASTHandle) error {
	conjunction, err := p.basicOperation(expression, ast.ASTOpAnd)
	if err != nil {
		return err
	}
	if !conjunction {
		*destination = append(*destination, expression)
		return nil
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(expression, i)
		if getErr != nil {
			return getErr
		}
		if getErr = p.flattenAnd(argument, destination); getErr != nil {
			return getErr
		}
	}
	return nil
}

func (p *KarrInvariantsPlugin) variableEquality(expression ast.ASTHandle) (uint32, *big.Rat, bool, error) {
	equality, err := p.basicOperation(expression, ast.ASTOpEq)
	if err != nil || !equality {
		return 0, nil, false, err
	}
	left, leftErr := p.manager.GetArg(expression, 0)
	right, rightErr := p.manager.GetArg(expression, 1)
	if err = errors.Join(leftErr, rightErr); err != nil {
		return 0, nil, false, err
	}
	leftKind, err := p.manager.GetKind(left)
	if err != nil {
		return 0, nil, false, err
	}
	if leftKind != ast.ASTVar {
		left, right = right, left
		leftKind, err = p.manager.GetKind(left)
		if err != nil || leftKind != ast.ASTVar {
			return 0, nil, false, err
		}
	}
	index, err := p.manager.GetVarIndex(left)
	if err != nil {
		return 0, nil, false, err
	}
	value, matched, err := p.numeral(right)
	return index, value, matched, err
}

func (p *KarrInvariantsPlugin) addInterpreted(target *rel.KarrRelation, expression ast.ASTHandle) error {
	var conjuncts []ast.ASTHandle
	if err := p.flattenAnd(expression, &conjuncts); err != nil {
		return err
	}
	for _, conjunct := range conjuncts {
		if equality, err := p.basicOperation(conjunct, ast.ASTOpEq); err != nil {
			return err
		} else if equality {
			left, leftErr := p.manager.GetArg(conjunct, 0)
			right, rightErr := p.manager.GetArg(conjunct, 1)
			if err = errors.Join(leftErr, rightErr); err != nil {
				return err
			}
			if _, err = p.addComparison(target, left, right, 1, -1, 0, true); err != nil {
				return err
			}
			continue
		}
		op, comparison, err := p.arithmeticOperation(conjunct, 2, 3, 4, 5)
		if err != nil {
			return err
		}
		if comparison {
			left, leftErr := p.manager.GetArg(conjunct, 0)
			right, rightErr := p.manager.GetArg(conjunct, 1)
			if err = errors.Join(leftErr, rightErr); err != nil {
				return err
			}
			leftMultiplier, rightMultiplier := int64(-1), int64(1)
			if op == 3 || op == 5 {
				leftMultiplier, rightMultiplier = 1, -1
			}
			adjustment := int64(0)
			if op == 4 || op == 5 {
				adjustment = -1
			}
			if _, err = p.addComparison(target, left, right, leftMultiplier, rightMultiplier, adjustment, false); err != nil {
				return err
			}
			continue
		}
		negation, err := p.basicOperation(conjunct, ast.ASTOpNot)
		if err != nil {
			return err
		}
		if negation {
			inner, getErr := p.manager.GetArg(conjunct, 0)
			if getErr != nil {
				return getErr
			}
			op, comparison, getErr = p.arithmeticOperation(inner, 2, 3, 4, 5)
			if getErr != nil {
				return getErr
			}
			if comparison {
				left, leftErr := p.manager.GetArg(inner, 0)
				right, rightErr := p.manager.GetArg(inner, 1)
				if err = errors.Join(leftErr, rightErr); err != nil {
					return err
				}
				// not(a < b) is a >= b; not(a <= b) is a >= b+1.
				leftMultiplier, rightMultiplier := int64(1), int64(-1)
				if op == 3 || op == 5 {
					leftMultiplier, rightMultiplier = -1, 1
				}
				adjustment := int64(0)
				if op == 2 || op == 3 {
					adjustment = -1
				}
				if _, err = p.addComparison(target, left, right, leftMultiplier, rightMultiplier, adjustment, false); err != nil {
					return err
				}
			}
			continue
		}
		disjunction, err := p.basicOperation(conjunct, ast.ASTOpOr)
		if err != nil {
			return err
		}
		if disjunction {
			count, countErr := p.manager.GetNumArgs(conjunct)
			if countErr != nil {
				return countErr
			}
			if count == 2 {
				left, leftErr := p.manager.GetArg(conjunct, 0)
				right, rightErr := p.manager.GetArg(conjunct, 1)
				if err = errors.Join(leftErr, rightErr); err != nil {
					return err
				}
				leftVariable, leftValue, leftMatch, leftErr := p.variableEquality(left)
				rightVariable, rightValue, rightMatch, rightErr := p.variableEquality(right)
				if err = errors.Join(leftErr, rightErr); err != nil {
					return err
				}
				if leftMatch && rightMatch && leftVariable == rightVariable && leftVariable < target.Dimension() {
					if leftValue.Cmp(rightValue) > 0 {
						leftValue, rightValue = rightValue, leftValue
					}
					lower := make([]big.Rat, target.Dimension())
					lower[leftVariable].SetInt64(1)
					if err = target.AddConstraint(lower, new(big.Rat).Neg(leftValue), false); err != nil {
						return err
					}
					upper := make([]big.Rat, target.Dimension())
					upper[leftVariable].SetInt64(-1)
					if err = target.AddConstraint(upper, rightValue, false); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (p *KarrInvariantsPlugin) ruleCandidate(rule *base.Rule, relations map[ast.ASTHandle]*rel.KarrRelation) (*rel.KarrRelation, ast.ASTHandle, error) {
	variables, err := ruleVariableCount(p.manager, rule)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	candidate := rel.NewKarrFull(variables, p.proceed)
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	for i := 0; i < uninterpreted; i++ {
		application, negated, getErr := rule.Tail(i)
		if getErr != nil || negated {
			return nil, ast.ASTHandle{}, errors.Join(getErr, ast.ErrASTDomain)
		}
		declaration, getErr := p.manager.GetDecl(application)
		if getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
		relation := relations[declaration]
		if relation == nil {
			arity, arityErr := p.manager.GetArity(declaration)
			if arityErr != nil {
				return nil, ast.ASTHandle{}, arityErr
			}
			relation = rel.NewKarrEmpty(arity, p.proceed)
			relations[declaration] = relation
		}
		if relation.Empty() {
			head, headErr := rule.HeadDeclaration()
			if headErr != nil {
				return nil, ast.ASTHandle{}, headErr
			}
			arity, arityErr := p.manager.GetArity(head)
			return rel.NewKarrEmpty(arity, p.proceed), head, arityErr
		}
		matrix, getErr := relation.Inequalities()
		if getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
		if matrix.Len() != 0 {
			if getErr = p.addPredicateApplication(candidate, application, matrix); getErr != nil {
				return nil, ast.ASTHandle{}, getErr
			}
		}
	}
	for i := uninterpreted; i < total; i++ {
		condition, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
		if getErr = p.addInterpreted(candidate, condition); getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
	}
	head, err := rule.Head()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	declaration, err := p.manager.GetDecl(head)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	matrix, err := candidate.Inequalities()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	extended := rel.NewKarrFull(variables+arity, p.proceed)
	for i := range matrix.A {
		row := make([]big.Rat, variables+arity)
		copy(row, matrix.A[i])
		if err = extended.AddConstraint(row, &matrix.B[i], matrix.Eq[i]); err != nil {
			return nil, ast.ASTHandle{}, err
		}
	}
	for i := uint32(0); i < arity; i++ {
		row := make([]big.Rat, variables+arity)
		row[variables+i].SetInt64(1)
		constant := new(big.Rat)
		argument, getErr := p.manager.GetArg(head, i)
		if getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
		matched, getErr := p.addLinear(argument, big.NewRat(-1, 1), row, constant)
		if getErr != nil {
			return nil, ast.ASTHandle{}, getErr
		}
		if matched {
			if getErr = extended.AddConstraint(row, constant, true); getErr != nil {
				return nil, ast.ASTHandle{}, getErr
			}
		}
	}
	removed := make([]uint32, variables)
	for i := range removed {
		removed[i] = uint32(i)
	}
	result, err := extended.Project(removed)
	return result, declaration, err
}

func (p *KarrInvariantsPlugin) getInvariants(source *base.RuleSet) (map[ast.ASTHandle]rel.KarrMatrix, error) {
	relations, order, err := p.relationMap(source)
	if err != nil {
		return nil, err
	}
	changed := true
	for changed && p.proceed() {
		changed = false
		count, countErr := source.Len()
		if countErr != nil {
			return nil, countErr
		}
		for i := 0; i < count; i++ {
			rule, getErr := source.Rule(i)
			if getErr != nil {
				return nil, getErr
			}
			candidate, declaration, candidateErr := p.ruleCandidate(rule, relations)
			candidateErr = errors.Join(candidateErr, rule.Close())
			if candidateErr != nil {
				return nil, candidateErr
			}
			updated, unionErr := relations[declaration].Union(candidate)
			if unionErr != nil {
				return nil, unionErr
			}
			changed = changed || updated
		}
	}
	if !p.proceed() {
		return nil, nil
	}
	result := make(map[ast.ASTHandle]rel.KarrMatrix, len(order))
	for _, declaration := range order {
		if relations[declaration].Empty() {
			continue
		}
		matrix, matrixErr := relations[declaration].Inequalities()
		if matrixErr != nil {
			return nil, matrixErr
		}
		if matrix.Len() != 0 {
			result[declaration] = matrix
		}
	}
	return result, nil
}

func combineKarrInvariants(destination map[ast.ASTHandle]rel.KarrMatrix, source map[ast.ASTHandle]rel.KarrMatrix) {
	for declaration, matrix := range source {
		prior := destination[declaration]
		prior.A = append(prior.A, matrix.A...)
		prior.B = append(prior.B, matrix.B...)
		prior.Eq = append(prior.Eq, matrix.Eq...)
		destination[declaration] = prior
	}
}

func (p *KarrInvariantsPlugin) projectModelInvariants(loop *LoopCounterPlugin, invariants map[ast.ASTHandle]rel.KarrMatrix) (map[ast.ASTHandle]rel.KarrMatrix, error) {
	result := make(map[ast.ASTHandle]rel.KarrMatrix)
	for original, derived := range loop.FreshPredicates() {
		matrix, found := invariants[derived]
		if !found || matrix.Len() == 0 || len(matrix.A[0]) == 0 {
			continue
		}
		relation := rel.NewKarrFull(uint32(len(matrix.A[0])), p.proceed)
		for i := range matrix.A {
			if err := relation.AddConstraint(matrix.A[i], &matrix.B[i], matrix.Eq[i]); err != nil {
				return nil, err
			}
		}
		projected, err := relation.Project([]uint32{uint32(len(matrix.A[0]) - 1)})
		if err != nil {
			return nil, err
		}
		projectedMatrix, err := projected.Inequalities()
		if err != nil {
			return nil, err
		}
		if projectedMatrix.Len() != 0 {
			result[original] = projectedMatrix
		}
	}
	return result, nil
}

func (p *KarrInvariantsPlugin) matrixFormula(matrix rel.KarrMatrix, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := p.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjuncts := make([]ast.ASTHandle, 0, matrix.Len())
	for i := range matrix.A {
		if len(matrix.A[i]) != len(arguments) {
			return ast.ASTHandle{}, rel.ErrKarrDimension
		}
		terms := make([]ast.ASTHandle, 0, len(arguments)+1)
		for column := range arguments {
			coefficient := &matrix.A[i][column]
			if coefficient.Sign() == 0 {
				continue
			}
			term := arguments[column]
			if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
				numeral, numeralErr := p.manager.ArithmeticNumeralRat(coefficient, true)
				if numeralErr != nil {
					return ast.ASTHandle{}, numeralErr
				}
				term, numeralErr = p.manager.MkArithmeticMul(numeral, term)
				if numeralErr != nil {
					return ast.ASTHandle{}, numeralErr
				}
			}
			terms = append(terms, term)
		}
		if matrix.B[i].Sign() != 0 {
			constant, constantErr := p.manager.ArithmeticNumeralRat(&matrix.B[i], true)
			if constantErr != nil {
				return ast.ASTHandle{}, constantErr
			}
			terms = append(terms, constant)
		}
		lhs := zero
		if len(terms) != 0 {
			lhs = terms[0]
			for j := 1; j < len(terms); j++ {
				lhs, err = p.manager.MkArithmeticAdd(lhs, terms[j])
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		var formula ast.ASTHandle
		if matrix.Eq[i] {
			formula, err = p.manager.MkEq(lhs, zero)
		} else {
			formula, err = p.manager.MkArithmeticGE(lhs, zero)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conjuncts = append(conjuncts, formula)
	}
	if len(conjuncts) == 1 {
		return conjuncts[0], nil
	}
	return p.manager.MkAndArray(conjuncts)
}

func (p *KarrInvariantsPlugin) updateRules(source *base.RuleSet, invariants map[ast.ASTHandle]rel.KarrMatrix) (_ *base.RuleSet, resultErr error) {
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		var pins []ast.ASTRef
		release := func() (releaseErr error) {
			for j := len(pins) - 1; j >= 0; j-- {
				releaseErr = errors.Join(releaseErr, p.manager.DelRef(&pins[j]))
			}
			return releaseErr
		}
		total, sizeErr := rule.TailSize()
		uninterpreted, uninterpretedErr := rule.UninterpretedTailSize()
		getErr = errors.Join(sizeErr, uninterpretedErr)
		tails := make([]ast.ASTHandle, 0, total+uninterpreted)
		negated := make([]bool, 0, total+uninterpreted)
		for j := 0; getErr == nil && j < total; j++ {
			tail, negative, tailErr := rule.Tail(j)
			getErr = tailErr
			tails = append(tails, tail)
			negated = append(negated, negative)
		}
		for j := 0; getErr == nil && j < uninterpreted; j++ {
			application := tails[j]
			declaration, declarationErr := p.manager.GetDecl(application)
			matrix, found := invariants[declaration]
			if declarationErr != nil {
				getErr = declarationErr
				break
			}
			if !found || matrix.Len() == 0 {
				continue
			}
			count, argumentErr := p.manager.GetNumArgs(application)
			arguments := make([]ast.ASTHandle, count)
			for k := uint32(0); argumentErr == nil && k < count; k++ {
				arguments[k], argumentErr = p.manager.GetArg(application, k)
			}
			var formula ast.ASTHandle
			if argumentErr == nil {
				formula, argumentErr = p.matrixFormula(matrix, arguments)
			}
			if argumentErr == nil {
				var reference ast.ASTRef
				reference, argumentErr = p.manager.NewRef(formula)
				if argumentErr == nil {
					pins = append(pins, reference)
					tails = append(tails, formula)
					negated = append(negated, false)
				}
			}
			getErr = argumentErr
		}
		head, headErr := rule.Head()
		name, nameErr := rule.Name()
		getErr = errors.Join(getErr, headErr, nameErr)
		var replacement *base.Rule
		if getErr == nil {
			replacement, getErr = p.rules.NewRule(head, tails, negated, name)
		}
		if getErr == nil {
			getErr = result.Add(replacement)
		}
		if replacement != nil {
			getErr = errors.Join(getErr, replacement.Close())
		}
		getErr = errors.Join(getErr, release(), rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

func (p *KarrInvariantsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	p.cancelled = false
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		negated, negationErr := rule.HasNegation()
		getErr = errors.Join(negationErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if negated {
			return nil, nil
		}
	}
	loop := NewLoopCounterPlugin(p.manager, p.rules, 0)
	defer func() { resultErr = errors.Join(resultErr, loop.Close()) }()
	loopRules, err := loop.Transform(source)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, loopRules.Close()) }()
	forward, err := p.getInvariants(loopRules)
	if err != nil || forward == nil {
		return nil, err
	}
	backwards := NewBackwardsPlugin(p.manager, p.rules, 0)
	defer func() { resultErr = errors.Join(resultErr, backwards.Close()) }()
	reversed, err := backwards.Transform(loopRules)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, reversed.Close()) }()
	reverseInvariants, err := p.getInvariants(reversed)
	if err != nil || reverseInvariants == nil {
		return nil, err
	}
	combineKarrInvariants(forward, reverseInvariants)
	p.invariants = forward
	p.model, err = p.projectModelInvariants(loop, forward)
	if err != nil {
		return nil, err
	}
	annotated, err := p.updateRules(loopRules, forward)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, annotated.Close()) }()
	result, err := loop.Revert(annotated)
	if err != nil {
		return nil, err
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

func (p *KarrInvariantsPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.invariants = nil
	p.model = nil
	return nil
}
