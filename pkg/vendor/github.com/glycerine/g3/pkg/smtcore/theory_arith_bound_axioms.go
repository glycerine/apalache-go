package smtcore

// Source: src/smt/theory_arith_core.h:959-1210 bound axioms and delayed flush.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// Axis identity replaces the native theory-variable occurrence vector. It is
// the actual left-hand AST, not an expanded polynomial or a scaled alias.
type smtArithmeticBoundAxiomAtom struct {
	axis      ast.ASTHandle
	threshold smtArithmeticValue
	kind      smtArithmeticAtomKind
	literal   SMTLiteral
	integral  bool
}

func (t *SMTLinearArithmeticTheory) registerOrdinaryBoundAxiom(index int) error {
	a := &t.atoms[index]
	if !a.supported {
		return nil
	}
	rhs := a.right
	op, own, err := t.operation(rhs)
	if err != nil {
		return err
	}
	if own && op == smtArithOpToReal {
		rhs, err = t.state.manager.GetArg(rhs, 0)
		if err != nil {
			return err
		}
	}
	r, numeral, err := t.numeral(rhs)
	if err != nil || !numeral {
		// Native arithmetic internalization expects a normalized numeral RHS.
		// The Go direct-internalizer extension also accepts general comparisons.
		return err
	}
	integer, err := t.isIntegerExpression(a.left)
	if err != nil {
		return err
	}
	b := &smtArithmeticBoundAxiomAtom{axis: a.left, literal: NewSMTLiteral(a.variable, false), integral: integer}
	switch a.op {
	case smtArithOpGE, smtArithOpLT:
		b.kind = smtArithmeticLowerAtom
	case smtArithOpLE, smtArithOpGT:
		b.kind = smtArithmeticUpperAtom
	default:
		return ast.ErrASTDomain
	}
	if a.op == smtArithOpLT || a.op == smtArithOpGT {
		b.literal = b.literal.Negated() // Native strict atoms are negated >=/<=.
	}
	if integer && !r.IsInt() {
		n := floorSMTRational(r)
		if b.kind == smtArithmeticLowerAtom {
			n.Add(n, big.NewInt(1))
		}
		r.SetInt(n)
	}
	b.threshold.rational.Set(r)
	a.boundAxiom = b
	if err := t.addBoundAxioms(index, b); err != nil {
		t.newBoundAtoms = append(t.newBoundAtoms, index)
		return err
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) addBoundAxioms(index int, a *smtArithmeticBoundAxiomAtom) error {
	if !t.state.searching {
		t.newBoundAtoms = append(t.newBoundAtoms, index)
		return nil
	}
	// lo_inf,lo_sup,hi_inf,hi_sup, in native publication order.
	var neighbors [4]*smtArithmeticBoundAxiomAtom
	for i := range t.atoms {
		b := t.atoms[i].boundAxiom
		if i == index || b == nil || b.axis != a.axis {
			continue
		}
		cmp := b.threshold.cmp(&a.threshold)
		if cmp == 0 && b.kind == a.kind {
			continue
		}
		slot := 0
		if b.kind == smtArithmeticUpperAtom {
			slot = 2
		}
		if cmp >= 0 {
			slot++
		}
		old := neighbors[slot]
		if old == nil || cmp < 0 && b.threshold.cmp(&old.threshold) > 0 || cmp >= 0 && b.threshold.cmp(&old.threshold) < 0 {
			neighbors[slot] = b
		}
	}
	for _, b := range neighbors {
		if b != nil {
			if err := t.makeBoundAxiom(a, b); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) makeBoundAxiom(a, b *smtArithmeticBoundAxiomAtom) error {
	cmp := a.threshold.cmp(&b.threshold)
	if cmp == 0 && a.kind == b.kind {
		return nil
	}
	l1, l2 := a.literal, b.literal
	var clauses [][2]SMTLiteral
	if a.kind == smtArithmeticLowerAtom {
		if b.kind == smtArithmeticLowerAtom {
			if cmp >= 0 {
				clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2})
			} else {
				clauses = append(clauses, [2]SMTLiteral{l1, l2.Negated()})
			}
		} else if cmp <= 0 {
			clauses = append(clauses, [2]SMTLiteral{l1, l2})
		} else {
			clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2.Negated()})
			var gap smtArithmeticValue
			gap.set(&a.threshold)
			gap.rational.Sub(&a.threshold.rational, big.NewRat(1, 1))
			if a.integral && gap.cmp(&b.threshold) == 0 {
				clauses = append(clauses, [2]SMTLiteral{l1, l2})
			}
		}
	} else if b.kind == smtArithmeticLowerAtom {
		if cmp >= 0 {
			clauses = append(clauses, [2]SMTLiteral{l1, l2})
		} else {
			clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2.Negated()})
			var gap smtArithmeticValue
			gap.set(&a.threshold)
			gap.rational.Add(&a.threshold.rational, big.NewRat(1, 1))
			if a.integral && gap.cmp(&b.threshold) == 0 {
				clauses = append(clauses, [2]SMTLiteral{l1, l2})
			}
		}
	} else if cmp >= 0 {
		clauses = append(clauses, [2]SMTLiteral{l1, l2.Negated()})
	} else {
		clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2})
	}
	for _, clause := range clauses {
		if err := t.publishBoundAxiom(clause[:]); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) publishBoundAxiom(literals []SMTLiteral) (result error) {
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	var parameters []ast.ASTParameter
	if proofs {
		antecedents := smtArithmeticAntecedents{literalCoefficients: []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}
		parameters, err = antecedents.parameters(t.state.manager, "farkas")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(t.state.manager, parameters)) }()
	}
	return t.state.addTheoryAxiomWithParameters(t.FamilyID(), literals, parameters)
}

func (t *SMTLinearArithmeticTheory) flushBoundAxioms() error {
	for len(t.newBoundAtoms) != 0 {
		last := len(t.newBoundAtoms) - 1
		batch := []int{t.newBoundAtoms[last]}
		t.newBoundAtoms = t.newBoundAtoms[:last]
		axis := t.atoms[batch[0]].boundAxiom.axis
		for i := 0; i < len(t.newBoundAtoms); {
			index := t.newBoundAtoms[i]
			if t.atoms[index].boundAxiom.axis != axis {
				i++
				continue
			}
			batch = append(batch, index)
			last = len(t.newBoundAtoms) - 1
			t.newBoundAtoms[i] = t.newBoundAtoms[last]
			t.newBoundAtoms = t.newBoundAtoms[:last]
		}
		var occurrences []int
		for i := range t.atoms {
			if b := t.atoms[i].boundAxiom; b != nil && b.axis == axis {
				occurrences = append(occurrences, i)
			}
		}
		less := func(a, b int) bool { return t.atoms[a].boundAxiom.threshold.cmp(&t.atoms[b].boundAxiom.threshold) < 0 }
		// Native sorts only by threshold. Stable Go order resolves equal keys.
		sort.SliceStable(batch, func(i, j int) bool { return less(batch[i], batch[j]) })
		sort.SliceStable(occurrences, func(i, j int) bool { return less(occurrences[i], occurrences[j]) })
		end := len(occurrences)
		first := func(kind smtArithmeticAtomKind) int {
			for i, index := range occurrences {
				if t.atoms[index].boundAxiom.kind == kind {
					return i
				}
			}
			return end
		}
		lo, hi := first(smtArithmeticLowerAtom), first(smtArithmeticUpperAtom)
		positions := [4]int{lo, lo, hi, hi}
		visited := make(map[int]bool)
		for _, index := range batch {
			a := t.atoms[index].boundAxiom
			visited[index] = true
			for slot := range positions {
				kind := smtArithmeticLowerAtom
				if slot >= 2 {
					kind = smtArithmeticUpperAtom
				}
				result, compatible := end, false
				for i := positions[slot]; i < end; i++ {
					other := occurrences[i]
					b := t.atoms[other].boundAxiom
					if other == index || b.kind != kind {
						continue
					}
					compatible = true
					cmp := b.threshold.cmp(&a.threshold)
					if slot%2 == 0 {
						if cmp <= 0 {
							result = i
						} else {
							break
						}
					} else if cmp > 0 {
						result = i
						break
					}
				}
				if result != end {
					positions[slot] = result
				}
				if !compatible {
					positions[slot] = end
				}
				if result != end && positions[slot] != end && !visited[occurrences[positions[slot]]] {
					if err := t.makeBoundAxiom(a, t.atoms[occurrences[positions[slot]]].boundAxiom); err != nil {
						// Keep the reached batch available after a Go allocation
						// refusal. Already emitted clauses may be repeated safely.
						t.newBoundAtoms = append(t.newBoundAtoms, batch...)
						return err
					}
				}
			}
		}
	}
	return nil
}
