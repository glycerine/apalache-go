package proofs

// Source: src/ast/proofs/proof_utils.cpp:75-337.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type legacyHypothesisReducer struct {
	manager   *ast.ASTManager
	cache     map[ast.ASTHandle]ast.ASTHandle
	units     map[ast.ASTHandle]ast.ASTHandle
	unitOrder []ast.ASTHandle
	hyps      map[ast.ASTHandle]struct{}
	hypmark   map[ast.ASTHandle]bool
	pins      []ast.ASTRef
}

func (r *legacyHypothesisReducer) close() (err error) {
	for i := len(r.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, r.manager.DelRef(&r.pins[i]))
	}
	return err
}

func (r *legacyHypothesisReducer) pin(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.pins = append(r.pins, ref)
	}
	return err
}

func (r *legacyHypothesisReducer) insertUnit(fact, proof ast.ASTHandle) {
	if _, found := r.units[fact]; found {
		return
	}
	r.unitOrder = append(r.unitOrder, fact)
	r.units[fact] = proof
}

func (r *legacyHypothesisReducer) computeMark(p ast.ASTHandle) (bool, error) {
	lemma, err := r.manager.IsProofKind(p, ast.ASTPrLemma)
	if err != nil {
		return false, err
	}
	marked := false
	if !lemma {
		n, err := r.manager.GetProofNumParents(p)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < n; i++ {
			parent, err := r.manager.GetProofParent(p, i)
			if err != nil {
				return false, err
			}
			if r.hypmark[parent] {
				marked = true
				break
			}
		}
	}
	r.hypmark[p] = marked
	return marked, nil
}

func (r *legacyHypothesisReducer) computeMarks(root ast.ASTHandle) error {
	it, err := NewProofPostOrder(root, r.manager)
	if err != nil {
		return err
	}
	for {
		yes, err := it.HasNext()
		if err != nil {
			return err
		}
		if !yes {
			return nil
		}
		p, err := it.Next()
		if err != nil {
			return err
		}
		hypothesis, err := r.manager.IsProofKind(p, ast.ASTPrHypothesis)
		if err != nil {
			return err
		}
		if hypothesis {
			r.hypmark[p] = true
			fact, err := r.manager.GetProofFact(p)
			if err != nil {
				return err
			}
			r.hyps[fact] = struct{}{}
			continue
		}
		marked, err := r.computeMark(p)
		if err != nil {
			return err
		}
		hasFact, err := r.manager.ProofHasFact(p)
		if err != nil {
			return err
		}
		if !marked && hasFact {
			fact, err := r.manager.GetProofFact(p)
			if err != nil {
				return err
			}
			if _, wanted := r.hyps[fact]; wanted {
				r.insertUnit(fact, p)
			}
		}
	}
}

func (r *legacyHypothesisReducer) complement(a, b ast.ASTHandle) (bool, error) {
	if r.manager.IsTrue(a) && r.manager.IsFalse(b) || r.manager.IsFalse(a) && r.manager.IsTrue(b) {
		return true, nil
	}
	yes, child, err := r.manager.IsNot(a)
	if err != nil {
		return false, err
	}
	if yes && child == b {
		return true, nil
	}
	yes, child, err = r.manager.IsNot(b)
	return yes && child == a, err
}

func (r *legacyHypothesisReducer) literals(fact ast.ASTHandle) ([]ast.ASTHandle, error) {
	kind, err := r.manager.GetKind(fact)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTApp {
		fid, err := r.manager.GetFamilyID(fact)
		if err != nil {
			return nil, err
		}
		op, err := r.manager.GetDeclKind(fact)
		if err != nil {
			return nil, err
		}
		if fid == r.manager.BasicFamilyID() && op == ast.ASTOpOr {
			n, err := r.manager.GetNumArgs(fact)
			if err != nil {
				return nil, err
			}
			values := make([]ast.ASTHandle, n)
			for i := range values {
				values[i], err = r.manager.GetArg(fact, uint32(i))
				if err != nil {
					return nil, err
				}
			}
			return values, nil
		}
	}
	return []ast.ASTHandle{fact}, nil
}

func (r *legacyHypothesisReducer) isReduced(literal ast.ASTHandle) (bool, error) {
	yes, child, err := r.manager.IsNot(literal)
	if err != nil {
		return false, err
	}
	other := child
	var ref ast.ASTRef
	if !yes {
		other, err = r.manager.MkNot(literal)
		if err != nil {
			return false, err
		}
		ref, err = r.manager.NewRef(other)
		if err != nil {
			return false, err
		}
	}
	_, found := r.units[other]
	if ref != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&ref); err != nil {
			return false, err
		}
	}
	return found, nil
}

func (r *legacyHypothesisReducer) lemma(child, fact ast.ASTHandle) (ast.ASTHandle, error) {
	literals, err := r.literals(fact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kept := literals[:0]
	for _, literal := range literals {
		reduced, err := r.isReduced(literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !reduced {
			kept = append(kept, literal)
		}
	}
	if len(kept) == 0 {
		return child, nil
	}
	newFact := kept[0]
	if len(kept) > 1 {
		newFact, err = r.manager.MkOrArray(kept)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = r.pin(newFact); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result, err := r.manager.MkProofLemma(child, newFact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	if _, wanted := r.hyps[newFact]; wanted {
		r.insertUnit(newFact, result)
	}
	return result, nil
}

func (r *legacyHypothesisReducer) unitResolution(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	clause, err := r.manager.GetProofFact(args[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals, err := r.literals(clause)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parents := []ast.ASTHandle{args[0]}
	remaining := make([]ast.ASTHandle, 0, len(literals))
	for _, literal := range literals {
		found := false
		for _, candidate := range args[1:] {
			fact, err := r.manager.GetProofFact(candidate)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			found, err = r.complement(literal, fact)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if found {
				parents = append(parents, candidate)
				break
			}
		}
		if !found {
			remaining = append(remaining, literal)
		}
	}
	if len(remaining)+len(parents)-1 != len(literals) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	newFact, err := r.manager.MkOrArray(remaining)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.pin(newFact); err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := r.manager.MkProofUnitResolutionFact(parents, newFact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (r *legacyHypothesisReducer) reduce(root ast.ASTHandle) (ast.ASTHandle, error) {
	todo := []ast.ASTHandle{root}
	result := ast.ASTHandle{}
	for len(todo) != 0 {
		p := todo[len(todo)-1]
		if cached, found := r.cache[p]; found {
			result = cached
			todo = todo[:len(todo)-1]
			continue
		}
		n, err := r.manager.GetProofNumParents(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, 0, n)
		dirty := false
		before := len(todo)
		for i := uint32(0); i < n; i++ {
			parent, err := r.manager.GetProofParent(p, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if replacement, found := r.cache[parent]; found {
				args = append(args, replacement)
				dirty = dirty || replacement != parent
			} else {
				todo = append(todo, parent)
			}
		}
		if len(todo) != before {
			continue
		}
		todo = todo[:len(todo)-1]
		hypothesis, err := r.manager.IsProofKind(p, ast.ASTPrHypothesis)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lemma, err := r.manager.IsProofKind(p, ast.ASTPrLemma)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		unit, err := r.manager.IsProofKind(p, ast.ASTPrUnitResolution)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result = p
		switch {
		case hypothesis:
			fact, err := r.manager.GetProofFact(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if replacement, found := r.units[fact]; found {
				result = replacement
			}
		case !dirty:
		case lemma:
			if len(args) != 1 {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			fact, err := r.manager.GetProofFact(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = r.lemma(args[0], fact)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if _, err = r.computeMark(result); err != nil {
				return ast.ASTHandle{}, err
			}
		case unit:
			result, err = r.unitResolution(args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if _, err = r.computeMark(result); err != nil {
				return ast.ASTHandle{}, err
			}
		default:
			hasFact, err := r.manager.ProofHasFact(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if hasFact {
				fact, err := r.manager.GetProofFact(p)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args = append(args, fact)
			}
			decl, err := r.manager.GetDecl(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = r.manager.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if err = r.pin(result); err != nil {
				return ast.ASTHandle{}, err
			}
			if _, err = r.computeMark(result); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		if result.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		r.cache[p] = result
		hasFact, err := r.manager.ProofHasFact(result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if hasFact {
			fact, err := r.manager.GetProofFact(result)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if r.manager.IsFalse(fact) {
				break
			}
		}
	}
	return result, nil
}

// ReduceHypothesesLegacy ports the standalone reduce_hypotheses(proof_ref&)
// entry used by the legacy spacer unsat-core path.
func ReduceHypothesesLegacy(m *ast.ASTManager, proof ast.ASTRef) (err error) {
	if m == nil {
		return ast.ErrASTHandle
	}
	root, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	r := &legacyHypothesisReducer{manager: m, cache: make(map[ast.ASTHandle]ast.ASTHandle), units: make(map[ast.ASTHandle]ast.ASTHandle), hyps: make(map[ast.ASTHandle]struct{}), hypmark: make(map[ast.ASTHandle]bool)}
	defer func() { err = errors.Join(err, r.close()) }()
	if err = r.computeMarks(root); err != nil {
		return err
	}
	for _, fact := range r.unitOrder {
		candidate, e := r.reduce(r.units[fact])
		if e != nil {
			return e
		}
		candidateFact, e := m.GetProofFact(candidate)
		if e != nil {
			return e
		}
		if m.IsFalse(candidateFact) {
			return m.SetRef(proof, candidate)
		}
	}
	result, err := r.reduce(root)
	if err != nil {
		return err
	}
	return m.SetRef(proof, result)
}
