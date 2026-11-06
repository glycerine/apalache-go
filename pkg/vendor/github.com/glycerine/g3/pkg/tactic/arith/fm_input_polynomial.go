package arith

// Source: src/tactic/arith/fm_tactic.cpp:438-472,1029-1061.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *FMInputParser) arithmetic(term ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != p.Family {
		return false, err
	}
	op, err := p.manager.GetDeclKind(term)
	return op == operation, err
}

func (p *FMInputParser) monomial(term ast.ASTHandle) (ast.ASTHandle, big.Rat, bool, error) {
	base, yes, err := p.variableTerm(term)
	if err != nil || yes {
		return base, *big.NewRat(1, 1), yes, err
	}
	mul, err := p.arithmetic(term, 9)
	if err != nil || !mul {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	n, err := p.manager.GetNumArgs(term)
	if err != nil || n != 2 {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	coefficient, err := p.manager.GetArg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	text, _, numeral, err := p.manager.ArithmeticNumeralText(coefficient)
	if err != nil || !numeral {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	variable, err := p.manager.GetArg(term, 1)
	if err != nil {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	base, yes, err = p.variableTerm(variable)
	if err != nil || !yes {
		return ast.ASTHandle{}, big.Rat{}, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return ast.ASTHandle{}, big.Rat{}, false, ast.ErrASTDomain
	}
	return base, *value, true, nil
}

func (p *FMInputParser) polynomial(term ast.ASTHandle, intern bool) ([]uint32, []big.Rat, bool, error) {
	terms := []ast.ASTHandle{term}
	if add, err := p.arithmetic(term, 6); err != nil {
		return nil, nil, false, err
	} else if add {
		n, err := p.manager.GetNumArgs(term)
		if err != nil {
			return nil, nil, false, err
		}
		terms = terms[:0]
		for i := uint32(0); i < n; i++ {
			h, err := p.manager.GetArg(term, i)
			if err != nil {
				return nil, nil, false, err
			}
			terms = append(terms, h)
		}
	}
	seen := map[ast.ASTHandle]bool{}
	xs, coefficients := make([]uint32, 0, len(terms)), make([]big.Rat, 0, len(terms))
	allForbidden := true
	for _, monomial := range terms {
		base, coefficient, yes, err := p.monomial(monomial)
		if err != nil || !yes || seen[base] {
			return nil, nil, false, err
		}
		seen[base] = true
		integer, err := p.integerVariable(base)
		if err != nil {
			return nil, nil, false, err
		}
		declaration, err := p.manager.GetDecl(base)
		if err != nil {
			return nil, nil, false, err
		}
		if !p.forbidden[declaration] && (!p.RealOnly || !integer) {
			allForbidden = false
		}
		if intern {
			x, err := p.internVariable(base)
			if err != nil {
				return nil, nil, false, err
			}
			xs, coefficients = append(xs, x), append(coefficients, coefficient)
		}
	}
	return xs, coefficients, !allForbidden, nil
}
