package smtcore

// Source: src/smt/theory_str.cpp final_check_eh per-String regex intersection.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtStringRegexConstraint struct {
	regex ast.ASTHandle
	owner *rewriter.StringRegexAutomaton
}

func (f *smtStringRegexStage) intersections(options SMTStrState) error {
	t, m := f.theory, f.theory.state.manager
	var words []ast.ASTHandle
	for word := range t.regexTermsByString {
		words = append(words, word)
	}
	stringSortHandles(words)
	for _, word := range words {
		var constraints []smtStringRegexConstraint
		for _, member := range t.regexTermsByString[word] {
			regex, err := m.GetArg(member, 1)
			if err != nil {
				return err
			}
			// Native executes these readers before inspecting assumptions.
			// Their returned values are unused for the retained unbounded
			// specialization, but their real preconditions/errors still apply.
			if _, _, err = t.lengthValue(word); err != nil {
				return err
			}
			length, err := f.pin(m.MkSequenceLength(word))
			if err != nil {
				return err
			}
			if _, _, err = t.stringArithmeticBound(length, true); err != nil {
				return err
			}
			if _, _, err = t.stringArithmeticBound(length, false); err != nil {
				return err
			}
			if owner := t.regexStage.automata[regex]; owner != nil {
				// The source comment says "assigned", but the actual loop
				// has no assignment filter. Keep its actual selection.
				constraints = append(constraints, smtStringRegexConstraint{regex: regex, owner: owner})
			}
		}
		var intersection *rewriter.StringRegexAutomaton
		var used []smtStringRegexConstraint
		for _, constraint := range constraints {
			if intersection == nil {
				// Native uses the first cached positive automaton directly,
				// even when this membership is currently false or unassigned.
				intersection = constraint.owner
				used = append(used, constraint)
				continue
			}
			if stringRegexCounter(t.regexStage.lengthAttempts, constraint.regex) < options.RegexAutomataLengthAttemptThreshold {
				continue
			}
			left, err := intersection.Graph()
			if err != nil {
				return err
			}
			right, err := constraint.owner.Graph()
			if err != nil {
				return err
			}
			difficulty, err := estimateStringAutomataIntersectionDifficulty(left, right)
			if err != nil {
				return err
			}
			if difficulty > options.RegexAutomataIntersectionDifficultyThreshold && stringRegexCounter(t.regexStage.intersectionFail, constraint.regex) < options.RegexAutomataFailedIntersectionThreshold {
				incrementStringRegexCounter(t.regexStage.intersectionFail, constraint.regex)
				continue
			}
			member, err := f.pin(m.MkSequenceInRegex(word, constraint.regex))
			if err != nil {
				return err
			}
			assignment, err := f.assignment(member)
			if err != nil {
				return err
			}
			input := constraint.owner
			if assignment != lbool.True {
				complement, err := f.pin(m.MkRegexComplement(constraint.regex))
				if err != nil {
					return err
				}
				input, err = rewriter.NewStringRegexAutomaton(m, complement, t.regexExpressionSolver)
				if err != nil {
					return err
				}
				t.regexStage.owned = append(t.regexStage.owned, input)
			}
			product, err := rewriter.NewStringRegexAutomatonProduct(intersection, input, t.regexExpressionSolver)
			if err != nil {
				return err
			}
			t.regexStage.products = append(t.regexStage.products, product)
			intersection = product
			used = append(used, constraint)
			graph, err := product.Graph()
			if err != nil {
				return err
			}
			if graph.IsEmpty() {
				break
			}
		}
		if intersection != nil {
			graph, err := intersection.Graph()
			if err != nil {
				return err
			}
			graph.Compress()
		}
		var conflictTerms []ast.ASTHandle
		for _, constraint := range used {
			member, err := f.pin(m.MkSequenceInRegex(word, constraint.regex))
			if err != nil {
				return err
			}
			assignment, err := f.assignment(member)
			if err != nil {
				return err
			}
			if assignment == lbool.True {
				conflictTerms = append(conflictTerms, member)
			} else if assignment == lbool.False {
				negative, err := f.pin(m.MkNot(member))
				if err != nil {
					return err
				}
				conflictTerms = append(conflictTerms, negative)
			}
		}
		guard, err := f.pin(m.MkAndArray(conflictTerms))
		if err != nil {
			return err
		}
		if intersection == nil {
			continue
		}
		graph, err := intersection.Graph()
		if err != nil {
			return err
		}
		if len(used) > 1 && len(graph.FinalStates()) == 1 && graph.IsFinal(graph.Initial()) && len(graph.MovesFrom(graph.Initial())) == 0 {
			empty, err := f.pin(t.stringLiteral(""))
			if err != nil {
				return err
			}
			wordEmpty, err := f.pin(m.MkEq(word, empty))
			if err != nil {
				return err
			}
			length, err := f.pin(m.MkSequenceLength(word))
			if err != nil {
				return err
			}
			zero, err := f.number(new(big.Int))
			if err != nil {
				return err
			}
			lengthZero, err := f.pin(m.MkEq(length, zero))
			if err != nil {
				return err
			}
			rhs, err := f.pin(m.MkAnd(wordEmpty, lengthZero))
			if err != nil {
				return err
			}
			if err = f.imply(guard, rhs); err != nil {
				return err
			}
		}
		if graph.IsEmpty() {
			conflict, err := f.pin(m.MkNot(guard))
			if err != nil {
				return err
			}
			if err = f.publish(conflict); err != nil {
				return err
			}
			if err = t.addPersistedStringAxiom(conflict); err != nil {
				return err
			}
		}
	}
	return nil
}
