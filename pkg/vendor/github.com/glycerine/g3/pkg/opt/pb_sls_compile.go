package opt

// Source: src/opt/pb_sls.cpp:522-706.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (s *PBSLS) evaluateBoolean(expression ast.ASTHandle) (value bool, result error) {
	if s.originalModel == nil {
		return false, ErrPBSLS
	}
	ref, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&ref)) }()
	if err = s.originalModel.Evaluate(expression, ref, false, s.config.Model); err != nil {
		return false, err
	}
	handle, err := s.manager.RefGet(ref)
	return s.manager.IsTrue(handle), err
}

func (s *PBSLS) SetModel(source *model.Model) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if source == nil || source.Core().Manager() != s.manager {
		return ErrPBSLS
	}
	copy, err := source.Copy()
	if err != nil {
		return err
	}
	if s.originalModel != nil {
		if err = s.originalModel.Close(); err != nil {
			return errors.Join(err, copy.Close())
		}
	}
	s.originalModel = copy
	for i, expression := range s.variableExprs {
		value, err := s.evaluateBoolean(expression)
		if err != nil {
			return err
		}
		s.assignment[i] = value
	}
	return nil
}

func (s *PBSLS) newVariable(expression ast.ASTHandle) (pbSLSLiteral, error) {
	if variable, found := s.variables[expression]; found {
		return pbSLSLiteral{variable: variable}, nil
	}
	value, err := s.evaluateBoolean(expression)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	if err = s.manager.IncRef(expression); err != nil {
		return pbSLSNullLiteral, err
	}
	variable := uint32(len(s.variableExprs))
	s.variables[expression] = variable
	s.variableExprs = append(s.variableExprs, expression)
	s.assignment = append(s.assignment, value)
	s.hardOccurrences = append(s.hardOccurrences, nil)
	s.softOccurrences = append(s.softOccurrences, nil)
	return pbSLSLiteral{variable: variable}, nil
}

func (s *PBSLS) basicOperation(expression ast.ASTHandle, operation int32) (bool, error) {
	family, err := s.manager.GetFamilyID(expression)
	if err != nil || family != s.manager.BasicFamilyID() {
		return false, err
	}
	kind, err := s.manager.GetDeclKind(expression)
	return kind == operation, err
}

func pbSLSUnitClause(literals ...pbSLSLiteral) pbSLSClause {
	clause := pbSLSClause{literals: append([]pbSLSLiteral(nil), literals...), equality: false}
	clause.bound.SetInt64(1)
	clause.weights = make([]big.Int, len(literals))
	for i := range clause.weights {
		clause.weights[i].SetInt64(1)
	}
	return clause
}

func (s *PBSLS) appendHardDefinition(clause pbSLSClause, origin ast.ASTHandle) error {
	if err := s.manager.IncRef(origin); err != nil {
		return err
	}
	s.hard = append(s.hard, clause)
	s.hardOrigins = append(s.hardOrigins, origin)
	return nil
}

func (s *PBSLS) makeLiteral(expression ast.ASTHandle) (pbSLSLiteral, error) {
	if matched, child, err := s.manager.IsNot(expression); err != nil {
		return pbSLSNullLiteral, err
	} else if matched {
		literal, err := s.makeLiteral(child)
		return literal.negate(), err
	}
	if s.manager.IsTrue(expression) {
		return pbSLSTrueLiteral, nil
	}
	if s.manager.IsFalse(expression) {
		return pbSLSFalseLiteral, nil
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return pbSLSNullLiteral, err
	}
	n, err := s.manager.GetNumArgs(expression)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	family, err := s.manager.GetFamilyID(expression)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	if n == 0 && family == ast.NullFamilyID {
		return s.newVariable(expression)
	}
	isAnd, err := s.basicOperation(expression, ast.ASTOpAnd)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	isOr, err := s.basicOperation(expression, ast.ASTOpOr)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	isEq, err := s.basicOperation(expression, ast.ASTOpEq)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	isIff, err := s.basicOperation(expression, ast.ASTOpIff)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	isBooleanEq := false
	if isEq && n == 2 {
		lhs, e := s.manager.GetArg(expression, 0)
		if e != nil {
			return pbSLSNullLiteral, e
		}
		lhsSort, e := s.manager.GetSort(lhs)
		if e != nil {
			return pbSLSNullLiteral, e
		}
		boolSort, e := s.manager.MkBoolSort()
		if e != nil {
			return pbSLSNullLiteral, e
		}
		isBooleanEq = lhsSort == boolSort
	}
	if !isAnd && !isOr && !(isBooleanEq || isIff) {
		return s.newVariable(expression)
	}
	literals := make([]pbSLSLiteral, n)
	for i := uint32(0); i < n; i++ {
		arg, err := s.manager.GetArg(expression, i)
		if err != nil {
			return pbSLSNullLiteral, err
		}
		literals[i], err = s.makeLiteral(arg)
		if err != nil {
			return pbSLSNullLiteral, err
		}
	}
	result, err := s.newVariable(expression)
	if err != nil {
		return pbSLSNullLiteral, err
	}
	if isAnd {
		for _, literal := range literals {
			if err = s.appendHardDefinition(pbSLSUnitClause(result.negate(), literal), expression); err != nil {
				return pbSLSNullLiteral, err
			}
		}
		final := make([]pbSLSLiteral, 0, len(literals)+1)
		for _, literal := range literals {
			final = append(final, literal.negate())
		}
		final = append(final, result)
		if err = s.appendHardDefinition(pbSLSUnitClause(final...), expression); err != nil {
			return pbSLSNullLiteral, err
		}
		return result, nil
	}
	if isOr {
		for _, literal := range literals {
			if err = s.appendHardDefinition(pbSLSUnitClause(result, literal.negate()), expression); err != nil {
				return pbSLSNullLiteral, err
			}
		}
		final := append(append([]pbSLSLiteral(nil), literals...), result.negate())
		if err = s.appendHardDefinition(pbSLSUnitClause(final...), expression); err != nil {
			return pbSLSNullLiteral, err
		}
		return result, nil
	}
	// Boolean equality/iff: four clauses defining result <-> (a == b).
	a, b := literals[0], literals[1]
	clauses := []pbSLSClause{
		pbSLSUnitClause(result.negate(), a.negate(), b),
		pbSLSUnitClause(result.negate(), a, b.negate()),
		pbSLSUnitClause(result, a, b),
		pbSLSUnitClause(result, a.negate(), b.negate()),
	}
	for _, clause := range clauses {
		if err = s.appendHardDefinition(clause, expression); err != nil {
			return pbSLSNullLiteral, err
		}
	}
	return result, nil
}

func (s *PBSLS) rationalInteger(value *unumeral.OwnedRational) (*big.Int, error) {
	text, err := s.manager.NumeralService().String(value)
	if err != nil {
		return nil, err
	}
	rational, ok := new(big.Rat).SetString(text)
	if !ok || rational.Denom().Cmp(big.NewInt(1)) != 0 {
		return nil, ErrPBSLS
	}
	return new(big.Int).Set(rational.Num()), nil
}

type pbSLSRelation uint8

const (
	pbSLSGreaterEqual pbSLSRelation = iota
	pbSLSEqual
	pbSLSLessEqual
)

func (s *PBSLS) relation(expression ast.ASTHandle) (pbSLSRelation, bool, error) {
	checks := []struct {
		relation pbSLSRelation
		check    func(ast.ASTHandle) (bool, error)
	}{
		{pbSLSGreaterEqual, s.manager.IsPseudoBooleanGE},
		{pbSLSGreaterEqual, s.manager.IsPseudoBooleanAtLeast},
		{pbSLSEqual, s.manager.IsPseudoBooleanEQ},
		{pbSLSLessEqual, s.manager.IsPseudoBooleanLE},
		{pbSLSLessEqual, s.manager.IsPseudoBooleanAtMost},
	}
	for _, candidate := range checks {
		matched, err := candidate.check(expression)
		if err != nil {
			return 0, false, err
		}
		if matched {
			return candidate.relation, true, nil
		}
	}
	return 0, false, nil
}

func (s *PBSLS) literalValue(literal pbSLSLiteral) bool {
	return literal.negated != s.assignment[literal.variable]
}

func (s *PBSLS) compileRelation(expression ast.ASTHandle, relation pbSLSRelation) (pbSLSClause, bool, error) {
	service := s.manager.NumeralService()
	boundValue, err := s.manager.PseudoBooleanBound(expression)
	if err != nil {
		return pbSLSClause{}, false, err
	}
	bound, err := s.rationalInteger(&boundValue)
	cleanup := service.Del(&boundValue)
	if err != nil || cleanup != nil {
		return pbSLSClause{}, false, errors.Join(err, cleanup)
	}
	n, err := s.manager.GetNumArgs(expression)
	if err != nil {
		return pbSLSClause{}, false, err
	}
	clause := pbSLSClause{equality: relation == pbSLSEqual}
	clause.bound.Set(bound)
	var sum, constantShift big.Int
	for i := uint32(0); i < n; i++ {
		coefficientValue, err := s.manager.PseudoBooleanCoefficient(expression, i)
		if err != nil {
			return pbSLSClause{}, false, err
		}
		coefficient, err := s.rationalInteger(&coefficientValue)
		cleanup := service.Del(&coefficientValue)
		if err != nil || cleanup != nil {
			return pbSLSClause{}, false, errors.Join(err, cleanup)
		}
		arg, err := s.manager.GetArg(expression, i)
		if err != nil {
			return pbSLSClause{}, false, err
		}
		literal, err := s.makeLiteral(arg)
		if err != nil {
			return pbSLSClause{}, false, err
		}
		if relation == pbSLSLessEqual {
			literal = literal.negate()
			sum.Add(&sum, coefficient)
		}
		if literal == pbSLSFalseLiteral {
			continue
		}
		if literal == pbSLSTrueLiteral {
			if relation == pbSLSLessEqual {
				constantShift.Add(&constantShift, coefficient)
			} else {
				clause.bound.Sub(&clause.bound, coefficient)
			}
			continue
		}
		clause.literals = append(clause.literals, literal)
		clause.weights = append(clause.weights, *new(big.Int).Set(coefficient))
		if s.literalValue(literal) {
			clause.value.Add(&clause.value, coefficient)
		}
	}
	if relation == pbSLSLessEqual {
		clause.bound.Sub(&sum, bound)
		clause.bound.Sub(&clause.bound, &constantShift)
		clause.equality = false
	}
	return clause, true, nil
}

func (s *PBSLS) compileExpression(expression ast.ASTHandle) (pbSLSClause, bool, error) {
	if matched, child, err := s.manager.IsNot(expression); err != nil {
		return pbSLSClause{}, false, err
	} else if matched {
		if relation, yes, err := s.relation(child); err != nil {
			return pbSLSClause{}, false, err
		} else if yes && relation == pbSLSGreaterEqual {
			// Integer ~(sum >= k) is sum <= k-1.
			clause, compiled, err := s.compileRelation(child, pbSLSLessEqual)
			if err == nil && compiled {
				clause.bound.Add(&clause.bound, big.NewInt(1))
			}
			return clause, compiled, err
		}
	}
	if relation, yes, err := s.relation(expression); err != nil {
		return pbSLSClause{}, false, err
	} else if yes {
		return s.compileRelation(expression, relation)
	}
	if isOr, err := s.basicOperation(expression, ast.ASTOpOr); err != nil {
		return pbSLSClause{}, false, err
	} else if isOr {
		n, err := s.manager.GetNumArgs(expression)
		if err != nil {
			return pbSLSClause{}, false, err
		}
		clause := pbSLSUnitClause()
		for i := uint32(0); i < n; i++ {
			arg, err := s.manager.GetArg(expression, i)
			if err != nil {
				return pbSLSClause{}, false, err
			}
			literal, err := s.makeLiteral(arg)
			if err != nil {
				return pbSLSClause{}, false, err
			}
			if literal == pbSLSTrueLiteral {
				return pbSLSClause{}, false, nil
			}
			if literal == pbSLSFalseLiteral {
				continue
			}
			clause.literals = append(clause.literals, literal)
			clause.weights = append(clause.weights, *big.NewInt(1))
			if s.literalValue(literal) {
				clause.value.Add(&clause.value, big.NewInt(1))
			}
		}
		return clause, true, nil
	}
	if s.manager.IsTrue(expression) {
		return pbSLSClause{}, false, nil
	}
	literal, err := s.makeLiteral(expression)
	if err != nil || literal == pbSLSNullLiteral {
		return pbSLSClause{}, false, err
	}
	clause := pbSLSUnitClause(literal)
	if s.literalValue(literal) {
		clause.value.SetInt64(1)
	}
	return clause, true, nil
}

func (s *PBSLS) compile(expression ast.ASTHandle) (clause pbSLSClause, compiled bool, result error) {
	// Preserve pseudo-Boolean structure for the concrete SLS compiler.  The
	// general theory rewriter may otherwise abstract a PB atom to a fresh
	// Boolean constant before this retained consumer sees its coefficients.
	if _, matched, err := s.relation(expression); err != nil {
		return clause, false, err
	} else if matched {
		return s.compileExpression(expression)
	}
	if negated, child, err := s.manager.IsNot(expression); err != nil {
		return clause, false, err
	} else if negated {
		if _, matched, err := s.relation(child); err != nil {
			return clause, false, err
		} else if matched {
			return s.compileExpression(expression)
		}
	}
	ref, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return clause, false, err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&ref)) }()
	if err = s.rewrite.Rewrite(expression, ref); err != nil {
		return clause, false, err
	}
	rewritten, err := s.manager.RefGet(ref)
	if err != nil {
		return clause, false, err
	}
	return s.compileExpression(rewritten)
}

func (s *PBSLS) AddHard(expression ast.ASTHandle) error {
	if err := s.open(); err != nil || s.originalModel == nil {
		if err != nil {
			return err
		}
		return ErrPBSLS
	}
	clause, compiled, err := s.compile(expression)
	if err != nil || !compiled {
		return err
	}
	return s.appendHardDefinition(clause, expression)
}

func (s *PBSLS) AddSoft(expression ast.ASTHandle, weight *big.Rat) error {
	if err := s.open(); err != nil || s.originalModel == nil || weight == nil {
		if err != nil {
			return err
		}
		return ErrPBSLS
	}
	clause, compiled, err := s.compile(expression)
	if err != nil || !compiled {
		return err
	}
	s.soft = append(s.soft, clause)
	s.softWeights = append(s.softWeights, *new(big.Rat).Set(weight))
	return nil
}
