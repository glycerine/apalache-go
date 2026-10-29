package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::is_linear overloads.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type legacyArithmeticProjection struct {
	syntax *ArithmeticProjection
	theory *rewriter.TheoryRewriter
}

type legacyLinearLiteral struct {
	manager     *ast.ASTManager
	term        ast.ASTRef
	coefficient *big.Rat
	divisor     *big.Rat
	strict      bool
	equality    bool
	disequality bool
}

func newLegacyArithmeticProjection(manager *ast.ASTManager, config Config) (*legacyArithmeticProjection, error) {
	syntax, err := NewArithmeticProjection(manager, config)
	if err != nil {
		return nil, err
	}
	theory, err := rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err != nil {
		return nil, errors.Join(err, syntax.Close())
	}
	return &legacyArithmeticProjection{syntax: syntax, theory: theory}, nil
}

func (p *legacyArithmeticProjection) Close() error {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	var err error
	if p.theory != nil {
		err = p.theory.Close()
	}
	if err == nil && p.theory != nil {
		p.theory = nil
	}
	if err == nil {
		err = p.syntax.Close()
	}
	if err == nil {
		p.syntax = nil
	}
	return err
}

func (l *legacyLinearLiteral) Close() error {
	if l == nil || l.manager == nil {
		return ast.ErrASTHandle
	}
	err := l.manager.DelRef(&l.term)
	if err == nil {
		l.manager = nil
	}
	return err
}

func (p *legacyArithmeticProjection) contains(root, variable ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]bool)
	work := []ast.ASTHandle{root}
	for len(work) != 0 {
		index := len(work) - 1
		current := work[index]
		work = work[:index]
		if current == variable {
			return true, nil
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return false, err
		}
		for argument := uint32(0); argument < count; argument++ {
			handle, err := p.syntax.manager.GetArg(current, argument)
			if err != nil {
				return false, err
			}
			work = append(work, handle)
		}
	}
	return false, nil
}

func (p *legacyArithmeticProjection) scaledTerm(multiplier *big.Rat, term ast.ASTHandle, integer bool) (ast.ASTHandle, error) {
	if multiplier.Cmp(big.NewRat(1, 1)) == 0 {
		return term, nil
	}
	numeral, err := p.syntax.manager.ArithmeticNumeralRat(multiplier, integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.syntax.manager.MkArithmeticMul(numeral, term)
}

func (p *legacyArithmeticProjection) linearTerm(multiplier *big.Rat, root, variable ast.ASTHandle, coefficient *big.Rat, terms *[]ast.ASTHandle, integer bool) (bool, error) {
	if root == variable {
		coefficient.Add(coefficient, multiplier)
		return true, nil
	}
	args, matched, err := p.syntax.operation(root, p.syntax.family, 9)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 2 {
		for index, argument := range args {
			value, numeral, err := p.syntax.numeral(argument)
			if err != nil {
				return false, err
			}
			if numeral {
				return p.linearTerm(new(big.Rat).Mul(multiplier, value), args[1-index], variable, coefficient, terms, integer)
			}
		}
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 6)
	if err != nil {
		return false, err
	}
	if matched {
		for _, argument := range args {
			ok, err := p.linearTerm(multiplier, argument, variable, coefficient, terms, integer)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 7)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 2 {
		ok, err := p.linearTerm(multiplier, args[0], variable, coefficient, terms, integer)
		if err != nil || !ok {
			return ok, err
		}
		return p.linearTerm(new(big.Rat).Neg(multiplier), args[1], variable, coefficient, terms, integer)
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 8)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 1 {
		return p.linearTerm(new(big.Rat).Neg(multiplier), args[0], variable, coefficient, terms, integer)
	}
	value, numeral, err := p.syntax.numeral(root)
	if err != nil {
		return false, err
	}
	if numeral {
		term, err := p.syntax.manager.ArithmeticNumeralRat(new(big.Rat).Mul(multiplier, value), integer)
		if err != nil {
			return false, err
		}
		*terms = append(*terms, term)
		return true, nil
	}
	contains, err := p.contains(root, variable)
	if err != nil || contains {
		return false, err
	}
	term, err := p.scaledTerm(multiplier, root, integer)
	if err != nil {
		return false, err
	}
	*terms = append(*terms, term)
	return true, nil
}

func (p *legacyArithmeticProjection) linearLiteral(variable, literal ast.ASTHandle) (_ *legacyLinearLiteral, resultErr error) {
	if p == nil || p.syntax == nil {
		return nil, ast.ErrASTHandle
	}
	integer, arithmetic, err := p.syntax.arithmetic(variable)
	if err != nil || !arithmetic {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	negative, inner, err := p.syntax.manager.IsNot(literal)
	if err != nil {
		return nil, err
	}
	if negative {
		literal = inner
	}
	multiplier := big.NewRat(1, 1)
	if negative {
		multiplier.Neg(multiplier)
	}
	result := &legacyLinearLiteral{manager: p.syntax.manager, coefficient: new(big.Rat), divisor: new(big.Rat)}
	var left, right ast.ASTHandle
	matched := false
	for _, operation := range []int32{2, 3, 4, 5} {
		args, yes, err := p.syntax.operation(literal, p.syntax.family, operation)
		if err != nil {
			return nil, err
		}
		if !yes || len(args) != 2 {
			continue
		}
		left, right = args[0], args[1]
		if operation == 3 || operation == 5 {
			left, right = right, left
		}
		result.strict = (operation == 2 || operation == 3) == negative
		matched = true
		break
	}
	if !matched {
		args, equality, err := p.syntax.operation(literal, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return nil, err
		}
		if equality && len(args) == 2 {
			for index := 0; index < 2 && !matched; index++ {
				mod, modular, err := p.syntax.operation(args[index], p.syntax.family, 13)
				if err != nil {
					return nil, err
				}
				zero, numeral, err := p.syntax.numeral(args[1-index])
				if err != nil {
					return nil, err
				}
				if !modular || len(mod) != 2 || !numeral || zero.Sign() != 0 {
					continue
				}
				divisor, numeral, err := p.syntax.numeral(mod[1])
				if err != nil {
					return nil, err
				}
				if !numeral || divisor.Sign() == 0 {
					return nil, nil
				}
				left, right = mod[0], args[1-index]
				result.divisor.Set(divisor)
				matched = true
			}
		}
		if equality && len(args) == 2 && !matched {
			_, arithmetic, err := p.syntax.arithmetic(args[0])
			if err != nil {
				return nil, err
			}
			if arithmetic {
				left, right = args[0], args[1]
				result.equality = !negative
				result.disequality = negative
				matched = true
			}
		}
	}
	if !matched {
		return nil, nil
	}
	terms := make([]ast.ASTHandle, 0, 4)
	ok, err := p.linearTerm(multiplier, left, variable, result.coefficient, &terms, integer)
	if err == nil && ok {
		ok, err = p.linearTerm(new(big.Rat).Neg(multiplier), right, variable, result.coefficient, &terms, integer)
	}
	if err != nil || !ok {
		return nil, err
	}
	var term ast.ASTHandle
	switch len(terms) {
	case 0:
		term, err = p.syntax.manager.ArithmeticNumeralRat(new(big.Rat), integer)
	case 1:
		term = terms[0]
	default:
		term = terms[0]
		for _, next := range terms[1:] {
			term, err = p.syntax.manager.MkArithmeticAdd(term, next)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return nil, err
	}
	result.term, err = p.syntax.manager.NewRef(term)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func closeLegacyLinearLiterals(values ...*legacyLinearLiteral) (result error) {
	for _, value := range values {
		if value != nil {
			result = errors.Join(result, value.Close())
		}
	}
	return result
}
