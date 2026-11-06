package tab

// Source: src/muz/tab/tab_context.cpp, imp::get_proof and resolve_rule.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// Proof replays the current successful path and returns an owning proof reference.
func (s *tabSearch) Proof() (result ast.ASTRef, err error) {
	if s == nil || s.closed || len(s.stack) == 0 || len(s.stack[len(s.stack)-1].predicates) != 0 {
		return result, ast.ErrASTDomain
	}
	m := s.manager
	attempt := &tabProofAttempt{manager: m}
	defer func() { result, err = finishTabProofAttempt(attempt, result, err) }()
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	attempt.scope = scope
	converter, err := tactic.NewReplaceProofConverter(m)
	if err != nil {
		return result, err
	}
	attempt.converter = converter
	current := s.stack[len(s.stack)-1]
	root, err := current.Formula()
	if err != nil {
		return result, err
	}
	attempt.root = root
	for current.index != 0 {
		if current.parentIndex >= current.index || int(current.parentIndex) >= len(s.stack) {
			return result, ast.ErrASTDomain
		}
		parent := s.stack[current.parentIndex]
		selected, err := m.RefGet(parent.predicates[parent.predicateIndex])
		if err != nil {
			return result, err
		}
		decl, err := m.GetDecl(selected)
		if err != nil {
			return result, err
		}
		rule, err := s.rules.Rule(decl, int(current.parentRule))
		if err != nil {
			return result, err
		}
		if err = s.insertResolutionProof(converter, parent, rule, current); err != nil {
			return result, err
		}
		current = parent
	}
	if err = converter.Invert(); err != nil {
		return result, err
	}
	formula, err := m.RefGet(root)
	if err != nil {
		return result, err
	}
	asserted, err := m.MkProofAsserted(formula)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = converter.Convert(m, []ast.ASTHandle{asserted}, result); err != nil {
		return result, err
	}
	return result, nil
}
func (s *tabSearch) insertResolutionProof(converter interface{ Insert(ast.ASTHandle) error }, parent, rule, child *Clause) (err error) {
	m := s.manager
	attempt := &tabResolutionProofAttempt{manager: m}
	defer func() { err = finishTabResolutionProofAttempt(attempt, err) }()
	unfolded, matched, err := unfoldClauseExpressionsWithBindings(parent, parent.predicateIndex, rule, true)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	attempt.unfolded = unfolded
	replayed, matched, err := finalizeUnfoldedClause(unfolded, s.rewriter, s.lite)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	attempt.replayed = replayed
	sub, err := unfolded.proofSubstitutions(parent, rule)
	if err != nil {
		return err
	}
	attempt.sub = sub
	substitutions := make([][]ast.ASTHandle, 2)
	for bank := range sub.banks {
		for _, ref := range sub.banks[bank] {
			h, e := m.RefGet(ref)
			if e != nil {
				return e
			}
			substitutions[bank] = append(substitutions[bank], h)
		}
	}
	var premises []ast.ASTHandle
	for _, clause := range []*Clause{parent, rule} {
		ref, e := clause.Formula()
		if e != nil {
			return e
		}
		attempt.refs = append(attempt.refs, ref)
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		proof, e := m.MkProofAsserted(h)
		if e != nil {
			return e
		}
		pin, e := m.NewRef(proof)
		if e != nil {
			return e
		}
		attempt.refs = append(attempt.refs, pin)
		premises = append(premises, proof)
	}
	conclusionRef, err := child.Formula()
	if err != nil {
		return err
	}
	attempt.refs = append(attempt.refs, conclusionRef)
	conclusion, err := m.RefGet(conclusionRef)
	if err != nil {
		return err
	}
	proof, err := m.MkProofHyperResolve(premises, conclusion, []ast.ASTProofPosition{{First: parent.predicateIndex + 1, Second: 0}}, substitutions)
	if err != nil {
		return err
	}
	return converter.Insert(proof)
}
