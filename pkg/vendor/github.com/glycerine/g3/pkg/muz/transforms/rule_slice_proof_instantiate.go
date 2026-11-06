package transforms

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// instantiateAsserted translates sliced substitutions into the original
// rule's variable bank, keeping removed variables quantified.
func (c *SliceProofConverter) instantiateAsserted(original *base.Rule, originalProof ast.ASTRef, variables []uint32, input []ast.ASTHandle) (rule *base.Rule, proof ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil {
			if rule != nil {
				resultErr = errors.Join(resultErr, rule.Close())
				rule = nil
			}
			if proof != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proof))
			}
		}
	}()
	sorts, err := original.VariableSorts()
	if err != nil {
		return nil, proof, err
	}
	if len(input) > len(variables) {
		return nil, proof, fmt.Errorf("%w: substitution exceeds sliced variables", ErrSliceProofUnsupported)
	}
	bindings := make([]ast.ASTHandle, len(sorts))
	var refs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, refs)) }()
	usedInput, err := ast.NewASTUsedVars(m)
	if err != nil {
		return nil, proof, err
	}
	for i, term := range input {
		if variables[i] == ^uint32(0) {
			continue
		}
		if int(variables[i]) >= len(bindings) || term.IsNull() {
			return nil, proof, ast.ErrASTDomain
		}
		sort, e := m.GetSort(term)
		if e != nil {
			return nil, proof, e
		}
		if sort != sorts[variables[i]] {
			return nil, proof, ast.ErrASTDomain
		}
		if e = usedInput.Process(term); e != nil {
			return nil, proof, e
		}
	}
	inputMap := make([]ast.ASTHandle, usedInput.MaxFoundVarIndexPlusOne())
	for i := range inputMap {
		sort := usedInput.Contains(uint32(i))
		if sort.IsNull() {
			continue
		}
		var index uint32
		mapped := i < len(variables) && variables[i] != ^uint32(0)
		if mapped && int(variables[i]) >= len(sorts) {
			return nil, proof, ast.ErrASTDomain
		}
		// Terms in a hyper-resolution substitution belong to its result
		// variable bank, not necessarily the premise's bank. An index can
		// therefore denote a different sort than the corresponding premise
		// variable; give that result variable a fresh slot instead of rejecting
		// an otherwise well-sorted substitution.
		if mapped && sort == sorts[variables[i]] {
			index = variables[i]
		} else {
			fresh := uint64(len(sorts)) + uint64(i)
			if fresh > uint64(^uint32(0)) {
				return nil, proof, ast.ErrASTDomain
			}
			index = uint32(fresh)
		}
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		refs = append(refs, ref)
		variable, e := m.MkVar(index, sort)
		e = errors.Join(e, m.SetRef(ref, variable))
		if e != nil {
			return nil, proof, e
		}
		inputMap[i] = variable
	}
	var sub *rewriter.VarSubstitution
	if len(inputMap) != 0 {
		sub, err = rewriter.NewVarSubstitution(m, false)
		if err != nil {
			if sub != nil {
				err = errors.Join(err, sub.Close())
			}
			return nil, proof, err
		}
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	for i, term := range input {
		if variables[i] == ^uint32(0) {
			continue
		}
		if sub != nil {
			ref, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return nil, proof, e
			}
			refs = append(refs, ref)
			if e = sub.Substitute(term, inputMap, ref); e != nil {
				return nil, proof, e
			}
			term, e = m.RefGet(ref)
			if e != nil {
				return nil, proof, e
			}
		}
		bindings[variables[i]] = term
	}
	needsHeadCheck := false
	for _, binding := range bindings {
		if binding.IsNull() {
			continue
		}
		kind, e := m.GetKind(binding)
		if e != nil {
			return nil, proof, e
		}
		if kind == ast.ASTVar {
			continue
		}
		value, e := m.IsValue(binding)
		if e != nil {
			return nil, proof, e
		}
		needsHeadCheck = needsHeadCheck || !value
	}
	if needsHeadCheck {
		if sub == nil {
			sub, err = rewriter.NewVarSubstitution(m, false)
			if err != nil {
				if sub != nil {
					err = errors.Join(err, sub.Close())
				}
				return nil, proof, err
			}
			defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
		}
		head, e := original.Head()
		if e != nil {
			return nil, proof, e
		}
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		refs = append(refs, ref)
		if e = sub.Substitute(head, bindings, ref); e != nil {
			return nil, proof, e
		}
		head, e = m.RefGet(ref)
		if e != nil {
			return nil, proof, e
		}
		count, e := m.GetNumArgs(head)
		if e != nil {
			return nil, proof, e
		}
		for i := uint32(0); i < count; i++ {
			arg, e := m.GetArg(head, i)
			if e != nil {
				return nil, proof, e
			}
			kind, e := m.GetKind(arg)
			if e != nil {
				return nil, proof, e
			}
			if kind == ast.ASTVar {
				continue
			}
			value, e := m.IsValue(arg)
			if e != nil {
				return nil, proof, e
			}
			if !value {
				return c.instantiateCompoundHead(original, originalProof, bindings)
			}
		}
	}
	temporary, err := c.rules.SubstituteRule(original, bindings)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, temporary.Close()) }()
	used, err := temporary.UsedVariables()
	if err != nil {
		return nil, proof, err
	}
	renaming := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	var next uint32
	for i := range renaming {
		sort := used.Contains(uint32(i))
		if sort.IsNull() {
			continue
		}
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		refs = append(refs, ref)
		variable, e := m.MkVar(next, sort)
		e = errors.Join(e, m.SetRef(ref, variable))
		if e != nil {
			return nil, proof, e
		}
		renaming[i] = variable
		next++
	}
	if sub == nil && len(renaming) != 0 {
		sub, err = rewriter.NewVarSubstitution(m, false)
		if err != nil {
			if sub != nil {
				err = errors.Join(err, sub.Close())
			}
			return nil, proof, err
		}
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	for i := range bindings {
		if bindings[i].IsNull() {
			if i >= len(renaming) || renaming[i].IsNull() {
				return nil, proof, fmt.Errorf("%w: missing substituted variable", ErrSliceProofUnsupported)
			}
			bindings[i] = renaming[i]
		} else if sub != nil {
			ref, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return nil, proof, e
			}
			refs = append(refs, ref)
			if e = sub.Substitute(bindings[i], renaming, ref); e != nil {
				return nil, proof, e
			}
			bindings[i], e = m.RefGet(ref)
			if e != nil {
				return nil, proof, e
			}
		}
	}
	head, err := temporary.Head()
	if err != nil {
		return nil, proof, err
	}
	count, err := temporary.TailSize()
	if err != nil {
		return nil, proof, err
	}
	tails, negative := make([]ast.ASTHandle, count), make([]bool, count)
	for i := range tails {
		tails[i], negative[i], err = temporary.Tail(i)
		if err != nil {
			return nil, proof, err
		}
	}
	name, err := temporary.NameSymbol()
	if err != nil {
		return nil, proof, err
	}
	rule, err = c.rules.NewRuleWithSymbol(head, tails, negative, name)
	if err != nil {
		return nil, proof, err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return rule, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&formula)) }()
	fact, err := m.RefGet(formula)
	if err != nil {
		return rule, proof, err
	}
	parent, err := m.RefGet(originalProof)
	if err != nil {
		return rule, proof, err
	}
	proof, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return rule, proof, err
	}
	node, err := m.MkProofHyperResolve([]ast.ASTHandle{parent}, fact, nil, [][]ast.ASTHandle{bindings})
	if err != nil {
		return rule, proof, err
	}
	err = m.SetRef(proof, node)
	return rule, proof, err
}
