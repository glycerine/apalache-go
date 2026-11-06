package automata

// Source: src/math/automata/automaton.h:645-683.

func sameMove(a, b Move) bool { return a == b }

func removeMoveAt(moves []Move, index int) []Move {
	moves[index] = moves[len(moves)-1]
	return moves[:len(moves)-1]
}

func findMove(moves []Move, target Move) int {
	for i, move := range moves {
		if sameMove(move, target) {
			return i
		}
	}
	return -1
}

func (a *Automaton) removeMove(move Move) bool {
	outIndex := findMove(a.out[move.Source], move)
	inIndex := findMove(a.in[move.Destination], move)
	if outIndex < 0 || inIndex < 0 {
		return false
	}
	a.out[move.Source] = removeMoveAt(a.out[move.Source], outIndex)
	a.in[move.Destination] = removeMoveAt(a.in[move.Destination], inIndex)
	return true
}

func (a *Automaton) allEpsilonIncoming(state uint32) bool {
	for _, move := range a.in[state] {
		if !move.Epsilon {
			return false
		}
	}
	return true
}
