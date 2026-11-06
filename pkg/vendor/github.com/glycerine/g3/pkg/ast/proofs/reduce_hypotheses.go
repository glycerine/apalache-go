package proofs

// Source: src/ast/proofs/proof_utils.cpp:374-420,468-584,683-727.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type hypothesisReducer struct {
	manager *ast.ASTManager
	sets    *hypothesisSets
	cache   map[ast.ASTHandle]ast.ASTHandle
	units   map[ast.ASTHandle]ast.ASTHandle
	trail   []ast.ASTHandle
	refs    []ast.ASTRef
}

func (r *hypothesisReducer) pin(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err != nil {
		return err
	}
	r.refs = append(r.refs, ref)
	return nil
}

func (r *hypothesisReducer) close() error {
	var result error
	for i := range r.refs {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.sets.reset()
	clear(r.cache)
	clear(r.units)
	r.trail = nil
	return result
}

func (r *hypothesisReducer) popUnits(limit int) {
	for len(r.trail) > limit {
		last := r.trail[len(r.trail)-1]
		delete(r.units, last)
		r.trail = r.trail[:len(r.trail)-1]
	}
}

func (r *hypothesisReducer) complementary(a, b ast.ASTHandle) (bool, error) {
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

func (r *hypothesisReducer) complement(e ast.ASTHandle) (ast.ASTHandle, ast.ASTRef, error) {
	yes, child, err := r.manager.IsNot(e)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	if yes {
		ref, e := r.manager.NewRef(child)
		return child, ref, e
	}
	other, err := r.manager.MkNot(e)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	ref, err := r.manager.NewRef(other)
	return other, ref, err
}

func (r *hypothesisReducer) inHypotheses(e ast.ASTHandle, set map[ast.ASTHandle]struct{}) (bool, error) {
	if set == nil {
		return false, nil
	}
	other, ref, err := r.complement(e)
	if err != nil {
		return false, err
	}
	_, found := set[other]
	return found, r.manager.DelRef(&ref)
}

func (r *hypothesisReducer) literals(clause ast.ASTHandle) ([]ast.ASTHandle, error) {
	kind, err := r.manager.GetKind(clause)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTApp {
		family, e := r.manager.GetFamilyID(clause)
		if e != nil {
			return nil, e
		}
		op, e := r.manager.GetDeclKind(clause)
		if e != nil {
			return nil, e
		}
		if family == r.manager.BasicFamilyID() && op == ast.ASTOpOr {
			n, e := r.manager.GetNumArgs(clause)
			if e != nil {
				return nil, e
			}
			result := make([]ast.ASTHandle, 0, n)
			for i := uint32(0); i < n; i++ {
				arg, e := r.manager.GetArg(clause, i)
				if e != nil {
					return nil, e
				}
				result = append(result, arg)
			}
			return result, nil
		}
	}
	return []ast.ASTHandle{clause}, nil
}

func (r *hypothesisReducer) literalInClause(literal, clause ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(clause)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(clause)
	if err != nil {
		return false, err
	}
	op, err := r.manager.GetDeclKind(clause)
	if err != nil || family != r.manager.BasicFamilyID() || op != ast.ASTOpOr {
		return false, err
	}
	args, err := r.literals(clause)
	if err != nil {
		return false, err
	}
	for _, arg := range args {
		if arg == literal {
			return true, nil
		}
	}
	return false, nil
}

func (r *hypothesisReducer) unitResolution(p ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := r.manager.GetProofNumParents(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if n < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	first, err := r.manager.GetProofParent(p, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parents := []ast.ASTHandle{first}
	limit := len(r.trail)
	defer r.popUnits(limit)
	for i := uint32(1); i < n; i++ {
		parent, e := r.manager.GetProofParent(p, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		unit, e := r.elim(parent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		fact, e := r.manager.GetProofFact(unit)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if r.manager.IsFalse(fact) {
			return unit, nil
		}
		if ast.ASTSourceDebug {
			oldFact, e := r.manager.GetProofFact(parent)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if fact != oldFact {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
		}
		parents = append(parents, unit)
		if len(r.sets.byProof[unit]) == 0 {
			if _, exists := r.units[fact]; !exists {
				r.units[fact] = unit
				r.trail = append(r.trail, fact)
			}
		}
	}
	oldClause, err := r.manager.GetProofFact(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	clauseProof, err := r.elim(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parents[0] = clauseProof
	clause, err := r.manager.GetProofFact(clauseProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if r.manager.IsFalse(clause) {
		if err := r.pin(clauseProof); err != nil {
			return ast.ASTHandle{}, err
		}
		return clauseProof, nil
	}
	contained, err := r.literalInClause(clause, oldClause)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if contained {
		for i := 1; i < len(parents); i++ {
			fact, e := r.manager.GetProofFact(parents[i])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			complement, e := r.complementary(clause, fact)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if complement {
				result, e := r.manager.MkProofUnitResolution([]ast.ASTHandle{clauseProof, parents[i]})
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if e = r.pin(result); e != nil {
					return ast.ASTHandle{}, e
				}
				return result, r.sets.add(result)
			}
		}
		return clauseProof, nil
	}
	literals, err := r.literals(clause)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := 1; i < len(parents); {
		fact, e := r.manager.GetProofFact(parents[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		found := false
		for _, literal := range literals {
			found, e = r.complementary(literal, fact)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if found {
				break
			}
		}
		if !found {
			parents[i] = parents[len(parents)-1]
			parents = parents[:len(parents)-1]
			continue
		}
		i++
	}
	if len(parents) == 1 {
		return clauseProof, nil
	}
	result, err := r.manager.MkProofUnitResolution(parents)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, r.sets.add(result)
}

func (r *hypothesisReducer) lemma(p ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := r.manager.GetProofNumParents(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if n != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	parent, err := r.manager.GetProofParent(p, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	child, err := r.elim(parent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	hyps := r.sets.byProof[child]
	var remaining map[ast.ASTHandle]struct{}
	var remainingOwner *hypothesisSetOwner
	if hyps != nil {
		remainingOwner, err = r.sets.newOwnedSet(517, hyps)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		remaining = remainingOwner.values
	}
	fact, err := r.manager.GetProofFact(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	whole, err := r.inHypotheses(fact, hyps)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var literals []ast.ASTHandle
	if hyps != nil && len(hyps) == 1 && whole {
		literals = []ast.ASTHandle{fact}
	} else {
		literals, err = r.literals(fact)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for i := 0; i < len(literals); {
		found, e := r.inHypotheses(literals[i], hyps)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !found {
			literals[i] = literals[len(literals)-1]
			literals = literals[:len(literals)-1]
			continue
		}
		complement, ref, e := r.complement(literals[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		delete(remaining, complement)
		if e = r.manager.DelRef(&ref); e != nil {
			return ast.ASTHandle{}, e
		}
		i++
	}
	result := child
	if len(literals) != 0 {
		clause := literals[0]
		if len(literals) > 1 {
			clause, err = r.manager.MkOrArray(literals)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if err = r.pin(clause); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		result, err = r.manager.MkProofLemma(child, clause)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = r.pin(result); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if len(remaining) == 0 {
		r.sets.releaseOwnedSet(remainingOwner, "src/ast/proofs/proof_utils.cpp", 569)
		remaining = nil
	}
	r.sets.byProof[result] = remaining
	return result, nil
}

func (r *hypothesisReducer) elim(p ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := r.cache[p]; found {
		return result, nil
	}
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
	result := p
	switch {
	case hypothesis:
		fact, e := r.manager.GetProofFact(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if replacement, found := r.units[fact]; found {
			result = replacement
		}
		err = r.sets.add(result)
	case lemma:
		result, err = r.lemma(p)
	case unit:
		result, err = r.unitResolution(p)
	default:
		var changed bool
		args := make([]ast.ASTHandle, 0)
		n, e := r.manager.GetProofNumParents(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for i := uint32(0); i < n; i++ {
			parent, e := r.manager.GetProofParent(p, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			child, e := r.elim(parent)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			fact, e := r.manager.GetProofFact(child)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if r.manager.IsFalse(fact) {
				result = child
				goto done
			}
			changed = changed || child != parent
			args = append(args, child)
		}
		hasFact, e := r.manager.ProofHasFact(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if hasFact {
			fact, e := r.manager.GetProofFact(p)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args = append(args, fact)
		}
		if changed {
			decl, e := r.manager.GetDecl(p)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = r.manager.MkApp(decl, args)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if e = r.pin(result); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		err = r.sets.add(result)
	}
done:
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.cache[p] = result
	return result, nil
}

// ReduceHypotheses applies the retained proof_utils reduction to an owning
// proof reference, then releases its temporary proof and expression roots.
func ReduceHypotheses(m *ast.ASTManager, proof ast.ASTRef) (err error) {
	if m == nil {
		return ast.ErrASTHandle
	}
	root, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	r := &hypothesisReducer{manager: m, sets: newHypothesisSets(m), cache: make(map[ast.ASTHandle]ast.ASTHandle), units: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() { err = errors.Join(err, r.close()) }()
	result, err := r.elim(root)
	if err != nil {
		return err
	}
	return m.SetRef(proof, result)
}
