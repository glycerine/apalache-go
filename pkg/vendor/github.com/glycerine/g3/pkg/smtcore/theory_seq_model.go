package smtcore

// Source: src/smt/theory_seq.cpp seq_value_proc and theory_seq::mk_value.

import (
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceModelSourceKind uint8

const (
	smtSequenceModelLiteral smtSequenceModelSourceKind = iota
	smtSequenceModelSequence
	smtSequenceModelUnit
	smtSequenceModelInteger
)

type smtSequenceModelSource struct {
	kind    smtSequenceModelSourceKind
	literal ast.ASTHandle
}

type smtSequenceValueProcedure struct {
	sort         ast.ASTHandle
	sources      []smtSequenceModelSource
	dependencies []SMTModelValueDependency
}

func (p *smtSequenceValueProcedure) Dependencies() []SMTModelValueDependency {
	if p == nil {
		return nil
	}
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}

func (*smtSequenceValueProcedure) IsFresh() bool { return false }

func (p *smtSequenceValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || generator == nil || p.sort.IsNull() || len(values) != len(p.dependencies) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager := generator.manager
	stringSort, err := manager.IsStringSort(p.sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if stringSort {
		var result strings.Builder
		dependency := 0
		for _, source := range p.sources {
			switch source.kind {
			case smtSequenceModelLiteral:
				text, matched, textErr := manager.SequenceStringValue(source.literal)
				if textErr != nil || !matched {
					if textErr != nil {
						return ast.ASTHandle{}, textErr
					}
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				result.WriteString(text)
			case smtSequenceModelSequence:
				text, matched, textErr := manager.SequenceStringValue(values[dependency])
				dependency++
				if textErr != nil || !matched {
					if textErr != nil {
						return ast.ASTHandle{}, textErr
					}
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				result.WriteString(text)
			case smtSequenceModelUnit:
				value, matched, valueErr := manager.SequenceUnitByteElementValue(values[dependency])
				dependency++
				if valueErr != nil || !matched {
					if valueErr != nil {
						return ast.ASTHandle{}, valueErr
					}
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				result.WriteByte(value)
			case smtSequenceModelInteger:
				text, integer, matched, textErr := manager.ArithmeticNumeralText(values[dependency])
				dependency++
				if textErr != nil || !matched || !integer {
					if textErr != nil {
						return ast.ASTHandle{}, textErr
					}
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				value, ok := new(big.Int).SetString(text, 10)
				if !ok {
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				if value.Sign() >= 0 {
					result.WriteString(value.String())
				}
			default:
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
		}
		if dependency != len(values) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return manager.MkString(result.String())
	}

	parts := make([]ast.ASTHandle, 0, len(p.sources))
	dependency := 0
	for _, source := range p.sources {
		switch source.kind {
		case smtSequenceModelLiteral:
			parts = append(parts, source.literal)
		case smtSequenceModelSequence:
			parts = append(parts, values[dependency])
			dependency++
		case smtSequenceModelUnit:
			unit, unitErr := manager.MkSequenceUnit(values[dependency])
			dependency++
			if unitErr != nil {
				return ast.ASTHandle{}, unitErr
			}
			parts = append(parts, unit)
		case smtSequenceModelInteger:
			return ast.ASTHandle{}, ast.ErrASTDomain
		default:
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	if dependency != len(values) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(parts) == 0 {
		return manager.MkSequenceEmpty(p.sort)
	}
	result := parts[len(parts)-1]
	for index := len(parts) - 2; index >= 0; index-- {
		result, err = manager.MkSequenceConcat(parts[index], result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

func (t *SMTSequenceTheory) appendModelDependency(procedure *smtSequenceValueProcedure, kind smtSequenceModelSourceKind, node *SMTEnode) error {
	dependency, err := NewSMTEnodeValueDependency(node)
	if err != nil {
		return err
	}
	procedure.sources = append(procedure.sources, smtSequenceModelSource{kind: kind})
	procedure.dependencies = append(procedure.dependencies, dependency)
	return nil
}

func (t *SMTSequenceTheory) appendModelExpression(procedure *smtSequenceValueProcedure, expression ast.ASTHandle, root *SMTEnode, generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, right, concat, err := state.manager.SequenceConcatParts(expression)
	if err != nil {
		return err
	}
	if concat {
		if err = t.appendModelExpression(procedure, left, root, generator); err != nil {
			return err
		}
		return t.appendModelExpression(procedure, right, root, generator)
	}
	if _, stringValue, textErr := state.manager.SequenceStringValue(expression); textErr != nil {
		return textErr
	} else if stringValue {
		procedure.sources = append(procedure.sources, smtSequenceModelSource{kind: smtSequenceModelLiteral, literal: expression})
		return nil
	}
	if empty, emptyErr := state.manager.IsSequenceEmpty(expression); emptyErr != nil {
		return emptyErr
	} else if empty {
		return nil
	}
	if unit, unitErr := state.manager.IsSequenceUnit(expression); unitErr != nil {
		return unitErr
	} else if unit {
		argument, argumentErr := state.manager.GetArg(expression, 0)
		if argumentErr != nil {
			return argumentErr
		}
		node := state.context.enodes[argument]
		if node == nil {
			return ast.ErrASTDomain
		}
		return t.appendModelDependency(procedure, smtSequenceModelUnit, node)
	}
	if argument, conversion, conversionErr := state.manager.SequenceIntToStringArgument(expression); conversionErr != nil {
		return conversionErr
	} else if conversion {
		node := state.context.enodes[argument]
		if node == nil {
			return ast.ErrASTDomain
		}
		return t.appendModelDependency(procedure, smtSequenceModelInteger, node)
	}
	if node := state.context.enodes[expression]; node != nil && node.Root() != root {
		return t.appendModelDependency(procedure, smtSequenceModelSequence, node)
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	value, err := generator.model.FreshValue(sort)
	if err != nil {
		return err
	}
	procedure.sources = append(procedure.sources, smtSequenceModelSource{kind: smtSequenceModelLiteral, literal: value})
	return nil
}

func (t *SMTSequenceTheory) structuredModelValue(expression ast.ASTHandle, root *SMTEnode, generator *SMTModelGenerator) (*smtSequenceValueProcedure, bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, false, err
	}
	_, _, concat, err := state.manager.SequenceConcatParts(expression)
	if err != nil {
		return nil, false, err
	}
	unit, err := state.manager.IsSequenceUnit(expression)
	if err != nil {
		return nil, false, err
	}
	_, conversion, err := state.manager.SequenceIntToStringArgument(expression)
	if err != nil {
		return nil, false, err
	}
	if !concat && !unit && !conversion {
		return nil, false, nil
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return nil, false, err
	}
	procedure := &smtSequenceValueProcedure{sort: sort}
	if err = t.appendModelExpression(procedure, expression, root, generator); err != nil {
		return nil, false, err
	}
	return procedure, true, nil
}
