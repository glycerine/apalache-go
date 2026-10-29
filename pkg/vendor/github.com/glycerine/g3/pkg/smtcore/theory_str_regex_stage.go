package smtcore

// Source: src/smt/theory_str.cpp final_check_eh regex automata stage.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtStringRegexStage struct {
	theory *SMTStringTheory
	locals smtStringLengthPremises
	added  bool
}

func (f *smtStringRegexStage) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	return value, f.locals.add(value)
}

func (f *smtStringRegexStage) number(value *big.Int) (ast.ASTHandle, error) {
	return f.pin(f.theory.state.manager.ArithmeticIntegerNumeralText(value.String()))
}

func (f *smtStringRegexStage) publish(expression ast.ASTHandle) error {
	if err := f.theory.assertAxiom(expression); err != nil {
		return err
	}
	f.added = true
	return nil
}

func (f *smtStringRegexStage) imply(guard, consequence ast.ASTHandle) error {
	p := smtStringLengthPremises{theory: f.theory, values: []ast.ASTHandle{guard}}
	if err := p.imply(consequence); err != nil {
		return err
	}
	f.added = true
	return nil
}

func (f *smtStringRegexStage) assignment(member ast.ASTHandle) (lbool.LBool, error) {
	variable, exists := f.theory.state.BoolVar(member)
	if !exists {
		return lbool.Undef, nil
	}
	return f.theory.state.Assignment(NewSMTLiteral(variable, false))
}

func stringRegexUnsignedLength(value *big.Int) (uint32, error) {
	if value == nil || value.Sign() < 0 || !value.IsUint64() || value.Uint64() > uint64(^uint32(0)) {
		return 0, ast.ErrASTDomain
	}
	return uint32(value.Uint64()), nil
}

func (f *smtStringRegexStage) registerLengths(member, word, regex ast.ASTHandle, assignment lbool.LBool) (result error) {
	t, m := f.theory, f.theory.state.manager
	if _, done := t.regexStage.lengthDone[member]; done || assignment != lbool.True {
		return nil
	}
	linear, err := t.checkRegexLengthLinearity(regex, false)
	if err != nil || !linear {
		return err
	}
	formula, cached := t.regexStage.lengthFormulas[member]
	variables := t.regexStage.lengthVariables[member]
	if !cached {
		length, err := f.pin(m.MkSequenceLength(word))
		if err != nil {
			return err
		}
		root, err := t.inferStringRegexLengths(length, regex, &variables)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, m.DelRef(&root)) }()
		formula, err = m.RefGet(root)
		if err != nil {
			return err
		}
		if t.state.theoryRewriter == nil {
			return ast.ErrASTHandle
		}
		rw, err := t.state.theoryRewriter.FreshDefault()
		if rw != nil {
			defer func() { result = errors.Join(result, rw.Close()) }()
		}
		if err != nil {
			return err
		}
		if err = rw.Rewrite(formula, root); err != nil {
			return err
		}
		formula, err = m.RefGet(root)
		if err != nil {
			return err
		}
		if err = f.locals.add(formula); err != nil {
			return err
		}
	}
	if err = f.publish(formula); err != nil {
		return err
	}
	for _, variable := range variables {
		if cached {
			if err = t.refreshStringVariable(variable); err != nil {
				return err
			}
		}
		zero, err := f.number(new(big.Int))
		if err != nil {
			return err
		}
		bound, err := f.pin(m.MkArithmeticGE(variable, zero))
		if err != nil {
			return err
		}
		if err = f.publish(bound); err != nil {
			return err
		}
	}
	if !cached {
		t.regexStage.lengthFormulas[member] = formula
		t.regexStage.lengthVariables[member] = append([]ast.ASTHandle(nil), variables...)
	}
	t.regexStage.lengthDone[member] = t.level
	return nil
}

func (f *smtStringRegexStage) construct(member, regex ast.ASTHandle, options SMTStrState, readFailureFirst bool) error {
	t := f.theory
	complexity, err := t.estimateStringRegexComplexity(regex, false)
	if err != nil {
		return err
	}
	allowed := complexity <= options.RegexAutomataDifficultyThreshold
	if readFailureFirst {
		failures := stringRegexCounter(t.regexStage.failures, member)
		allowed = allowed || failures >= options.RegexAutomataFailedAutomatonThreshold
	} else if !allowed {
		allowed = stringRegexCounter(t.regexStage.failures, member) >= options.RegexAutomataFailedAutomatonThreshold
	}
	if !allowed {
		incrementStringRegexCounter(t.regexStage.failures, member)
		return nil
	}
	owner, err := rewriter.NewStringRegexAutomaton(t.state.manager, regex, t.regexExpressionSolver)
	if err != nil {
		return err
	}
	graph, err := owner.Graph()
	if err != nil {
		return errors.Join(err, owner.Close())
	}
	graph.Compress()
	t.regexStage.owned = append(t.regexStage.owned, owner)
	t.regexStage.automata[regex] = owner
	f.added = true
	return t.findStringAutomatonInitialBounds(member, graph)
}

func (f *smtStringRegexStage) exactGuard(member, word ast.ASTHandle, assignment lbool.LBool, value *big.Int) (ast.ASTHandle, error) {
	m := f.theory.state.manager
	polarity := member
	var err error
	if assignment == lbool.False {
		polarity, err = f.pin(m.MkNot(member))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	length, err := f.pin(m.MkSequenceLength(word))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	number, err := f.number(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equal, err := f.pin(m.MkEq(length, number))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.pin(m.MkAndArray([]ast.ASTHandle{polarity, equal}))
}

func (f *smtStringRegexStage) exactPath(member, word ast.ASTHandle, assignment lbool.LBool, owner *rewriter.StringRegexAutomaton, value *big.Int) (result error) {
	t, m := f.theory, f.theory.state.manager
	if value.Sign() == 0 {
		graph, err := owner.Graph()
		if err != nil {
			return err
		}
		accepts := stringAutomatonFrontierAccepts(graph, graph.EpsilonClosure(graph.Initial()))
		if assignment == lbool.False {
			accepts = !accepts
		} // All retained cached automata have positive polarity.
		guard, err := f.exactGuard(member, word, assignment, value)
		if err != nil {
			return err
		}
		if !accepts {
			conflict, err := f.pin(m.MkNot(guard))
			if err != nil {
				return err
			}
			return f.publish(conflict)
		}
		empty, err := f.pin(t.stringLiteral(""))
		if err != nil {
			return err
		}
		rhs, err := f.pin(m.MkEq(word, empty))
		if err != nil {
			return err
		}
		if err = f.imply(guard, rhs); err != nil {
			return err
		}
		t.regexStage.pathDone[member] = t.level
		return nil
	}
	unsigned, err := stringRegexUnsignedLength(value)
	if err != nil {
		return err
	}
	characters, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&characters)) }()
	path, err := t.generateStringRegexPathConstraints(word, owner, unsigned, characters)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&path)) }()
	pathValue, err := m.RefGet(path)
	if err != nil {
		return err
	}
	characterValue, err := m.RefGet(characters)
	if err != nil {
		return err
	}
	guard, err := f.exactGuard(member, word, assignment, value)
	if err != nil {
		return err
	}
	if m.IsFalse(pathValue) {
		if assignment == lbool.True {
			conflict, err := f.pin(m.MkNot(guard))
			if err != nil {
				return err
			}
			return f.publish(conflict)
		}
		if err = f.imply(guard, characterValue); err != nil {
			return err
		}
	} else {
		if assignment == lbool.False {
			pathValue, err = f.pin(m.MkNot(pathValue))
			if err != nil {
				return err
			}
		}
		rhs, err := f.pin(m.MkAnd(pathValue, characterValue))
		if err != nil {
			return err
		}
		if err = f.imply(guard, rhs); err != nil {
			return err
		}
	}
	t.regexStage.pathDone[member] = t.level
	return nil
}

func (f *smtStringRegexStage) refineBound(member, length ast.ASTHandle, owner *rewriter.StringRegexAutomaton, value *big.Int, upper bool) error {
	m := f.theory.state.manager
	unsigned, err := stringRegexUnsignedLength(value)
	if err != nil {
		return err
	}
	graph, err := owner.Graph()
	if err != nil {
		return err
	}
	var at bool
	var next int64
	if upper {
		at, next, err = refineStringAutomatonUpperBound(graph, unsigned)
	} else {
		at, next, err = refineStringAutomatonLowerBound(graph, unsigned)
	}
	if err != nil {
		return err
	}
	number, err := f.number(value)
	if err != nil {
		return err
	}
	var bound ast.ASTHandle
	if upper {
		bound, err = f.pin(m.MkArithmeticLE(length, number))
	} else {
		bound, err = f.pin(m.MkArithmeticGE(length, number))
	}
	if err != nil {
		return err
	}
	guard, err := f.pin(m.MkAndArray([]ast.ASTHandle{member, bound}))
	if err != nil {
		return err
	}
	var rhs ast.ASTHandle
	if at {
		if next == -1 {
			rhs, err = f.pin(m.MkEq(length, number))
		} else {
			// The lower-bound equality-or-next case is explicitly disabled in
			// native code because it causes arithmetic nontermination.
			if !upper {
				return nil
			}
			previous, createErr := f.number(big.NewInt(next))
			if createErr != nil {
				return createErr
			}
			below, createErr := f.pin(m.MkArithmeticLE(length, previous))
			if createErr != nil {
				return createErr
			}
			exact, createErr := f.pin(m.MkEq(length, number))
			if createErr != nil {
				return createErr
			}
			rhs, err = f.pin(m.MkOr(exact, below))
		}
	} else if next == -1 {
		rhs, err = f.pin(m.MkNot(bound))
	} else {
		refined, createErr := f.number(big.NewInt(next))
		if createErr != nil {
			return createErr
		}
		if upper {
			rhs, err = f.pin(m.MkArithmeticLE(length, refined))
		} else {
			rhs, err = f.pin(m.MkArithmeticGE(length, refined))
		}
	}
	if err != nil {
		return err
	}
	return f.imply(guard, rhs)
}

func (t *SMTStringTheory) runStringRegexAutomataStage() (added bool, result error) {
	enabled, err := t.regexAutomataEnabled()
	if err != nil || !enabled {
		return false, err
	}
	options := SMTStrState{RegexAutomataDifficultyThreshold: 1000, RegexAutomataIntersectionDifficultyThreshold: 1000, RegexAutomataFailedAutomatonThreshold: 10, RegexAutomataFailedIntersectionThreshold: 10, RegexAutomataLengthAttemptThreshold: 10}
	if t.parameterOwner != nil {
		options, err = t.stringParameterState()
		if err != nil {
			return false, err
		}
	}
	f := smtStringRegexStage{theory: t, locals: smtStringLengthPremises{theory: t}}
	defer func() { added = f.added; result = errors.Join(result, f.locals.close()) }()
	m := t.state.manager
	var terms []ast.ASTHandle
	for term := range t.regexTerms {
		terms = append(terms, term)
	}
	stringSortHandles(terms)
	for _, member := range terms {
		word, regex, matched, err := m.SequenceBinaryOperationArguments(member, 12)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, ast.ErrASTDomain
		}
		assignment, err := f.assignment(member)
		if err != nil {
			return false, err
		}
		if assignment == lbool.Undef {
			continue
		}
		if err = f.registerLengths(member, word, regex, assignment); err != nil {
			return false, err
		}
		value, exact, err := t.lengthValue(word)
		if err != nil {
			return false, err
		}
		owner := t.regexStage.automata[regex]
		if exact {
			if _, done := t.regexStage.pathDone[member]; done {
				continue
			}
			if owner != nil {
				if err = f.exactPath(member, word, assignment, owner, value); err != nil {
					return false, err
				}
				incrementStringRegexCounter(t.regexStage.lengthAttempts, regex)
			} else if err = f.construct(member, regex, options, false); err != nil {
				return false, err
			}
			continue
		}
		length, err := f.pin(m.MkSequenceLength(word))
		if err != nil {
			return false, err
		}
		lower, hasLower, err := t.stringArithmeticBound(length, true)
		if err != nil {
			return false, err
		}
		upper, hasUpper, err := t.stringArithmeticBound(length, false)
		if err != nil {
			return false, err
		}
		if !hasLower {
			lower = new(big.Int)
		}
		if !hasUpper {
			upper = new(big.Int)
		} // Native unknown rationals still default to zero.
		lastLower, seenLower := t.regexLastLower[word]
		newLower := !seenLower || lastLower.Cmp(lower) != 0
		lastUpper, seenUpper := t.regexStage.lastUpper[word]
		newUpper := !seenUpper || lastUpper.Cmp(upper) != 0
		if newLower {
			t.regexLastLower[word] = new(big.Int).Set(lower)
		}
		if newUpper {
			t.regexStage.lastUpper[word] = new(big.Int).Set(upper)
		}
		if hasUpper && newUpper {
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
				continue
			}
			if assignment == lbool.True {
				if err = f.refineBound(member, length, owner, upper, true); err != nil {
					return false, err
				}
			}
		} else if hasLower && lower.Sign() != 0 && newLower {
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
				continue
			}
			if assignment == lbool.True {
				if err = f.refineBound(member, length, owner, lower, false); err != nil {
					return false, err
				}
			}
		} else {
			// This read happens even when an existing automaton makes the
			// following construction unnecessary; reads insert missing zero.
			stringRegexCounter(t.regexStage.failures, member)
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
			} else {
				incrementStringRegexCounter(t.regexStage.failures, member)
			}
		}
	}
	if err = f.intersections(options); err != nil {
		return false, err
	}
	// Native regex_axiom_add's FC_CONTINUE return is commented out. The
	// enclosing final checker must continue into its remaining stages.
	return f.added, nil
}
