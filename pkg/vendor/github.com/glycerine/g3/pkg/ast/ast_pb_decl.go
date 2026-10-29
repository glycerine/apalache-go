package ast

// Source: src/ast/pb_decl_plugin.cpp:23-101; src/ast/pb_decl_plugin.h:32-78.

import (
	"errors"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	ASTPBOpAtMost int32 = iota
	ASTPBOpAtLeast
	ASTPBOpLE
	ASTPBOpGE
	ASTPBOpEQ
	ASTPBOpAuxBool
	ASTPBLastOp
)

func (m *ASTManager) pseudoBooleanDecl(k int32, ps []ASTParameter, domain []ASTHandle) (result astNodeRef, err error) {
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil || k < ASTPBOpAtMost || k > ASTPBOpEQ {
		return result, ErrASTDomain
	}
	for _, h := range domain {
		s, e := m.requireKind(h, ASTSort)
		if e != nil {
			return result, e
		}
		if s != m.basic.boolSort {
			return result, ErrASTDomain
		}
	}
	parameters := ps
	var locals []ASTParameter
	defer func() {
		for _, value := range locals {
			err = errors.Join(err, m.dropParameter(value))
		}
	}()
	switch k {
	case ASTPBOpAtMost, ASTPBOpAtLeast:
		if len(ps) != 1 {
			return result, ErrASTDomain
		}
		value, e := m.parameterOfKind(ps[0], ASTParameterInt)
		if e != nil || value.integer < 0 {
			return result, ErrASTDomain
		}
	case ASTPBOpLE, ASTPBOpGE, ASTPBOpEQ:
		if len(ps) != len(domain)+1 {
			return result, ErrASTDomain
		}
		parameters = make([]ASTParameter, len(ps))
		copy(parameters, ps)
		for i, parameter := range ps {
			value, e := m.parameter(parameter)
			if e != nil {
				return result, e
			}
			switch value.Kind {
			case ASTParameterInt:
			case ASTParameterRational:
				fits, e := m.numerals.IsInt32(&value.Rational)
				if e != nil {
					return result, e
				}
				if fits {
					n, e := m.numerals.Int32(&value.Rational)
					if e != nil {
						return result, e
					}
					converted, e := m.newParameterInt(n)
					if e != nil {
						return result, e
					}
					locals = append(locals, converted)
					parameters[i] = converted
				}
			default:
				return result, ErrASTDomain
			}
		}
	}
	return m.pluginDecl(p.symbols[k], p.family, k, parameters, domain, m.basic.boolSort, ASTFuncFlags{}, false)
}

// GetPseudoBooleanOpNames preserves the source QF_FD/ALL visibility rule.
func (m *ASTManager) GetPseudoBooleanOpNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.auxiliaryPlugin(astPseudoBooleanPlugin) == nil {
		return ErrASTHandle
	}
	visible := logic.IsNull()
	if !visible {
		for _, text := range [...]string{"QF_FD", "ALL"} {
			yes, e := m.Symbols.EqualString(logic, &text)
			if e != nil {
				return e
			}
			if yes {
				visible = true
				break
			}
		}
	}
	if !visible {
		return nil
	}
	for _, entry := range [...]struct {
		text string
		kind int32
	}{
		{"at-most", ASTPBOpAtMost}, {"at-least", ASTPBOpAtLeast},
		{"pble", ASTPBOpLE}, {"pbge", ASTPBOpGE}, {"pbeq", ASTPBOpEQ},
	} {
		if e := m.appendBuiltinName(names, entry.text, entry.kind); e != nil {
			return e
		}
	}
	return nil
}
