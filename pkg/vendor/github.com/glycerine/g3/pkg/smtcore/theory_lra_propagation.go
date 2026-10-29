package smtcore

// Source: src/smt/theory_lra.cpp:1303-1485,1733-1811. Live basic and LP propagation.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTLRATheory) setLRAConflict() error {
	evidence, err := t.numeric.infeasibilityEvidence()
	if err != nil {
		return err
	}
	t.conflictCount++ // Search-wide count: restored by init_search, not by pop.
	t.statistics.conflicts++
	if err = t.emitLRA("set_conflict", 2222, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("scope: %d\n", t.state.scopeLevel)); err != nil {
			return err
		}
		return t.displayEvidence(out, evidence)
	}); err != nil {
		return err
	}
	if err = t.emitLRA("set_conflict", 2223, t.Display); err != nil {
		return err
	}
	hook := smtLRABoundPropagation{owner: t}
	for _, e := range evidence {
		if e.coefficient.Sign() != 0 {
			if err := hook.Consume(nil, e.constraint); err != nil {
				return err
			}
		}
	}
	core := hook.core
	if err := t.checkValidation(smtLRAValidateConflict, core, SMTFalseLiteral, nil, nil); err != nil {
		return err
	}
	s := t.state
	if s.inconsistent {
		return nil
	}
	// Source set_conflict always allocates the extended justification,
	// including the immediate path with an empty equality-premise vector.
	owner, err := NewSMTExtTheoryConflictJustification(s.manager, t.FamilyID(), core, nil, nil)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(s.manager))
	}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	s.inconsistent = true
	s.conflict = wrapped
	s.conflictLiteral = SMTNullLiteral
	s.statistics.NumConflicts++
	return nil
}

func (t *SMTLRATheory) impliedBoundLiteral(kind lp.LConstraintKind, value *big.Rat, b *smtLRABound) SMTLiteral {
	cmp := value.Cmp(&b.value)
	if (kind == lp.LConstraintLE || kind == lp.LConstraintLT) && b.upper && cmp <= 0 ||
		(kind == lp.LConstraintGE || kind == lp.LConstraintGT) && !b.upper && cmp >= 0 {
		return NewSMTLiteral(b.boolean, false)
	}
	if !b.upper && (kind == lp.LConstraintLE && cmp < 0 || kind == lp.LConstraintLT && cmp <= 0) ||
		b.upper && (kind == lp.LConstraintGE && cmp > 0 || kind == lp.LConstraintGT && cmp >= 0) {
		return NewSMTLiteral(b.boolean, true)
	}
	return SMTNullLiteral
}

// Numerical column and term indices occupy different source directories.
func (t *SMTLRATheory) variableForLPIndex(index uint32) SMTTheoryVar {
	vars := t.numeric.variables
	if t.numeric.solver.IsTerm(index) {
		vars = t.numeric.terms
		index -= lp.LARTermStartIndex
	}
	if uint64(index) >= uint64(len(vars)) {
		return SMTNullTheoryVar
	}
	return vars[index]
}

type smtLRABoundPropagation struct {
	owner *SMTLRATheory
	core  []SMTLiteral
}

func (p *smtLRABoundPropagation) BoundIsInteresting(index uint32, kind lp.LConstraintKind, value *numeral.Rational) (bool, error) {
	t := p.owner
	v := t.variableForLPIndex(index)
	if v == SMTNullTheoryVar {
		return false, nil
	}
	if int(v) >= len(t.bounds) || t.unassigned[v] == 0 {
		err := t.emitLRA("bound_is_interesting", 1339, func(out io.Writer) error {
			return smtWriteTheoryText(out, "return\n")
		})
		return false, err
	}
	r, err := t.numeric.rational(value)
	if err != nil {
		return false, err
	}
	for _, bound := range t.bounds[v] {
		assignment, err := t.state.Assignment(NewSMTLiteral(bound.boolean, false))
		if err != nil {
			return false, err
		}
		if assignment == lbool.Undef && t.impliedBoundLiteral(kind, r, bound) != SMTNullLiteral {
			return true, nil
		}
	}
	return false, nil
}

func (p *smtLRABoundPropagation) Consume(_ *numeral.Rational, witness uint32) error {
	// Native local_bound_propagator intentionally ignores the coefficient.
	// Definitions are hard constraints; immediate LRA has only inequality sources.
	if witness == smtLRANoIndex {
		return nil
	}
	s := p.owner.numeric
	if uint64(witness) >= uint64(len(s.inequalities)) {
		return ast.ErrASTDomain
	}
	if d, enabled, err := lraAssertionContext(); err != nil {
		return err
	} else if enabled && s.inequalities[witness] == SMTNullLiteral {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 2187, "lit != null_literal")
	}
	if s.inequalities[witness] == SMTNullLiteral {
		return ast.ErrASTDomain
	}
	p.core = append(p.core, s.inequalities[witness])
	return nil
}

func (t *SMTLRATheory) propagationEnabled() bool {
	return t.config.BoundProp != 0 && t.conflictCount < t.config.PropagationThreshold
}

func (t *SMTLRATheory) propagateLPBounds() (err error) {
	if !t.propagationEnabled() {
		return nil
	}
	defer func() {
		if err != nil {
			t.numeric.stopped = true
		}
	}()
	previousImplied := t.numeric.solver.Settings.Stats.ImpliedBounds
	hook := &smtLRABoundPropagation{owner: t}
	t.propagator = t.numeric.solver.NewBoundPropagator()
	t.propagator.Interest, t.propagator.Evidence = hook, hook
	if err = t.numeric.solver.PropagateBoundsForTouchedRows(t.propagator); err != nil {
		return err
	}
	canceled, err := t.state.manager.Canceled()
	if err != nil {
		return err
	}
	if !canceled {
		currentImplied := t.numeric.solver.Settings.Stats.ImpliedBounds
		if currentImplied > previousImplied {
			if err = t.emitLRA("propagate_bounds_with_lp_solver", 1316, func(out io.Writer) error {
				return smtWriteTheoryText(out, fmt.Sprintf("found %d implied bounds\n", currentImplied))
			}); err != nil {
				return err
			}
		}
		if t.numeric.solver.Bounds.Status.Value == lp.StatusInfeasible {
			if err = t.setLRAConflict(); err != nil {
				return err
			}
		} else {
			for _, implied := range t.propagator.FoundBounds {
				canceled, err = t.state.manager.Canceled()
				if err != nil {
					return err
				}
				if canceled || t.state.inconsistent {
					break
				}
				v := t.variableForLPIndex(implied.Column)
				if v == SMTNullTheoryVar {
					continue
				}
				if err = t.emitLRA("propagate_lp_solver_bound", 1383, func(out io.Writer) error {
					kind, err := lp.LConstraintKindString(implied.Kind())
					if err != nil {
						return err
					}
					r, err := t.numeric.rational(&implied.Bound)
					if err != nil {
						return err
					}
					return smtWriteTheoryText(out, fmt.Sprintf("v%d %s %s\n", v, kind, r.RatString()))
				}); err != nil {
					return err
				}
				if int(v) >= len(t.bounds) || t.unassigned[v] == 0 {
					if err = t.emitLRA("propagate_lp_solver_bound", 1389, func(out io.Writer) error { return smtWriteTheoryText(out, "return\n") }); err != nil {
						return err
					}
					continue
				}
				r, err := t.numeric.rational(&implied.Bound)
				if err != nil {
					return err
				}
				first := true
				for _, bound := range t.bounds[v] {
					assignment, err := t.state.Assignment(NewSMTLiteral(bound.boolean, false))
					if err != nil {
						return err
					}
					if assignment != lbool.Undef {
						continue
					}
					literal := t.impliedBoundLiteral(implied.Kind(), r, bound)
					if literal == SMTNullLiteral {
						continue
					}
					t.numeric.solver.Settings.Stats.ImpliedBounds++
					if first {
						first = false
						hook.core = nil
						if err = t.numeric.solver.ExplainImpliedBound(implied, t.propagator); err != nil {
							return err
						}
					}
					if t.unassigned[v] == 0 {
						if err = t.emitLRA("propagate_lp_solver_bound", 1413, func(out io.Writer) error { return smtWriteTheoryText(out, "missed bound\n") }); err != nil {
							return err
						}
					}
					if err = t.updateUnassignedBounds(v, -1); err != nil {
						return err
					}
					if err = t.emitLRA("propagate_lp_solver_bound", 1415, func(out io.Writer) error {
						if err := t.state.DisplayLiteralsVerbose(out, hook.core); err != nil {
							return err
						}
						if err := smtWriteTheoryText(out, "\n --> "); err != nil {
							return err
						}
						if err := t.state.DisplayLiteralVerbose(out, literal); err != nil {
							return err
						}
						if err := smtWriteTheoryText(out, "\n"); err != nil {
							return err
						}
						return t.numeric.solver.PrintImpliedBound(implied, out)
					}); err != nil {
						return err
					}
					if err = t.checkLRAPropagationCore(hook.core); err != nil {
						return err
					}
					t.lpPropagations++
					if err = t.assignLRABound(literal, hook.core, nil); err != nil {
						return err
					}
				}
			}
		}
	}
	if err = t.propagator.Close(); err != nil {
		return err
	}
	t.propagator = nil
	return nil
}

func (t *SMTLRATheory) propagateBasicBounds() error {
	for _, bv := range t.boundChecks {
		b := t.boundByBoolean[bv]
		if b == nil {
			return ast.ErrASTDomain
		}
		assignment, err := t.state.Assignment(NewSMTLiteral(bv, false))
		if err != nil {
			return err
		}
		if err = t.propagateBasicBound(b, assignment == lbool.True); err != nil {
			return err
		}
		if t.state.inconsistent {
			break
		}
	}
	t.boundChecks = nil
	return nil
}

func (t *SMTLRATheory) propagateBasicBound(b *smtLRABound, truth bool) (result error) {
	if !t.propagationEnabled() {
		return nil
	}
	if d, enabled, err := lraAssertionContext(); err != nil {
		return err
	} else if enabled && len(t.bounds[b.variable]) == 0 {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1758, "!bounds.empty()")
	}
	if len(t.bounds[b.variable]) == 1 || t.unassigned[b.variable] == 0 {
		return nil
	}
	findLower := truth != b.upper
	var nearest *smtLRABound
	for _, other := range t.bounds[b.variable] {
		if other == b {
			continue
		}
		cmp := other.value.Cmp(&b.value)
		// False bounds have +/-epsilon; comparing a rational against that
		// infinitesimal reproduces these strict/equal threshold cases.
		eligible := cmp < 0 || !truth && cmp == 0
		if !findLower {
			eligible = cmp > 0 || !truth && cmp == 0
		}
		if !eligible {
			continue
		}
		if nearest == nil || findLower && other.value.Cmp(&nearest.value) > 0 || !findLower && other.value.Cmp(&nearest.value) < 0 {
			nearest = other
		}
	}
	if nearest == nil {
		return nil
	}
	literal := NewSMTLiteral(nearest.boolean, nearest.upper == findLower)
	if err := t.emitLRA("propagate_bound", 1797, func(out io.Writer) error {
		if err := t.state.DisplayLiteralVerbose(out, NewSMTLiteral(b.boolean, !truth)); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " => "); err != nil {
			return err
		}
		if err := t.state.DisplayLiteralVerbose(out, literal); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if err := t.updateUnassignedBounds(b.variable, -1); err != nil {
		return err
	}
	t.basicPropagations++
	// Source m_params is populated independently of proof production. The
	// external-justification branch owns its copy even when proofs are off.
	antecedents := smtArithmeticAntecedents{literalCoefficients: []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}
	parameters, err := antecedents.parameters(t.state.manager, "farkas")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(t.state.manager, parameters)) }()

	// Preserve the source quirk: m_core contains lit2, not lit1. In the
	// small-lemma path this creates a tautology, not a new implication.
	if err = t.assignLRABound(literal, []SMTLiteral{literal}, parameters); err != nil {
		return err
	}
	t.statistics.boundsPropagations++
	return nil
}

func (t *SMTLRATheory) assignLRABound(literal SMTLiteral, core []SMTLiteral, parameters []ast.ASTParameter) error {
	if err := t.checkValidation(smtLRAValidateAssignment, core, literal, nil, nil); err != nil {
		return err
	}
	s := t.state
	if uint64(len(core)) < uint64(t.config.SmallLemmaSize) {
		clause := make([]SMTLiteral, 0, len(core)+1)
		for _, premise := range core {
			clause = append(clause, premise.Negated())
		}
		clause = append(clause, literal)
		var justification *SMTTheoryLemmaJustification
		proofs, err := s.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs {
			justification, err = NewSMTTheoryLemmaJustification(s.manager, t.FamilyID(), clause, s.boolExpr, parameters)
			if err != nil {
				return err
			}
		}
		var owner SMTJustification
		if justification != nil {
			owner = justification
		}
		beforeClauses, beforeOwners := len(s.clauses), len(s.ownedJustifications)
		err = s.addClauseKindWithJustificationAndDeleteHandler(clause, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
		if justification != nil && len(s.clauses) == beforeClauses && len(s.ownedJustifications) == beforeOwners {
			return errors.Join(err, justification.Release(s.manager))
		}
		return err
	}
	owner, err := NewSMTExtTheoryPropagationJustification(s.manager, t.FamilyID(), core, nil, literal, parameters)
	if err != nil {
		return err
	}
	justification, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(s.manager))
	}
	if err = s.AssignLiteral(literal, justification); err != nil {
		return errors.Join(err, owner.Release(s.manager))
	}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	return nil
}

// Source DEBUG_CODE/SASSERT at theory_lra.cpp:1424-1425. Neither the
// assignment lookups nor their failure paths exist in a native build.
func (t *SMTLRATheory) checkLRAPropagationCore(core []SMTLiteral) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	for _, literal := range core {
		value, err := t.state.Assignment(literal)
		if err != nil {
			return err
		}
		if value != lbool.True {
			return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 1425, "ctx().get_assignment(lit) == l_true")
		}
	}
	return nil
}
