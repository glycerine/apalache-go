package automata

// Package automata implements the retained finite and symbolic automata from
// src/math/automata with concrete uint32 condition handles.

import (
	"fmt"
	"io"
)

// Move is an automaton edge. Epsilon distinguishes an epsilon edge from a
// labeled edge whose condition handle happens to be zero.
type Move struct {
	Source, Destination uint32
	Condition           uint32
	Epsilon             bool
}

func EpsilonMove(source, destination uint32) Move {
	return Move{Source: source, Destination: destination, Epsilon: true}
}

func ConditionMove(source, destination, condition uint32) Move {
	return Move{Source: source, Destination: destination, Condition: condition}
}

type Automaton struct {
	initial uint32
	out     [][]Move
	in      [][]Move
	final   []uint32
	finals  map[uint32]bool
}

func New(initial uint32, final []uint32, moves []Move) *Automaton {
	n := initial + 1
	for _, state := range final {
		if state >= n {
			n = state + 1
		}
	}
	for _, move := range moves {
		if move.Source >= n {
			n = move.Source + 1
		}
		if move.Destination >= n {
			n = move.Destination + 1
		}
	}
	a := &Automaton{initial: initial, out: make([][]Move, n), in: make([][]Move, n), finals: make(map[uint32]bool)}
	for _, state := range final {
		a.AddFinal(state)
	}
	for _, move := range moves {
		a.add(move)
	}
	return a
}

func Empty() *Automaton   { return New(0, nil, nil) }
func Epsilon() *Automaton { return New(0, []uint32{0}, nil) }
func Loop(condition uint32) *Automaton {
	return New(0, []uint32{0}, []Move{ConditionMove(0, 0, condition)})
}
func Singleton(condition uint32) *Automaton {
	return New(0, []uint32{1}, []Move{ConditionMove(0, 1, condition)})
}
func Sequence(sequence []uint32) *Automaton {
	moves := make([]Move, len(sequence))
	for i, condition := range sequence {
		moves[i] = ConditionMove(uint32(i), uint32(i+1), condition)
	}
	return New(0, []uint32{uint32(len(sequence))}, moves)
}

func (a *Automaton) Clone() *Automaton { return New(a.initial, a.final, a.Moves()) }
func (a *Automaton) Initial() uint32   { return a.initial }
func (a *Automaton) NumStates() uint32 { return uint32(len(a.out)) }
func (a *Automaton) MoveCount() int {
	n := 0
	for i := range a.out {
		n += len(a.out[i])
	}
	return n
}
func (a *Automaton) FinalStates() []uint32     { return append([]uint32(nil), a.final...) }
func (a *Automaton) IsFinal(state uint32) bool { return a.finals[state] }
func (a *Automaton) IsEmpty() bool             { return len(a.final) == 0 }
func (a *Automaton) IsEpsilon() bool {
	return len(a.final) == 1 && a.final[0] == a.initial && a.MoveCount() == 0
}
func (a *Automaton) HasSingleFinalSink() bool {
	return len(a.final) == 1 && len(a.out[a.final[0]]) == 0
}
func (a *Automaton) FinalState() (uint32, error) {
	if len(a.final) != 1 {
		return 0, fmt.Errorf("automata: expected one final state")
	}
	return a.final[0], nil
}
func (a *Automaton) InDegree(state uint32) int     { return len(a.in[state]) }
func (a *Automaton) OutDegree(state uint32) int    { return len(a.out[state]) }
func (a *Automaton) InitialIsSource() bool         { return len(a.in[a.initial]) == 0 }
func (a *Automaton) MovesFrom(state uint32) []Move { return append([]Move(nil), a.out[state]...) }
func (a *Automaton) MovesTo(state uint32) []Move   { return append([]Move(nil), a.in[state]...) }
func (a *Automaton) Moves() []Move {
	var result []Move
	for i := range a.out {
		result = append(result, a.out[i]...)
	}
	return result
}
func (a *Automaton) MoveFrom(state uint32) (Move, error) {
	if len(a.out[state]) != 1 {
		return Move{}, fmt.Errorf("automata: expected one outgoing move")
	}
	return a.out[state][0], nil
}
func (a *Automaton) MoveTo(state uint32) (Move, error) {
	if len(a.in[state]) != 1 {
		return Move{}, fmt.Errorf("automata: expected one incoming move")
	}
	return a.in[state][0], nil
}

func (a *Automaton) AddFinal(state uint32) {
	a.ensure(state)
	if !a.finals[state] {
		a.finals[state] = true
		a.final = append(a.final, state)
	}
}
func (a *Automaton) RemoveFinal(state uint32) {
	if !a.finals[state] {
		return
	}
	delete(a.finals, state)
	for i, value := range a.final {
		if value == state {
			copy(a.final[i:], a.final[i+1:])
			a.final = a.final[:len(a.final)-1]
			return
		}
	}
}
func (a *Automaton) AddInitialToFinal() { a.AddFinal(a.initial) }
func (a *Automaton) AddFinalToInitialMoves() {
	for _, state := range append([]uint32(nil), a.final...) {
		if state == a.initial {
			continue
		}
		found := false
		for _, move := range a.out[state] {
			found = found || move.Epsilon && move.Destination == a.initial
		}
		if !found {
			a.add(EpsilonMove(state, a.initial))
		}
	}
}

func (a *Automaton) ensure(state uint32) {
	for uint32(len(a.out)) <= state {
		a.out = append(a.out, nil)
		a.in = append(a.in, nil)
	}
}
func (a *Automaton) add(move Move) {
	a.ensure(move.Source)
	a.ensure(move.Destination)
	if n := len(a.out[move.Source]); n != 0 && a.out[move.Source][n-1] == move {
		return
	}
	a.out[move.Source] = append(a.out[move.Source], move)
	a.in[move.Destination] = append(a.in[move.Destination], move)
}

func (a *Automaton) addUnique(move Move) {
	for _, present := range a.out[move.Source] {
		if present == move {
			return
		}
	}
	a.add(move)
}

func appendAutomaton(offset uint32, source *Automaton, moves *[]Move, final *[]uint32) {
	for _, move := range source.Moves() {
		move.Source += offset
		move.Destination += offset
		*moves = append(*moves, move)
	}
	for _, state := range source.final {
		*final = append(*final, state+offset)
	}
}

func Union(left, right *Automaton) *Automaton {
	if left.IsEmpty() {
		return right.Clone()
	}
	if right.IsEmpty() {
		return left.Clone()
	}
	offsetLeft, offsetRight := uint32(1), left.NumStates()+1
	moves := []Move{EpsilonMove(0, left.initial+offsetLeft), EpsilonMove(0, right.initial+offsetRight)}
	var final []uint32
	appendAutomaton(offsetLeft, left, &moves, &final)
	appendAutomaton(offsetRight, right, &moves, &final)
	return New(0, final, moves)
}

func Optional(source *Automaton) *Automaton {
	if source.IsEmpty() {
		return source.Clone()
	}
	offset, initial := uint32(0), source.initial
	var moves []Move
	if len(source.in[source.initial]) != 0 {
		offset, initial = 1, 0
		moves = append(moves, EpsilonMove(0, source.initial+offset))
	}
	moves = append(moves, EpsilonMove(initial, source.final[0]+offset))
	var final []uint32
	appendAutomaton(offset, source, &moves, &final)
	return New(initial, final, moves)
}

func Concat(left, right *Automaton) *Automaton {
	if left.IsEmpty() {
		return left.Clone()
	}
	if right.IsEmpty() {
		return right.Clone()
	}
	if left.IsEpsilon() {
		return right.Clone()
	}
	if right.IsEpsilon() {
		return left.Clone()
	}
	offsetLeft, offsetRight := uint32(1), left.NumStates()+1
	moves := []Move{EpsilonMove(0, left.initial+offsetLeft)}
	var ignored []uint32
	appendAutomaton(offsetLeft, left, &moves, &ignored)
	for _, state := range left.final {
		moves = append(moves, EpsilonMove(state+offsetLeft, right.initial+offsetRight))
	}
	var final []uint32
	appendAutomaton(offsetRight, right, &moves, &final)
	return New(0, final, moves)
}

func Reverse(source *Automaton) *Automaton {
	if source.IsEmpty() {
		return Empty()
	}
	moves := make([]Move, 0, source.MoveCount()+len(source.final))
	for _, move := range source.Moves() {
		move.Source, move.Destination = move.Destination, move.Source
		moves = append(moves, move)
	}
	initial := uint32(0)
	if len(source.final) == 1 {
		initial = source.final[0]
	} else {
		initial = source.NumStates()
		for _, state := range source.final {
			moves = append(moves, EpsilonMove(initial, state))
		}
	}
	return New(initial, []uint32{source.initial}, moves)
}

func (a *Automaton) EpsilonClosure(state uint32) []uint32 {
	return epsilonClosure(state, a.out, false)
}
func (a *Automaton) InverseEpsilonClosure(state uint32) []uint32 {
	return epsilonClosure(state, a.in, true)
}
func epsilonClosure(state uint32, edges [][]Move, inverse bool) []uint32 {
	seen := map[uint32]bool{state: true}
	stack := []uint32{state}
	var result []uint32
	for len(stack) != 0 {
		state = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result = append(result, state)
		for _, move := range edges[state] {
			next := move.Destination
			if inverse {
				next = move.Source
			}
			if move.Epsilon {
				if !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
	}
	return result
}

func (a *Automaton) ExpandedMovesFrom(state uint32, closeDestination bool) []Move {
	return a.expandedMoves(state, a.out, false, closeDestination)
}
func (a *Automaton) ExpandedMovesTo(state uint32, closeDestination bool) []Move {
	return a.expandedMoves(state, a.in, true, closeDestination)
}
func (a *Automaton) expandedMoves(state uint32, edges [][]Move, inverse, closeDestination bool) []Move {
	closure := a.EpsilonClosure(state)
	if inverse {
		closure = a.InverseEpsilonClosure(state)
	}
	var result []Move
	for _, current := range closure {
		for _, move := range edges[current] {
			if move.Epsilon {
				continue
			}
			target := move.Destination
			if inverse {
				target = move.Source
			}
			targets := []uint32{target}
			if closeDestination {
				if inverse {
					targets = a.InverseEpsilonClosure(target)
				} else {
					targets = a.EpsilonClosure(target)
				}
			}
			for _, next := range targets {
				if inverse {
					result = append(result, ConditionMove(next, state, move.Condition))
				} else {
					result = append(result, ConditionMove(current, next, move.Condition))
				}
			}
		}
	}
	return result
}

func (a *Automaton) IsFinalConfiguration(states map[uint32]bool) bool {
	for state := range states {
		if a.IsFinal(state) {
			return true
		}
	}
	return false
}
func (a *Automaton) IsEpsilonFree() bool {
	for _, move := range a.Moves() {
		if move.Epsilon {
			return false
		}
	}
	return true
}
func (a *Automaton) IsSink(state uint32) bool {
	if a.IsFinal(state) {
		return false
	}
	for _, move := range a.ExpandedMovesFrom(state, true) {
		if move.Destination != state {
			return false
		}
	}
	return true
}
func (a *Automaton) IsLoopState(state uint32) bool {
	for _, move := range a.ExpandedMovesFrom(state, true) {
		if move.Destination == state {
			return true
		}
	}
	return false
}
func (a *Automaton) IsSequence() (uint32, bool) {
	if a.IsFinal(a.initial) && (len(a.out[a.initial]) == 0 || len(a.out[a.initial]) == 1 && a.IsLoopState(a.initial)) {
		return 0, true
	}
	if a.IsEmpty() || len(a.in[a.initial]) != 0 || len(a.out[a.initial]) != 1 {
		return 0, false
	}
	length, state := uint32(1), a.out[a.initial][0].Destination
	seen := map[uint32]bool{a.initial: true}
	for !a.IsFinal(state) {
		if seen[state] || len(a.out[state]) != 1 || len(a.in[state]) != 1 {
			return 0, false
		}
		seen[state] = true
		state = a.out[state][0].Destination
		length++
	}
	return length, len(a.out[state]) == 0 || len(a.out[state]) == 1 && a.IsLoopState(state)
}

// Compress removes epsilon transitions, collapses their closures, and turns
// states that cannot reach a final state into sinks. This is the language
// preserving purpose of the source's local epsilon-splicing cases.
func (a *Automaton) Compress() {
	compressed := New(a.initial, nil, nil)
	compressed.ensure(a.NumStates() - 1)
	for state := uint32(0); state < a.NumStates(); state++ {
		closure := a.EpsilonClosure(state)
		for _, reached := range closure {
			if a.IsFinal(reached) {
				compressed.AddFinal(state)
			}
			for _, move := range a.out[reached] {
				if move.Epsilon {
					continue
				}
				for _, target := range a.EpsilonClosure(move.Destination) {
					compressed.addUnique(ConditionMove(state, target, move.Condition))
				}
			}
		}
	}

	live := make([]bool, compressed.NumStates())
	stack := append([]uint32(nil), compressed.final...)
	for _, state := range stack {
		live[state] = true
	}
	for len(stack) != 0 {
		state := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, move := range compressed.in[state] {
			if !live[move.Source] {
				live[move.Source] = true
				stack = append(stack, move.Source)
			}
		}
	}
	for state := range compressed.out {
		if !live[state] {
			compressed.out[state] = nil
		}
	}
	compressed.rebuildIncoming()
	for len(compressed.out) > 1 {
		last := uint32(len(compressed.out) - 1)
		if last == compressed.initial || len(compressed.in[last]) != 0 {
			break
		}
		compressed.RemoveFinal(last)
		compressed.out = compressed.out[:last]
		compressed.in = compressed.in[:last]
	}
	*a = *compressed
}

func (a *Automaton) rebuildIncoming() {
	a.in = make([][]Move, len(a.out))
	for state := range a.out {
		for _, move := range a.out[state] {
			a.in[move.Destination] = append(a.in[move.Destination], move)
		}
	}
}

func (a *Automaton) Accepts(sequence []uint32) bool {
	states := make(map[uint32]bool)
	for _, state := range a.EpsilonClosure(a.initial) {
		states[state] = true
	}
	for _, symbol := range sequence {
		next := make(map[uint32]bool)
		for state := range states {
			for _, move := range a.out[state] {
				if !move.Epsilon && move.Condition == symbol {
					for _, target := range a.EpsilonClosure(move.Destination) {
						next[target] = true
					}
				}
			}
		}
		states = next
	}
	return a.IsFinalConfiguration(states)
}

func (a *Automaton) Display(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "init: %d\nfinal: ", a.initial); err != nil {
		return err
	}
	for _, state := range a.final {
		if _, err := fmt.Fprintf(w, "%d ", state); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	for state := range a.out {
		for _, move := range a.out[state] {
			if move.Epsilon {
				if _, err := fmt.Fprintf(w, "%d -> %d \n", state, move.Destination); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(w, "%d -> %d if %d\n", state, move.Destination, move.Condition); err != nil {
				return err
			}
		}
	}
	return nil
}
