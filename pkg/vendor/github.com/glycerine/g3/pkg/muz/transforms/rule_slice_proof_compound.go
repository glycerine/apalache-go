package transforms

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// instantiateCompoundHead first proves the substituted formula, then applies
// the rule manager's Horn conversion, which hoists compound predicate arguments.
func (c *SliceProofConverter) instantiateCompoundHead(original *base.Rule, originalProof ast.ASTRef, bindings []ast.ASTHandle) (rule *base.Rule, proof ast.ASTRef, resultErr error) {
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
	var refs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, refs)) }()
	hold := func(handle ast.ASTHandle) (ast.ASTRef, error) {
		ref, err := m.NewRef(handle)
		if err == nil {
			refs = append(refs, ref)
		}
		return ref, err
	}
	formula, err := original.ToFormula()
	if err != nil {
		return nil, proof, err
	}
	refs = append(refs, formula)
	body, err := m.RefGet(formula)
	if err != nil {
		return nil, proof, err
	}
	isForall, err := m.IsForall(body)
	if err != nil {
		return nil, proof, err
	}
	if isForall {
		body, err = m.GetQuantifierBody(body)
		if err != nil {
			return nil, proof, err
		}
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	rawRef, err := hold(ast.ASTHandle{})
	if err != nil {
		return nil, proof, err
	}
	if err = sub.Substitute(body, bindings, rawRef); err != nil {
		return nil, proof, err
	}
	raw, err := m.RefGet(rawRef)
	if err != nil {
		return nil, proof, err
	}
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return nil, proof, err
	}
	if err = used.Collect(raw); err != nil {
		return nil, proof, err
	}
	renaming := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	var sorts []ast.ASTHandle
	for i := range renaming {
		sort := used.Contains(uint32(i))
		if sort.IsNull() {
			continue
		}
		owner, e := hold(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		variable, e := m.MkVar(uint32(len(sorts)), sort)
		e = errors.Join(e, m.SetRef(owner, variable))
		if e != nil {
			return nil, proof, e
		}
		renaming[i] = variable
		sorts = append(sorts, sort)
	}
	normalRef, err := hold(ast.ASTHandle{})
	if err != nil {
		return nil, proof, err
	}
	if err = sub.Substitute(raw, renaming, normalRef); err != nil {
		return nil, proof, err
	}
	normal, err := m.RefGet(normalRef)
	if err != nil {
		return nil, proof, err
	}
	for i, binding := range bindings {
		if binding.IsNull() {
			if i >= len(renaming) || renaming[i].IsNull() {
				return nil, proof, ast.ErrASTDomain
			}
			bindings[i] = renaming[i]
			continue
		}
		ref, e := hold(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		if e = sub.Substitute(binding, renaming, ref); e != nil {
			return nil, proof, e
		}
		bindings[i], e = m.RefGet(ref)
		if e != nil {
			return nil, proof, e
		}
	}
	if len(sorts) != 0 {
		names := make([]symbol.Symbol, len(sorts))
		reverse := make([]ast.ASTHandle, len(sorts))
		for i := range sorts {
			names[i] = symbol.Numerical(uint32(i))
			reverse[len(sorts)-1-i] = sorts[i]
		}
		owner, e := hold(ast.ASTHandle{})
		if e != nil {
			return nil, proof, e
		}
		normal, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: reverse, Names: names, Body: normal})
		err = errors.Join(err, m.SetRef(owner, normal))
		if err != nil {
			return nil, proof, err
		}
	}
	parent, err := m.RefGet(originalProof)
	if err != nil {
		return nil, proof, err
	}
	instantiatedOwner, err := hold(ast.ASTHandle{})
	if err != nil {
		return nil, proof, err
	}
	instantiated, err := m.MkProofHyperResolve([]ast.ASTHandle{parent}, normal, nil, [][]ast.ASTHandle{bindings})
	if err != nil {
		return nil, proof, err
	}
	if err = m.SetRef(instantiatedOwner, instantiated); err != nil {
		return nil, proof, err
	}
	name, err := original.NameSymbol()
	if err != nil {
		return nil, proof, err
	}
	rule, err = c.rules.NewRuleFromFormulaWithProofAndSymbol(normal, instantiated, name)
	if err != nil {
		return nil, proof, err
	}
	converted, found, err := rule.Proof()
	if err != nil {
		return rule, proof, err
	}
	if !found {
		return rule, proof, ast.ErrASTDomain
	}
	proof, err = m.NewRef(converted)
	return rule, proof, err
}
