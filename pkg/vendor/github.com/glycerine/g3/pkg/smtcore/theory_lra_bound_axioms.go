package smtcore

// Source: src/smt/theory_lra.cpp:1487-1731. Concrete rational LRA bounds.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *SMTLRATheory) mkBoundAxioms(bound *smtLRABound) error {
	if !t.state.searching {
		t.newBounds = append(t.newBounds, bound)
		return nil
	}
	// Native publication order: lower below, lower above, upper below, upper above.
	var neighbors [4]*smtLRABound
	for _, other := range t.bounds[bound.variable] {
		if other == bound || other.boolean == bound.boolean {
			continue
		}
		cmp := other.value.Cmp(&bound.value)
		if cmp == 0 && other.upper == bound.upper {
			continue
		}
		if d, enabled, err := lraAssertionContext(); err != nil {
			return err
		} else if enabled && !(cmp != 0 || other.upper != bound.upper) {
			return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1517, "k1 != k2 || kind1 != kind2")
		}
		slot := 0
		if other.upper {
			slot = 2
		}
		if cmp >= 0 {
			slot++
		}
		old := neighbors[slot]
		if old == nil || cmp < 0 && other.value.Cmp(&old.value) > 0 || cmp >= 0 && other.value.Cmp(&old.value) < 0 {
			neighbors[slot] = other
		}
	}
	for _, other := range neighbors {
		if other != nil {
			if err := t.mkBoundAxiom(bound, other); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTLRATheory) mkBoundAxiom(a, b *smtLRABound) error {
	if d, enabled, err := lraAssertionContext(); err != nil {
		return err
	} else if enabled && a.variable != b.variable {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1553, "v == b2.get_var()")
	}
	if a.variable != b.variable {
		return ast.ErrASTDomain
	}
	cmp := a.value.Cmp(&b.value)
	if cmp == 0 && a.upper == b.upper {
		return nil
	}
	if d, enabled, err := lraAssertionContext(); err != nil {
		return err
	} else if enabled && !(cmp != 0 || a.upper != b.upper) {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1555, "k1 != k2 || kind1 != kind2")
	}
	node, err := t.Enode(a.variable)
	if err != nil {
		return err
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return err
	}
	l1, l2 := NewSMTLiteral(a.boolean, false), NewSMTLiteral(b.boolean, false)
	var clauses [][2]SMTLiteral
	if !a.upper && !b.upper {
		if cmp >= 0 {
			clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2})
		} else {
			clauses = append(clauses, [2]SMTLiteral{l1, l2.Negated()})
		}
	} else if a.upper && b.upper {
		if cmp >= 0 {
			clauses = append(clauses, [2]SMTLiteral{l1, l2.Negated()})
		} else {
			clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2})
		}
	} else {
		lower, upper := a, b
		if a.upper {
			lower, upper = b, a
		}
		if lower.value.Cmp(&upper.value) <= 0 {
			clauses = append(clauses, [2]SMTLiteral{l1, l2})
		} else {
			clauses = append(clauses, [2]SMTLiteral{l1.Negated(), l2.Negated()})
			if kind == 1 && new(big.Rat).Sub(&lower.value, &upper.value).Cmp(big.NewRat(1, 1)) == 0 {
				clauses = append(clauses, [2]SMTLiteral{l1, l2})
			}
		}
	}
	for _, clause := range clauses {
		if err := t.publishBoundAxiom(clause[:]); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTLRATheory) publishBoundAxiom(literals []SMTLiteral) (result error) {
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
	if err = t.emitLRA("mk_clause", 440, func(out io.Writer) error {
		if err := t.state.DisplayLiteralsVerbose(out, literals); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	return t.state.addTheoryAxiomWithParameters(t.FamilyID(), literals, parameters)
}

func (t *SMTLRATheory) flushBoundAxioms() error {
	if len(t.newBounds) > 0 {
		if err := t.emitLRA("flush_bound_axioms", 1612, func(out io.Writer) error { return smtWriteTheoryText(out, "flush bound axioms\n") }); err != nil {
			return err
		}
	}
	for len(t.newBounds) > 0 {
		last := len(t.newBounds) - 1
		batch := []*smtLRABound{t.newBounds[last]}
		t.newBounds = t.newBounds[:last]
		variable := batch[0].variable
		for i := 0; i < len(t.newBounds); {
			if t.newBounds[i].variable != variable {
				i++
				continue
			}
			batch = append(batch, t.newBounds[i])
			last = len(t.newBounds) - 1
			t.newBounds[i] = t.newBounds[last]
			t.newBounds = t.newBounds[:last]
		}
		if err := t.emitLRATag("arith_verbose", "flush_bound_axioms", 1627, func(out io.Writer) error {
			for _, b := range batch {
				kind := "<="
				if b.upper {
					kind = ">="
				}
				if err := smtWriteTheoryText(out, fmt.Sprintf("v%d  %s %s\n", b.variable, kind, b.value.RatString())); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.newBounds = append(t.newBounds, batch...)
			return err
		}
		occurrences := append([]*smtLRABound(nil), t.bounds[variable]...)
		// Native compares only thresholds. Stable Go order resolves equal keys;
		// no C++ standard-library sorting implementation is reproduced.
		sort.SliceStable(batch, func(i, j int) bool { return batch[i].value.Cmp(&batch[j].value) < 0 })
		sort.SliceStable(occurrences, func(i, j int) bool { return occurrences[i].value.Cmp(&occurrences[j].value) < 0 })
		end := len(occurrences)
		first := func(upper bool) int {
			for i, b := range occurrences {
				if b.upper == upper {
					return i
				}
			}
			return end
		}
		lo, hi := first(false), first(true)
		positions := [4]int{lo, lo, hi, hi}
		visited := make(map[*smtLRABound]bool)
		for _, a := range batch {
			visited[a] = true
			for slot := range positions {
				result, compatible := end, false
				for i := positions[slot]; i < end; i++ {
					b := occurrences[i]
					if b == a || b.upper != (slot >= 2) {
						continue
					}
					compatible = true
					cmp := b.value.Cmp(&a.value)
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
					if err := t.mkBoundAxiom(a, occurrences[positions[slot]]); err != nil {
						// Keep the batch available after a Go publication refusal.
						t.newBounds = append(t.newBounds, batch...)
						return err
					}
				}
			}
		}
	}
	return nil
}
