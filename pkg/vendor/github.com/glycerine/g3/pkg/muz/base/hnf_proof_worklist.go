package base

// Source: src/muz/base/hnf.cpp imp::mk_horn, eliminate_disjunctions,
// eliminate_quantifier_body and proof composition helpers.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type hornProofWork struct{ formula, proof ast.ASTHandle }
type hornProofDefinition struct{ head, proof ast.ASTHandle }
type hornProofNormalizer struct {
	n      *HornNormalizer
	result *HornNormalization
	refs   []ast.ASTRef
	work   []hornProofWork
	memo   map[ast.ASTHandle]hornProofDefinition
}

func (s *hornProofNormalizer) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.n.pin(&s.refs, h); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}

func (s *hornProofNormalizer) transitivity(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.n.manager
	for i, p := range []ast.ASTHandle{a, b} {
		if p.IsNull() {
			continue
		}
		fact, err := m.GetProofFact(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		left, err := m.GetArg(fact, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		right, err := m.GetArg(fact, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if left == right {
			if i == 0 {
				return b, nil
			}
			return a, nil
		}
	}
	return s.keep(m.MkProofTransitivity(a, b))
}

func (s *hornProofNormalizer) modusPonens(p, equivalence ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.n.manager
	result, err := s.keep(m.MkProofModusPonens(p, equivalence))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	old, err := m.GetProofFact(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fact, err := m.GetProofFact(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if old == fact {
		return p, nil
	}
	return result, nil
}

func (s *hornProofNormalizer) quantIntro(prefix hornPrefix, a, b, p ast.ASTHandle) (ast.ASTHandle, error) {
	if len(prefix.sorts) == 0 {
		return p, nil
	}
	fact, err := s.n.manager.GetProofFact(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := s.n.manager.GetDeclKind(fact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTOpOEq {
		return s.keep(s.n.manager.MkProofOEqQuantIntro(a, b, p))
	}
	return s.keep(s.n.manager.MkProofQuantIntro(a, b, p))
}

func (s *hornProofNormalizer) congruence(p ast.ASTHandle, body []ast.ASTHandle, head ast.ASTHandle, defs []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(defs) == 0 {
		return p, nil
	}
	m := s.n.manager
	formula, err := s.n.implication(body, head, &s.refs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fact, err := m.GetProofFact(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := m.GetDeclKind(fact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTOpIff {
		p, err = s.keep(m.MkProofIffOEq(p))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		fact, err = m.GetProofFact(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	previous, err := m.GetArg(fact, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	step, err := s.keep(m.MkProofOEqCongruence(previous, formula, defs))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.transitivity(p, step)
}

func (s *hornProofNormalizer) disjunction(expression ast.ASTHandle) (parts []ast.ASTHandle, matched bool, resultErr error) {
	n, m := s.n, s.n.manager
	contains, err := n.containsPredicate(expression)
	if err != nil || !contains {
		return nil, false, err
	}
	if err := n.traceHornBody(expression); err != nil {
		return nil, false, err
	}
	op, app, err := n.operator(expression)
	if err != nil || !app {
		return nil, false, err
	}
	root := expression
	negate := false
	switch op {
	case ast.ASTOpNot:
		root, err = m.GetArg(expression, 0)
		if err != nil {
			return nil, false, err
		}
		inner, app, err := n.operator(root)
		if err != nil || !app || inner != ast.ASTOpAnd {
			return nil, false, err
		}
		negate = true
	case ast.ASTOpOr:
	case ast.ASTOpImplies:
		left, err := m.GetArg(expression, 0)
		if err != nil {
			return nil, false, err
		}
		right, err := m.GetArg(expression, 1)
		if err != nil {
			return nil, false, err
		}
		left, err = s.keep(m.MkNotSimplified(left))
		if err != nil {
			return nil, false, err
		}
		return []ast.ASTHandle{left, right}, true, nil
	default:
		return nil, false, nil
	}
	count, err := m.GetNumArgs(root)
	if err != nil {
		return nil, false, err
	}
	for i := uint32(0); i < count; i++ {
		arg, err := m.GetArg(root, i)
		if err != nil {
			return nil, false, err
		}
		if negate {
			arg, err = s.keep(m.MkNot(arg))
			if err != nil {
				return nil, false, err
			}
		}
		parts = append(parts, arg)
	}
	return parts, true, nil
}

func (s *hornProofNormalizer) step(item hornProofWork, hoister *rewriter.QuantifierHoister) error {
	n, m := s.n, s.n.manager
	if err := n.beginHornStep(); err != nil {
		return err
	}
	core, prefix, err := n.pullForall(hoister, item.formula, &s.refs)
	if err != nil {
		return err
	}
	bound, err := n.bind(prefix, core, &s.refs)
	if err != nil {
		return err
	}
	premise := item.proof
	if len(prefix.sorts) != 0 {
		pull, err := s.keep(m.MkProofPullQuant(item.formula, bound))
		if err != nil {
			return err
		}
		premise, err = s.modusPonens(premise, pull)
		if err != nil {
			return err
		}
	}
	head := core
	var body []ast.ASTHandle
	for {
		op, app, err := n.operator(head)
		if err != nil {
			return err
		}
		if !app || op != ast.ASTOpImplies {
			break
		}
		left, err := m.GetArg(head, 0)
		if err != nil {
			return err
		}
		body = append(body, left)
		head, err = m.GetArg(head, 1)
		if err != nil {
			return err
		}
	}
	body, err = m.FlattenAnd(body)
	if err != nil {
		return err
	}
	if err = n.storeHornBody(body); err != nil {
		return err
	}
	normalized, err := n.implication(body, head, &s.refs)
	if err != nil {
		return err
	}
	rewrite, err := s.keep(m.MkProofRewrite(core, normalized))
	if err != nil {
		return err
	}
	if len(body) == 1 {
		op, app, err := n.operator(body[0])
		if err != nil {
			return err
		}
		contains, err := n.containsPredicate(body[0])
		if err != nil {
			return err
		}
		if app && op == ast.ASTOpOr && contains {
			count, err := m.GetNumArgs(body[0])
			if err != nil {
				return err
			}
			start := len(s.work)
			formulas := make([]ast.ASTHandle, 0, count)
			for i := uint32(0); i < count; i++ {
				disjunct, err := m.GetArg(body[0], i)
				if err != nil {
					return err
				}
				clause, err := n.implication([]ast.ASTHandle{disjunct}, head, &s.refs)
				if err != nil {
					return err
				}
				clause, err = n.bind(prefix, clause, &s.refs)
				if err != nil {
					return err
				}
				formulas = append(formulas, clause)
				s.work = append(s.work, hornProofWork{formula: clause})
			}
			from, err := n.bind(prefix, normalized, &s.refs)
			if err != nil {
				return err
			}
			conjunction, err := s.keep(m.MkAndArgs(formulas))
			if err != nil {
				return err
			}
			iff, err := s.keep(m.MkIff(from, conjunction))
			if err != nil {
				return err
			}
			split, err := s.keep(m.MkProofDefAxiom(iff))
			if err != nil {
				return err
			}
			lifted, err := s.quantIntro(prefix, bound, from, rewrite)
			if err != nil {
				return err
			}
			split, err = s.transitivity(lifted, split)
			if err != nil {
				return err
			}
			split, err = s.modusPonens(premise, split)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				p, err := s.keep(m.MkProofAndElim(split, i))
				if err != nil {
					return err
				}
				s.work[start+int(i)].proof = p
			}
			return nil
		}
	}
	var defs []ast.ASTHandle
	for i, expression := range body {
		parts, matched, err := s.disjunction(expression)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		definition, found := s.memo[expression]
		if !found {
			fresh, err := n.freshHead(expression, s.result, &s.refs)
			if err != nil {
				return err
			}
			var introductions []ast.ASTHandle
			for _, part := range parts {
				clause, err := n.implication([]ast.ASTHandle{part}, fresh, &s.refs)
				if err != nil {
					return err
				}
				clause, err = n.bind(prefix, clause, &s.refs)
				if err != nil {
					return err
				}
				intro, err := s.keep(m.MkProofDefIntro(clause))
				if err != nil {
					return err
				}
				introductions = append(introductions, intro)
				s.work = append(s.work, hornProofWork{clause, intro})
			}
			application, err := s.keep(m.MkProofApplyDefs(expression, fresh, introductions))
			if err != nil {
				return err
			}
			definition = hornProofDefinition{fresh, application}
			s.memo[expression] = definition
			for _, root := range []ast.ASTHandle{application, expression, fresh} {
				if err := n.rememberHornRoot(root); err != nil {
					return err
				}
			}
		}
		body[i] = definition.head
		if err := n.replaceHornBody(i, body[i]); err != nil {
			return err
		}
		defs = append(defs, definition.proof)
	}
	rewrite, err = s.congruence(rewrite, body, head, defs)
	if err != nil {
		return err
	}
	defs = nil
	for i, expression := range body {
		kind, err := m.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		forall, err := m.IsForall(expression)
		if err != nil {
			return err
		}
		if !forall {
			continue
		}
		contains, err := n.containsPredicate(expression)
		if err != nil {
			return err
		}
		if !contains {
			continue
		}
		inner, err := m.GetQuantifierBody(expression)
		if err != nil {
			return err
		}
		predicate, err := n.predicate(inner)
		if err != nil {
			return err
		}
		if predicate {
			continue
		}
		fresh, err := n.freshHead(inner, s.result, &s.refs)
		if err != nil {
			return err
		}
		clause, err := n.implication([]ast.ASTHandle{inner}, fresh, &s.refs)
		if err != nil {
			return err
		}
		clause, err = n.bind(prefix, clause, &s.refs)
		if err != nil {
			return err
		}
		intro, err := s.keep(m.MkProofDefIntro(clause))
		if err != nil {
			return err
		}
		s.work = append(s.work, hornProofWork{clause, intro})
		body[i], err = s.keep(m.UpdateQuantifierBody(expression, fresh))
		if err != nil {
			return err
		}
		if err = n.replaceHornBody(i, body[i]); err != nil {
			return err
		}
		application, err := s.keep(m.MkProofApplyDef(inner, fresh, intro))
		if err != nil {
			return err
		}
		negative, err := s.keep(m.MkProofNNFNeg(expression, body[i], []ast.ASTHandle{application}))
		if err != nil {
			return err
		}
		defs = append(defs, negative)
	}
	rewrite, err = s.congruence(rewrite, body, head, defs)
	if err != nil {
		return err
	}
	normalized, err = n.implication(body, head, &s.refs)
	if err != nil {
		return err
	}
	normalized, err = n.bind(prefix, normalized, &s.refs)
	if err != nil {
		return err
	}
	lifted, err := s.quantIntro(prefix, bound, normalized, rewrite)
	if err != nil {
		return err
	}
	proof, err := s.modusPonens(premise, lifted)
	if err != nil {
		return err
	}
	formulaRef, err := m.NewRef(normalized)
	if err != nil {
		return err
	}
	s.result.Formulas = append(s.result.Formulas, formulaRef)
	proofRef, err := m.NewRef(proof)
	if err != nil {
		return err
	}
	s.result.Proofs = append(s.result.Proofs, proofRef)
	return nil
}

// NormalizeWithProof follows the source proof-carrying Horn worklist. Returned
// formula/proof pairs and fresh predicates have independent owning references.
func (n *HornNormalizer) NormalizeWithProof(formula, proof ast.ASTHandle) (result *HornNormalization, resultErr error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	if proof.IsNull() {
		return n.Normalize(formula)
	}
	m := n.manager
	fact, err := m.GetProofFact(proof)
	if err != nil {
		return nil, err
	}
	if fact != formula {
		return nil, fmt.Errorf("muz/base: HNF proof does not prove the supplied formula")
	}
	s := &hornProofNormalizer{n: n, result: &HornNormalization{manager: m}, memo: make(map[ast.ASTHandle]hornProofDefinition)}
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(m, s.refs))
		if resultErr != nil {
			resultErr = errors.Join(resultErr, s.result.Close())
			result = nil
		}
	}()
	mode, err := m.ProofMode()
	if err != nil {
		return nil, err
	}
	if mode == ast.ASTProofDisabled {
		if err = m.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, m.ToggleProofMode(mode)) }()
	}
	if _, err = s.keep(formula, nil); err != nil {
		return nil, err
	}
	if _, err = s.keep(proof, nil); err != nil {
		return nil, err
	}
	horn, err := n.isHorn(formula)
	if err != nil {
		return nil, err
	}
	if horn {
		formulaRef, err := m.NewRef(formula)
		if err != nil {
			return nil, err
		}
		s.result.Formulas = append(s.result.Formulas, formulaRef)
		proofRef, err := m.NewRef(proof)
		if err != nil {
			return nil, err
		}
		s.result.Proofs = append(s.result.Proofs, proofRef)
		return s.result, n.traceHornResult(formula, s.result, true)
	}
	if err = n.Reset(); err != nil {
		return nil, err
	}
	hoister := n.hoister
	s.work = []hornProofWork{{formula, proof}}
	defer func() {
		for _, item := range s.work {
			resultErr = errors.Join(resultErr, n.pin(&n.todoRefs, item.formula), n.pin(&n.todoRefs, item.proof))
		}
	}()
	for len(s.work) != 0 {
		canceled, err := m.Canceled()
		if err != nil {
			return nil, err
		}
		if canceled {
			break
		}
		last := len(s.work) - 1
		item := s.work[last]
		s.work = s.work[:last]
		if err = s.step(item, hoister); err != nil {
			return nil, err
		}
	}
	return s.result, n.traceHornResult(formula, s.result, false)
}
