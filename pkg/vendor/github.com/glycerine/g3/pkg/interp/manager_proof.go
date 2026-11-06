package interp

// Source checkpoint: src/interp/iz3mgr.cpp and src/interp/iz3mgr.h proof readers.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// LemmaTheory identifies the theory recorded on a theory-lemma proof.
type LemmaTheory uint8

const (
	ArithTheory LemmaTheory = iota
	ArrayTheory
	UnknownTheory
)

// LemmaKind identifies the retained arithmetic theory-lemma variants.
type LemmaKind uint8

const (
	FarkasKind LemmaKind = iota
	Leq2EqKind
	Eq2LeqKind
	GCDTestKind
	AssignBoundsKind
	EqPropagateKind
	GomoryCutKind
	ArithMysteryKind
	UnknownKind
)

func (m *Manager) ProofRule(proof ast.ASTHandle) (int32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	family, err := m.ast.GetFamilyID(proof)
	if err != nil {
		return 0, err
	}
	if family != m.ast.BasicFamilyID() {
		return 0, ast.ErrASTDomain
	}
	return m.ast.GetDeclKind(proof)
}

func (m *Manager) NumPremises(proof ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	n, err := m.ast.GetNumArgs(proof)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ast.ErrASTDomain
	}
	return n - 1, nil
}

func (m *Manager) Premise(proof ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	n, err := m.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= n {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.ast.GetArg(proof, index)
}

func (m *Manager) Conclusion(proof ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := m.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetArg(proof, n)
}

func (m *Manager) proofParameters(proof ast.ASTHandle) (parameters []ast.ASTParameter, result error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	decl, err := m.ast.GetDecl(proof)
	if err != nil {
		return nil, err
	}
	parameters, err = m.ast.GetDeclParametersCopy(decl)
	if err != nil {
		return nil, err
	}
	return parameters, nil
}

func (m *Manager) releaseParameters(parameters []ast.ASTParameter) (result error) {
	for i := range parameters {
		result = errors.Join(result, m.ast.DelParameter(&parameters[i]))
	}
	return result
}

func (m *Manager) parameterSymbolText(parameter ast.ASTParameter) (string, error) {
	symbol, err := m.ast.ParameterSymbol(parameter)
	if err != nil {
		return "", err
	}
	text, present, err := m.ast.Symbols.BareString(symbol)
	if err != nil {
		return "", err
	}
	if present {
		return text, nil
	}
	return m.ast.Symbols.String(symbol)
}

func (m *Manager) TheoryLemmaTheory(proof ast.ASTHandle) (result LemmaTheory, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return UnknownTheory, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) == 0 {
		return UnknownTheory, nil
	}
	name, err := m.parameterSymbolText(parameters[0])
	if err != nil {
		return UnknownTheory, nil
	}
	switch name {
	case "arith":
		return ArithTheory, nil
	case "array":
		return ArrayTheory, nil
	default:
		return UnknownTheory, nil
	}
}

func (m *Manager) TheoryLemmaKind(proof ast.ASTHandle) (result LemmaKind, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return UnknownKind, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) < 2 {
		return ArithMysteryKind, nil
	}
	name, err := m.parameterSymbolText(parameters[1])
	if err != nil {
		return UnknownKind, nil
	}
	switch name {
	case "farkas":
		return FarkasKind, nil
	case "triangle-eq":
		conclusion, e := m.Conclusion(proof)
		if e != nil {
			return UnknownKind, e
		}
		first, e := m.Arg(conclusion, 0)
		if e != nil {
			return UnknownKind, e
		}
		if op, e := m.Operator(first); e != nil {
			return UnknownKind, e
		} else if op == OperatorNot {
			return Eq2LeqKind, nil
		}
		return Leq2EqKind, nil
	case "gcd-test":
		return GCDTestKind, nil
	case "assign-bounds":
		return AssignBoundsKind, nil
	case "eq-propagate":
		return EqPropagateKind, nil
	case "gomory-cut":
		return GomoryCutKind, nil
	default:
		return UnknownKind, nil
	}
}

func (m *Manager) rationalParameter(parameter ast.ASTParameter) (result *big.Rat, err error) {
	service := m.ast.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, service.Del(&value)) }()
	matched, err := m.ast.ParameterRationalCopy(parameter, &value)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, NewError("Bad Farkas coefficient")
	}
	text, err := service.Display(&value)
	if err != nil {
		return nil, err
	}
	result, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, NewError("Bad Farkas coefficient")
	}
	return result, nil
}

func normalizeCoefficientDenominators(values []*big.Rat) {
	lcm := big.NewInt(1)
	for _, value := range values {
		denominator := value.Denom()
		gcd := new(big.Int).GCD(nil, nil, lcm, denominator)
		lcm.Mul(lcm, denominator)
		lcm.Quo(lcm, gcd)
	}
	factor := new(big.Rat).SetInt(lcm)
	for _, value := range values {
		value.Mul(value, factor)
	}
}

func (m *Manager) theoryLemmaCoefficients(proof ast.ASTHandle, absolute bool) (result []*big.Rat, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) < 2 {
		return nil, NewError("Bad Farkas coefficient")
	}
	result = make([]*big.Rat, len(parameters)-2)
	for i := range result {
		result[i], err = m.rationalParameter(parameters[i+2])
		if err != nil {
			return nil, err
		}
		if absolute && result[i].Sign() < 0 {
			result[i].Neg(result[i])
		}
	}
	normalizeCoefficientDenominators(result)
	return result, nil
}

func (m *Manager) FarkasCoefficients(proof ast.ASTHandle) ([]*big.Rat, error) {
	return m.theoryLemmaCoefficients(proof, true)
}

func (m *Manager) BrokenGCDTestCoefficients(proof ast.ASTHandle) ([]*big.Rat, error) {
	return m.theoryLemmaCoefficients(proof, false)
}

func (m *Manager) GomoryCutCoefficients(proof ast.ASTHandle) ([]*big.Rat, error) {
	return m.theoryLemmaCoefficients(proof, true)
}

func (m *Manager) inequalityHasNegativeOrientation(inequality ast.ASTHandle) (bool, error) {
	isNot, body, err := m.ast.IsNot(inequality)
	if err != nil {
		return false, err
	}
	target := inequality
	if isNot {
		target = body
	}
	op, err := m.Operator(target)
	if err != nil {
		return false, err
	}
	if isNot {
		return op == OperatorLE || op == OperatorLT, nil
	}
	return op == OperatorGE || op == OperatorGT, nil
}

func (m *Manager) assignBoundsCoefficients(proof ast.ASTHandle, rule bool) (result []*big.Rat, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) < 2 {
		return nil, NewError("Bad Farkas coefficient")
	}
	conclusion, err := m.Conclusion(proof)
	if err != nil {
		return nil, err
	}
	consequence, err := m.Arg(conclusion, 0)
	if err != nil {
		return nil, err
	}
	consequenceNegative, err := m.inequalityHasNegativeOrientation(consequence)
	if err != nil {
		return nil, err
	}
	result = make([]*big.Rat, len(parameters)-1)
	result[0] = new(big.Rat).SetInt64(1)
	for i := 2; i < len(parameters); i++ {
		value, e := m.rationalParameter(parameters[i])
		if e != nil {
			return nil, e
		}
		var inequality ast.ASTHandle
		if rule {
			premise, e := m.Premise(proof, uint32(i-2))
			if e != nil {
				return nil, e
			}
			inequality, e = m.Conclusion(premise)
			if e != nil {
				return nil, e
			}
		} else {
			inequality, e = m.Arg(conclusion, uint32(i-1))
			if e != nil {
				return nil, e
			}
		}
		negative, e := m.inequalityHasNegativeOrientation(inequality)
		if e != nil {
			return nil, e
		}
		if negative != consequenceNegative {
			value.Neg(value)
		}
		if value.Sign() < 0 {
			value.Neg(value)
		}
		result[i-1] = value
	}
	normalizeCoefficientDenominators(result)
	return result, nil
}

func (m *Manager) AssignBoundsCoefficients(proof ast.ASTHandle) ([]*big.Rat, error) {
	return m.assignBoundsCoefficients(proof, false)
}

func (m *Manager) AssignBoundsRuleCoefficients(proof ast.ASTHandle) ([]*big.Rat, error) {
	return m.assignBoundsCoefficients(proof, true)
}

func (m *Manager) coefficientTerms(values []*big.Rat) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		term, err := m.MakeInt(value)
		if err != nil {
			return nil, err
		}
		result[i] = term
	}
	return result, nil
}

func (m *Manager) FarkasCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.FarkasCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) AssignBoundsCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.AssignBoundsCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) AssignBoundsRuleCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.AssignBoundsRuleCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) GomoryCutCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.GomoryCutCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) IsFarkasCoefficientNegative(proof ast.ASTHandle, index uint32) (result bool, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	parameterIndex := int(index) + 2
	if parameterIndex >= len(parameters) {
		return false, ast.ErrASTDomain
	}
	value, err := m.rationalParameter(parameters[parameterIndex])
	if err != nil {
		return false, err
	}
	return value.Sign() < 0, nil
}
