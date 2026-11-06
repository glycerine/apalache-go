package automata

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// BooleanAlgebra operates on concrete uint32 condition handles. A retained AST
// consumer can keep its expressions in a handle table without exposing an
// untyped value or callback surface to this package.
type BooleanAlgebra interface {
	False() uint32
	True() uint32
	And(left, right uint32) uint32
	Or(left, right uint32) uint32
	Not(condition uint32) uint32
	Satisfiable(condition uint32) lbool.LBool
}

type Symbolic struct{ algebra BooleanAlgebra }

func NewSymbolic(algebra BooleanAlgebra) (*Symbolic, error) {
	if algebra == nil {
		return nil, fmt.Errorf("automata: nil Boolean algebra")
	}
	return &Symbolic{algebra: algebra}, nil
}

func (s *Symbolic) orAll(conditions []uint32) uint32 {
	result := s.algebra.False()
	for _, condition := range conditions {
		result = s.algebra.Or(result, condition)
	}
	return result
}

type minterm struct {
	selected  []bool
	condition uint32
}

func (s *Symbolic) minterms(conditions []uint32) ([]minterm, error) {
	var result []minterm
	var visit func(int, uint32, []bool) error
	visit = func(index int, current uint32, selected []bool) error {
		sat := s.algebra.Satisfiable(current)
		if sat == lbool.Undef {
			return fmt.Errorf("automata: Boolean algebra returned unknown")
		}
		if sat == lbool.False {
			return nil
		}
		if index == len(conditions) {
			result = append(result, minterm{append([]bool(nil), selected...), current})
			return nil
		}
		if err := visit(index+1, s.algebra.And(current, conditions[index]), append(selected, true)); err != nil {
			return err
		}
		return visit(index+1, s.algebra.And(current, s.algebra.Not(conditions[index])), append(selected, false))
	}
	if err := visit(0, s.algebra.True(), nil); err != nil {
		return nil, err
	}
	return result, nil
}

func stateKey(states map[uint32]bool) string {
	values := make([]int, 0, len(states))
	for state := range states {
		values = append(values, int(state))
	}
	sort.Ints(values)
	var b strings.Builder
	for _, state := range values {
		b.WriteString(strconv.Itoa(state))
		b.WriteByte(',')
	}
	return b.String()
}

func closureSet(a *Automaton, states map[uint32]bool) map[uint32]bool {
	result := make(map[uint32]bool)
	for state := range states {
		for _, reached := range a.EpsilonClosure(state) {
			result[reached] = true
		}
	}
	return result
}

func (s *Symbolic) determinize(source *Automaton, flipAcceptance bool) (*Automaton, error) {
	initialSet := closureSet(source, map[uint32]bool{source.initial: true})
	sets := []map[uint32]bool{initialSet}
	ids := map[string]uint32{stateKey(initialSet): 0}
	todo := []uint32{0}
	var final []uint32
	if source.IsFinalConfiguration(initialSet) != flipAcceptance {
		final = append(final, 0)
	}
	var moves []Move
	for len(todo) != 0 {
		id := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		states := sets[id]
		var expanded []Move
		for state := range states {
			expanded = append(expanded, source.ExpandedMovesFrom(state, true)...)
		}
		conditions := make([]uint32, len(expanded))
		for i := range expanded {
			conditions[i] = expanded[i].Condition
		}
		terms, err := s.minterms(conditions)
		if err != nil {
			return nil, err
		}
		for _, term := range terms {
			targets := make(map[uint32]bool)
			for i, selected := range term.selected {
				if selected {
					targets[expanded[i].Destination] = true
				}
			}
			key := stateKey(targets)
			target, found := ids[key]
			if !found {
				target = uint32(len(sets))
				ids[key] = target
				sets = append(sets, targets)
				todo = append(todo, target)
				if source.IsFinalConfiguration(targets) != flipAcceptance {
					final = append(final, target)
				}
			}
			moves = append(moves, ConditionMove(id, target, term.condition))
		}
	}
	if len(final) == 0 {
		return Empty(), nil
	}
	return New(0, final, moves), nil
}

func (s *Symbolic) Determinize(source *Automaton) (*Automaton, error) {
	return s.determinize(source, false)
}

func (s *Symbolic) Complement(source *Automaton) (*Automaton, error) {
	return s.determinize(source, true)
}

func (s *Symbolic) RemoveEpsilons(source *Automaton) *Automaton {
	var final []uint32
	var moves []Move
	for state := uint32(0); state < source.NumStates(); state++ {
		closure := source.EpsilonClosure(state)
		isFinal := false
		for _, reached := range closure {
			isFinal = isFinal || source.IsFinal(reached)
		}
		if isFinal {
			final = append(final, state)
		}
		for _, move := range source.ExpandedMovesFrom(state, true) {
			move.Source = state
			moves = append(moves, move)
		}
	}
	return New(source.initial, final, moves)
}

func (s *Symbolic) Total(source *Automaton) (*Automaton, error) {
	result := s.RemoveEpsilons(source)
	dead := result.NumStates()
	var additions []Move
	for state := uint32(0); state < dead; state++ {
		conditions := make([]uint32, len(result.out[state]))
		for i := range result.out[state] {
			conditions[i] = result.out[state][i].Condition
		}
		missing := s.algebra.Not(s.orAll(conditions))
		sat := s.algebra.Satisfiable(missing)
		if sat == lbool.Undef {
			return nil, fmt.Errorf("automata: Boolean algebra returned unknown")
		}
		if sat == lbool.True {
			additions = append(additions, ConditionMove(state, dead, missing))
		}
	}
	if len(additions) == 0 {
		return result, nil
	}
	additions = append(additions, ConditionMove(dead, dead, s.algebra.True()))
	for _, move := range additions {
		result.add(move)
	}
	return result, nil
}

type statePair struct{ left, right uint32 }

func (s *Symbolic) Product(left, right *Automaton) (*Automaton, error) {
	initial := statePair{left.initial, right.initial}
	ids := map[statePair]uint32{initial: 0}
	todo := []statePair{initial}
	var final []uint32
	if left.IsFinal(initial.left) && right.IsFinal(initial.right) {
		final = append(final, 0)
	}
	var moves []Move
	for len(todo) != 0 {
		pair := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		source := ids[pair]
		leftMoves := left.ExpandedMovesFrom(pair.left, true)
		rightMoves := right.ExpandedMovesFrom(pair.right, true)
		for _, lmove := range leftMoves {
			for _, rmove := range rightMoves {
				condition := s.algebra.And(lmove.Condition, rmove.Condition)
				sat := s.algebra.Satisfiable(condition)
				if sat == lbool.Undef {
					return nil, fmt.Errorf("automata: Boolean algebra returned unknown")
				}
				if sat == lbool.False {
					continue
				}
				targetPair := statePair{lmove.Destination, rmove.Destination}
				target, found := ids[targetPair]
				if !found {
					target = uint32(len(ids))
					ids[targetPair] = target
					todo = append(todo, targetPair)
					if left.IsFinal(targetPair.left) && right.IsFinal(targetPair.right) {
						final = append(final, target)
					}
				}
				moves = append(moves, ConditionMove(source, target, condition))
			}
		}
	}
	if len(final) == 0 {
		return Empty(), nil
	}

	back := make([]bool, len(ids))
	for _, state := range final {
		back[state] = true
	}
	changed := true
	for changed {
		changed = false
		for _, move := range moves {
			if back[move.Destination] && !back[move.Source] {
				back[move.Source] = true
				changed = true
			}
		}
	}
	filtered := moves[:0]
	for _, move := range moves {
		if back[move.Destination] {
			filtered = append(filtered, move)
		}
	}
	return New(0, final, filtered), nil
}

func (s *Symbolic) Difference(left, right *Automaton) (*Automaton, error) {
	complement, err := s.Complement(right)
	if err != nil {
		return nil, err
	}
	return s.Product(left, complement)
}

func (s *Symbolic) equivalent(left, right uint32) (bool, error) {
	difference := s.algebra.Or(s.algebra.And(left, s.algebra.Not(right)), s.algebra.And(s.algebra.Not(left), right))
	sat := s.algebra.Satisfiable(difference)
	if sat == lbool.Undef {
		return false, fmt.Errorf("automata: Boolean algebra returned unknown")
	}
	return sat == lbool.False, nil
}

func (s *Symbolic) Minimize(source *Automaton) (*Automaton, error) {
	total, err := s.Total(source)
	if err != nil || total.IsEmpty() || total.IsEpsilon() {
		return total, err
	}
	blocks := [][]uint32{{}, {}}
	for state := uint32(0); state < total.NumStates(); state++ {
		if total.IsFinal(state) {
			blocks[0] = append(blocks[0], state)
		} else {
			blocks[1] = append(blocks[1], state)
		}
	}
	if len(blocks[0]) == 0 {
		blocks = blocks[1:]
	}
	if len(blocks) > 1 && len(blocks[1]) == 0 {
		blocks = blocks[:1]
	}
	for {
		owner := make([]int, total.NumStates())
		for i, block := range blocks {
			for _, state := range block {
				owner[state] = i
			}
		}
		changed := false
		var next [][]uint32
		for _, block := range blocks {
			var groups [][]uint32
			for _, state := range block {
				placed := false
				for i := range groups {
					equal := true
					for targetBlock := range blocks {
						guards := func(q uint32) uint32 {
							var values []uint32
							for _, move := range total.out[q] {
								if owner[move.Destination] == targetBlock {
									values = append(values, move.Condition)
								}
							}
							return s.orAll(values)
						}
						ok, compareErr := s.equivalent(guards(state), guards(groups[i][0]))
						if compareErr != nil {
							return nil, compareErr
						}
						if !ok {
							equal = false
							break
						}
					}
					if equal {
						groups[i] = append(groups[i], state)
						placed = true
						break
					}
				}
				if !placed {
					groups = append(groups, []uint32{state})
				}
			}
			if len(groups) > 1 {
				changed = true
			}
			next = append(next, groups...)
		}
		blocks = next
		if !changed {
			break
		}
	}
	owner := make([]uint32, total.NumStates())
	for i, block := range blocks {
		for _, state := range block {
			owner[state] = uint32(i)
		}
	}
	type edge struct{ source, destination uint32 }
	conditions := make(map[edge]uint32)
	order := make([]edge, 0)
	for _, move := range total.Moves() {
		key := edge{owner[move.Source], owner[move.Destination]}
		if current, found := conditions[key]; found {
			conditions[key] = s.algebra.Or(current, move.Condition)
		} else {
			conditions[key] = move.Condition
			order = append(order, key)
		}
	}
	moves := make([]Move, 0, len(order))
	for _, key := range order {
		moves = append(moves, ConditionMove(key.source, key.destination, conditions[key]))
	}
	var final []uint32
	for i, block := range blocks {
		for _, state := range block {
			if total.IsFinal(state) {
				final = append(final, uint32(i))
				break
			}
		}
	}
	return New(owner[total.initial], final, moves), nil
}

// MinimizeTotal accepts an already total deterministic automaton. Total is
// idempotent, so it shares the checked implementation with Minimize.
func (s *Symbolic) MinimizeTotal(source *Automaton) (*Automaton, error) {
	return s.Minimize(source)
}

// BitsetAlgebra is a concrete Boolean algebra for alphabets with at most 32
// symbols. A condition is the bitset of symbols it accepts.
type BitsetAlgebra struct{ Universe uint32 }

func (b BitsetAlgebra) False() uint32                 { return 0 }
func (b BitsetAlgebra) True() uint32                  { return b.Universe }
func (b BitsetAlgebra) And(left, right uint32) uint32 { return left & right & b.Universe }
func (b BitsetAlgebra) Or(left, right uint32) uint32  { return (left | right) & b.Universe }
func (b BitsetAlgebra) Not(condition uint32) uint32   { return (^condition) & b.Universe }
func (b BitsetAlgebra) Satisfiable(condition uint32) lbool.LBool {
	if condition&b.Universe == 0 {
		return lbool.False
	}
	return lbool.True
}
