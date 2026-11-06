package fp

// Source: dparser::parse_arg, mk_const and mk_symbol_const.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) closeArguments() (result error) {
	remaining := make([]ast.ASTRef, 0, len(p.argumentRefs))
	for i := len(p.argumentRefs) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.argumentRefs[i]))
		if p.argumentRefs[i] != (ast.ASTRef{}) {
			remaining = append(remaining, p.argumentRefs[i])
		}
	}
	for left, right := 0, len(remaining)-1; left < right; left, right = left+1, right-1 {
		remaining[left], remaining[right] = remaining[right], remaining[left]
	}
	p.argumentRefs = remaining
	return result
}

func (p *datalogParser) newArgumentRef() (ast.ASTRef, error) {
	reference, err := p.manager.NewRef(ast.ASTHandle{})
	if err == nil {
		p.argumentRefs = append(p.argumentRefs, reference)
	}
	return reference, err
}

func (p *datalogParser) releaseArgumentRef(reference *ast.ASTRef) error {
	if reference == nil {
		return ast.ErrASTHandle
	}
	original := *reference
	err := p.manager.DelRef(reference)
	if *reference == (ast.ASTRef{}) {
		for i := range p.argumentRefs {
			if p.argumentRefs[i] == original {
				copy(p.argumentRefs[i:], p.argumentRefs[i+1:])
				p.argumentRefs = p.argumentRefs[:len(p.argumentRefs)-1]
				break
			}
		}
	}
	return err
}

func (p *datalogParser) parseArguments(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	if p.token.kind != datalogLeftParen {
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		if arity != 0 {
			return nil, p.fail("too few arguments passed to predicate")
		}
		return nil, nil
	}
	p.advance()
	if p.token.kind == datalogRightParen {
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		if arity == 0 {
			return nil, p.fail("too many arguments passed to predicate")
		}
		return nil, p.fail("expected predicate argument")
	}
	var arguments []ast.ASTHandle
	for p.token.kind != datalogRightParen {
		index := uint32(len(arguments))
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		if index >= arity {
			return nil, p.fail("too many arguments passed to predicate")
		}
		sort, err := p.manager.GetDomain(declaration, index)
		if err != nil {
			return nil, err
		}
		argument, err := p.parseArgument(sort)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
		if p.token.kind == datalogRightParen {
			break
		}
		if p.token.kind == datalogComma {
			p.advance()
			if p.token.kind == datalogRightParen {
				if uint32(len(arguments)) >= arity {
					return nil, p.fail("too many arguments passed to predicate")
				}
				return nil, p.fail("expected predicate argument")
			}
		}
	}
	p.advance()
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	if uint32(len(arguments)) != arity {
		return nil, p.fail("too few arguments passed to predicate")
	}
	return arguments, nil
}

func (p *datalogParser) parseArgument(sort ast.ASTHandle) (ast.ASTHandle, error) {
	token := p.token
	p.advance()
	return p.ownedArgument(token, sort, true)
}

// Source parse_args stores each argument in an expr_ref_vector. Reserve its
// owning slot before constructing the expression, then keep it through the
// complete rule so variables remain available to subsequent occurrences.
func (p *datalogParser) ownedArgument(token datalogToken, sort ast.ASTHandle, createVariable bool) (ast.ASTHandle, error) {
	reference, err := p.newArgumentRef()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	argument, err := p.argumentFromToken(token, sort, createVariable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.manager.SetRef(reference, argument); err != nil {
		return ast.ASTHandle{}, err
	}
	return argument, nil
}

// argumentFromToken implements dparser::parse_arg and the constant/variable
// operands shared with parse_infix. Infix parsing disables creation so an
// identifier must name a variable typed by an earlier predicate occurrence.
func (p *datalogParser) argumentFromToken(token datalogToken, sort ast.ASTHandle, createVariable bool) (ast.ASTHandle, error) {
	switch token.kind {
	case datalogWildcard:
		if !createVariable {
			return ast.ASTHandle{}, p.fail("wildcard is not an infix operand")
		}
		result, err := p.manager.MkVar(p.nextVar, sort)
		p.nextVar++
		return result, err
	case datalogIdentifier:
		if prior, exists := p.vars[token.text]; exists {
			priorSort, err := p.manager.GetSort(prior)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if priorSort != sort {
				return ast.ASTHandle{}, p.fail("variable used with inconsistent sorts")
			}
			return prior, nil
		}
		if !createVariable {
			return ast.ASTHandle{}, p.fail("infix relation requires a previously typed variable")
		}
		result, err := p.manager.MkVar(p.nextVar, sort)
		if err == nil {
			p.nextVar++
			p.vars[token.text] = result
		}
		return result, err
	case datalogString, datalogNumber:
		nameText := token.text
		if token.kind == datalogNumber && createVariable {
			// parse_arg uses mk_symbol_const(uint64), whereas parse_infix uses
			// mk_const(symbol). Only predicate numerals canonicalize spelling.
			if len(nameText) > 1 && nameText[0] == '-' && strings.Trim(nameText[1:], "0") == "" {
				nameText = "0"
			}
			value, err := strconv.ParseUint(nameText, 10, 64)
			if err != nil {
				return ast.ASTHandle{}, p.fail("invalid integer")
			}
			nameText = strconv.FormatUint(value, 10)
		}
		finite, err := p.manager.IsDatalogFiniteSort(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if finite {
			name, err := p.symbol(nameText)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			number, err := p.context.SymbolNumber(sort, name)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return p.manager.MkDatalogNumeral(number, sort)
		}
		if _, err = strconv.ParseUint(nameText, 10, 64); err != nil {
			return ast.ASTHandle{}, p.fail("invalid integer")
		}
		return p.manager.ArithmeticIntegerNumeralText(nameText)
	default:
		return ast.ASTHandle{}, p.fail("expected predicate argument")
	}
}
