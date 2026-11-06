package arith

// Source: src/tactic/arith/fm_tactic.cpp:474-510.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *FMInputParser) literal(term ast.ASTHandle) (ast.ASTHandle, bool, bool, error) {
	if yes, err := p.uninterpretedConstant(term); err != nil || yes {
		return term, false, yes, err
	}
	not, child, err := p.manager.IsNot(term)
	if err != nil || !not {
		return ast.ASTHandle{}, false, false, err
	}
	yes, err := p.uninterpretedConstant(child)
	return child, true, yes, err
}

func (p *FMInputParser) inequality(term ast.ASTHandle, intern bool) (xs []uint32, coefficients []big.Rat, constant big.Rat, strict, matched bool, err error) {
	not, child, err := p.manager.IsNot(term)
	if err != nil {
		return nil, nil, big.Rat{}, false, false, err
	}
	if not {
		term = child
	}
	if !intern {
		if err = p.store.diagnostics.recognition(term); err != nil {
			return nil, nil, big.Rat{}, false, false, err
		}
	}
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return nil, nil, big.Rat{}, false, false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != p.Family {
		return nil, nil, big.Rat{}, false, false, err
	}
	op, err := p.manager.GetDeclKind(term)
	if err != nil || (op != 2 && op != 3) {
		return nil, nil, big.Rat{}, false, false, err
	}
	lhs, err := p.manager.GetArg(term, 0)
	if err != nil {
		return nil, nil, big.Rat{}, false, false, err
	}
	rhs, err := p.manager.GetArg(term, 1)
	if err != nil {
		return nil, nil, big.Rat{}, false, false, err
	}
	text, _, numeral, err := p.manager.ArithmeticNumeralText(rhs)
	if err != nil || !numeral {
		return nil, nil, big.Rat{}, false, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, nil, big.Rat{}, false, false, ast.ErrASTDomain
	}
	xs, coefficients, useful, err := p.polynomial(lhs, intern)
	if err != nil || !useful {
		return nil, nil, big.Rat{}, false, false, err
	}
	negative := not
	strict = not
	if op == 3 {
		negative = !negative
	}
	if negative {
		value.Neg(value)
		for i := range coefficients {
			coefficients[i].Neg(&coefficients[i])
		}
	}
	return xs, coefficients, *value, strict, true, nil
}
