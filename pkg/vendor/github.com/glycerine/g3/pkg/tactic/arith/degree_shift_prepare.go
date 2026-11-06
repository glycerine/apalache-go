package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, discard_non_candidates
// and prepare_substitution.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type degreeShiftEntry struct {
	original, fresh, proof ast.ASTRef
	degree                 *big.Int
}
type degreeShiftPlan struct {
	manager *ast.ASTManager
	entries []degreeShiftEntry
}

func (p *degreeShiftPlan) Close() error {
	if p == nil || p.manager == nil {
		return nil
	}
	for i := range p.entries {
		for _, ref := range []*ast.ASTRef{&p.entries[i].proof, &p.entries[i].fresh, &p.entries[i].original} {
			if *ref != (ast.ASTRef{}) {
				if err := p.manager.DelRef(ref); err != nil {
					return err
				}
			}
		}
	}
	p.entries = nil
	p.manager = nil
	return nil
}
func degreeShiftDefinitionProof(m *ast.ASTManager, original, fresh ast.ASTHandle, degree *big.Int) (result ast.ASTRef, err error) {
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	pin := func(h ast.ASTHandle) error {
		r, e := m.NewRef(h)
		if e == nil {
			refs = append(refs, r)
		}
		return e
	}
	exponent, err := m.ArithmeticRealNumeralText(degree.String())
	if err != nil {
		return result, err
	}
	if err = pin(exponent); err != nil {
		return result, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return result, err
	}
	power, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{original, exponent})
	if err != nil {
		return result, err
	}
	if err = pin(power); err != nil {
		return result, err
	}
	equation, err := m.MkEq(fresh, power)
	if err != nil {
		return result, err
	}
	if err = pin(equation); err != nil {
		return result, err
	}
	intro, err := m.MkProofDefIntro(equation)
	if err != nil {
		return result, err
	}
	if err = pin(intro); err != nil {
		return result, err
	}
	proof, err := m.MkProofApplyDef(fresh, power, intro)
	if err != nil {
		return result, err
	}
	return m.NewRef(proof)
}
func degreeShiftPrepare(m *ast.ASTManager, roots []ast.ASTHandle, proofs bool) (plan *degreeShiftPlan, err error) {
	return degreeShiftPrepareWithCheckpoint(m, roots, proofs, nil)
}

func degreeShiftPrepareWithCheckpoint(m *ast.ASTManager, roots []ast.ASTHandle, proofs bool, checkpoint func() error) (plan *degreeShiftPlan, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	degrees, err := degreeShiftCandidatesWithCheckpoint(m, roots, checkpoint)
	if err != nil {
		return nil, err
	}
	type item struct {
		h  ast.ASTHandle
		id uint32
	}
	items := make([]item, 0, len(degrees))
	for h := range degrees {
		id, e := m.GetASTID(h)
		if e != nil {
			return nil, e
		}
		items = append(items, item{h, id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })
	plan = &degreeShiftPlan{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, plan.Close())
			plan = nil
		}
	}()
	for _, entry := range items {
		original, e := m.NewRef(entry.h)
		if e != nil {
			return plan, e
		}
		plan.entries = append(plan.entries, degreeShiftEntry{original: original, degree: new(big.Int).Set(degrees[entry.h])})
		current := &plan.entries[len(plan.entries)-1]
		sort, e := m.GetSort(entry.h)
		if e != nil {
			return plan, e
		}
		fresh, e := m.MkFreshConstNullable(nil, sort)
		if e != nil {
			return plan, e
		}
		current.fresh, e = m.NewRef(fresh)
		if e != nil {
			return plan, e
		}
		if proofs {
			current.proof, e = degreeShiftDefinitionProof(m, entry.h, fresh, current.degree)
			if e != nil {
				return plan, e
			}
		}
	}
	return plan, nil
}
