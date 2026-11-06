package util

import (
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

type PatternValidator struct {
	manager      *ast.ASTManager
	basic, label ast.FamilyID
	warnings     io.Writer
}

func NewPatternValidator(manager *ast.ASTManager, warnings io.Writer) (*PatternValidator, error) {
	if manager == nil {
		return nil, fmt.Errorf("parser: nil AST manager")
	}
	name, err := manager.Symbols.Intern("label")
	if err != nil {
		return nil, err
	}
	label, err := manager.FamilyIDForSymbol(name)
	if err != nil {
		return nil, err
	}
	if warnings == nil {
		warnings = io.Discard
	}
	return &PatternValidator{manager: manager, basic: manager.BasicFamilyID(), label: label, warnings: warnings}, nil
}

func (v *PatternValidator) warn(line, position uint32, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(v.warnings, "(%d,%d): "+format+"\n", append([]interface{}{line, position}, args...)...)
}

func (v *PatternValidator) ValidateNew(numNewBindings uint32, expression ast.ASTHandle, line, position uint32) (bool, error) {
	return v.Validate(math.MaxUint32, numNewBindings, expression, line, position)
}

func (v *PatternValidator) Validate(numBindings, numNewBindings uint32, expression ast.ASTHandle, line, position uint32) (bool, error) {
	kind, err := v.manager.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		v.warn(line, position, "invalid pattern: variable.")
		return false, nil
	}
	found := make(map[uint32]bool)
	foundAny := false
	valid := true
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		kind, err = v.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			index, readErr := v.manager.GetVarIndex(current)
			if readErr != nil {
				return false, readErr
			}
			if index >= numBindings {
				v.warn(line, position, "free variables cannot be used in patterns.")
				valid = false
			} else if index < numNewBindings {
				foundAny = true
				found[index] = true
			}
		case ast.ASTQuantifier:
			valid = false
		case ast.ASTApp:
			family, readErr := v.manager.GetFamilyID(current)
			if readErr != nil {
				return false, readErr
			}
			declKind, readErr := v.manager.GetDeclKind(current)
			if readErr != nil {
				return false, readErr
			}
			if family == v.label || family == v.basic && declKind != ast.ASTOpEq && declKind != ast.ASTOpTrue && declKind != ast.ASTOpFalse {
				decl, declErr := v.manager.GetDecl(current)
				if declErr != nil {
					return false, declErr
				}
				name, nameErr := v.manager.GetASTName(decl)
				if nameErr != nil {
					return false, nameErr
				}
				text, textErr := v.manager.Symbols.String(name)
				if textErr != nil {
					return false, textErr
				}
				v.warn(line, position, "'%s' cannot be used in patterns.", text)
				valid = false
			}
			count, countErr := v.manager.GetNumArgs(current)
			if countErr != nil {
				return false, countErr
			}
			for i := count; i > 0; i-- {
				child, childErr := v.manager.GetArg(current, i-1)
				if childErr != nil {
					return false, childErr
				}
				stack = append(stack, child)
			}
		}
	}
	if !valid {
		return false, nil
	}
	if !foundAny {
		v.warn(line, position, "pattern does not contain any variable.")
		return false, nil
	}
	if uint32(len(found)) != numNewBindings {
		v.warn(line, position, "pattern does not contain all quantified variables.")
		return false, nil
	}
	return true, nil
}
