package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, linear::mk_rule_vars.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// linearRuleBindings owns one reference for each populated variable slot.
// Empty slots correspond to gaps in the rule's variable indices.
func (c *Compiler) linearRuleBindings(rule *base.Rule, predicate ast.ASTHandle, ruleIndex, level uint32) (result []ast.ASTRef, resultErr error) {
	m := c.manager
	sorts, err := rule.VariableSorts()
	if err != nil {
		return nil, err
	}
	result = make([]ast.ASTRef, len(sorts))
	defer func() {
		owner := &linearBindingOwner{manager: m, outputs: result}
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	bindArguments := func(application, declaration ast.ASTHandle, atLevel uint32) error {
		arity, err := m.GetArity(declaration)
		if err != nil {
			return err
		}
		for i := uint32(0); i < arity; i++ {
			arg, err := m.GetArg(application, i)
			if err != nil {
				return err
			}
			kind, err := m.GetKind(arg)
			if err != nil {
				return err
			}
			if kind != ast.ASTVar {
				continue
			}
			index, err := m.GetVarIndex(arg)
			if err != nil || int(index) >= len(result) {
				return errors.Join(ast.ErrASTDomain, err)
			}
			if result[index] == (ast.ASTRef{}) {
				result[index], err = c.linearArgument(declaration, i, atLevel)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	if err = bindArguments(head, predicate, level); err != nil {
		return result, err
	}
	tails, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	for i := 0; i < tails; i++ {
		if level == 0 {
			return result, ast.ErrASTDomain
		}
		tail, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		decl, err := rule.TailDeclaration(i)
		if err != nil {
			return result, err
		}
		if err = bindArguments(tail, decl, level-1); err != nil {
			return result, err
		}
	}
	var fresh uint32
	for i, sort := range sorts {
		if sort.IsNull() || result[i] != (ast.ASTRef{}) {
			continue
		}
		result[i], err = c.linearName(predicate, fmt.Sprintf("#%d_%d_%d", level, ruleIndex, fresh), sort)
		if err != nil {
			return result, err
		}
		fresh++
	}
	return result, nil
}

// LinearModelBindings extracts the source rule-variable substitution from a
// winning linear BMC model. Constants without interpretations become bound
// variables. The caller owns each nonempty returned reference.
func (c *Compiler) LinearModelBindings(rule *base.Rule, ruleIndex, level uint32, md *model.ModelCore) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rule == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	predicate, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	bindings, err := c.linearRuleBindings(rule, predicate, ruleIndex, level)
	if err != nil {
		return nil, err
	}
	defer func() {
		owner := &linearBindingOwner{manager: m, inputs: bindings, outputs: result}
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	result = make([]ast.ASTRef, len(bindings))
	for i, binding := range bindings {
		if binding == (ast.ASTRef{}) {
			continue
		}
		constant, err := m.RefGet(binding)
		if err != nil {
			return result, err
		}
		decl, err := m.GetDecl(constant)
		if err != nil {
			return result, err
		}
		value, err := md.ConstantInterpretation(decl)
		if err != nil {
			return result, err
		}
		if value.IsNull() {
			sort, err := m.GetSort(constant)
			if err != nil {
				return result, err
			}
			value, err = m.MkVar(uint32(i), sort)
			if err != nil {
				return result, err
			}
		}
		result[i], err = m.NewRef(value)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
