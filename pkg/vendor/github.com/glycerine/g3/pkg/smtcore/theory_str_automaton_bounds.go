package smtcore

// Source: src/smt/theory_str.cpp refine_automaton_lower_bound,
// refine_automaton_upper_bound and find_automaton_initial_bounds.

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/automata"
)

func stringAutomatonNextDepth(aut *automata.Automaton, current []uint32) []uint32 {
	seen := make(map[uint32]struct{})
	var next []uint32
	for _, state := range current {
		for _, move := range aut.ExpandedMovesFrom(state, true) {
			if _, found := seen[move.Destination]; !found {
				seen[move.Destination] = struct{}{}
				next = append(next, move.Destination)
			}
		}
	}
	return next
}

func stringAutomatonFrontierAccepts(aut *automata.Automaton, frontier []uint32) bool {
	for _, state := range frontier {
		if aut.IsFinal(state) {
			return true
		}
	}
	return false
}

// The source converts its rational input to unsigned; the concrete retained
// domain is uint32. A signed result represents the source -1 sentinel.
func refineStringAutomatonLowerBound(aut *automata.Automaton, bound uint32) (atBound bool, next int64, result error) {
	if aut == nil {
		return false, -1, ast.ErrASTDomain
	}
	if len(aut.FinalStates()) == 0 {
		return false, -1, nil
	}
	frontier := aut.EpsilonClosure(aut.Initial())
	depth := uint32(0)
	for len(frontier) != 0 {
		if depth == bound {
			atBound = stringAutomatonFrontierAccepts(aut, frontier)
			break
		}
		frontier = stringAutomatonNextDepth(aut, frontier)
		depth++
	}
	if depth < bound {
		return false, -1, nil
	}
	for len(frontier) != 0 {
		if depth > bound && stringAutomatonFrontierAccepts(aut, frontier) {
			return atBound, int64(depth), nil
		}
		frontier = stringAutomatonNextDepth(aut, frontier)
		depth++
	}
	return atBound, -1, nil
}

func refineStringAutomatonUpperBound(aut *automata.Automaton, bound uint32) (atBound bool, previous int64, result error) {
	if aut == nil {
		return false, -1, ast.ErrASTDomain
	}
	if len(aut.FinalStates()) == 0 {
		return false, -1, nil
	}
	frontier := aut.EpsilonClosure(aut.Initial())
	previous = -1
	depth := uint32(0)
	for len(frontier) != 0 {
		if stringAutomatonFrontierAccepts(aut, frontier) {
			if depth == bound {
				atBound = true
			} else {
				previous = int64(depth)
			}
		}
		if depth == bound {
			break
		}
		frontier = stringAutomatonNextDepth(aut, frontier)
		depth++
	}
	return atBound, previous, nil
}

func (t *SMTStringTheory) findStringAutomatonInitialBounds(membership ast.ASTHandle, aut *automata.Automaton) (result error) {
	if aut == nil {
		return ast.ErrASTDomain
	}
	m := t.state.manager
	word, _, matched, err := m.SequenceBinaryOperationArguments(membership, 12)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return err
	}
	zeroAccepted, nonzero, err := refineStringAutomatonLowerBound(aut, 0)
	if err != nil {
		return err
	}
	if zeroAccepted {
		t.regexLastLower[word] = new(big.Int)
	}
	if nonzero == -1 {
		return ast.ErrASTDomain // Native UNREACHABLE: no nonzero solution.
	}
	if !zeroAccepted {
		t.regexLastLower[word] = big.NewInt(nonzero)
	}
	bound, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(nonzero, 10)))
	if err != nil {
		return err
	}
	rhs, err := pin(m.MkArithmeticGE(length, bound))
	if err != nil {
		return err
	}
	if zeroAccepted {
		zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return err
		}
		emptyLength, err := pin(m.MkEq(length, zero))
		if err != nil {
			return err
		}
		rhs, err = pin(m.MkOr(emptyLength, rhs))
		if err != nil {
			return err
		}
	}
	guard := smtStringLengthPremises{theory: t, values: []ast.ASTHandle{membership}}
	return guard.imply(rhs) // Upper-bound inference is a native TODO here.
}
