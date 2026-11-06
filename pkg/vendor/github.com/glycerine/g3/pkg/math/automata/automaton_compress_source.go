package automata

// Source: src/math/automata/automaton.h:340-469.

func (a *Automaton) compressSource() {
	for state := 0; state < len(a.out); state++ {
		for index := 0; index < len(a.out[state]); {
			move := a.out[state][index]
			if !move.Epsilon {
				index++
				continue
			}
			source, destination := move.Source, move.Destination
			removedByBranch := false
			switch {
			case source == destination:
			case len(a.in[source]) == 1 && len(a.out[source]) == 1 && source != a.initial && (!a.IsFinal(source) || a.IsFinal(destination)):
				incoming := a.in[source][0]
				if incoming.Source == source {
					index++
					continue
				}
				a.add(Move{Source: incoming.Source, Destination: destination, Condition: incoming.Condition, Epsilon: incoming.Epsilon})
				a.removeMove(incoming)
			case len(a.out[destination]) == 1 && len(a.in[destination]) == 1 && destination != a.initial && (!a.IsFinal(destination) || a.IsFinal(source)):
				outgoing := a.out[destination][0]
				if outgoing.Destination == destination {
					index++
					continue
				}
				a.add(Move{Source: source, Destination: outgoing.Destination, Condition: outgoing.Condition, Epsilon: outgoing.Epsilon})
				a.removeMove(outgoing)
			case len(a.in[destination]) == 1 && (!a.IsFinal(destination) || a.IsFinal(source)) && destination != a.initial:
				outgoing := append([]Move(nil), a.out[destination]...)
				for _, next := range outgoing {
					a.removeMove(next)
					a.add(Move{Source: source, Destination: next.Destination, Condition: next.Condition, Epsilon: next.Epsilon})
				}
			case len(a.out[destination]) == 1 && a.allEpsilonIncoming(destination) && destination != a.initial && !a.IsFinal(destination):
				outgoing := a.out[destination][0]
				incoming := append([]Move(nil), a.in[destination]...)
				for _, previous := range incoming {
					a.removeMove(previous)
					a.add(Move{Source: previous.Source, Destination: outgoing.Destination, Condition: outgoing.Condition, Epsilon: outgoing.Epsilon})
				}
				a.removeMove(outgoing)
				removedByBranch = true
			case len(a.out[source]) == 1 && source != a.initial && (!a.IsFinal(source) || a.IsFinal(destination)):
				incoming := append([]Move(nil), a.in[source]...)
				for _, previous := range incoming {
					a.removeMove(previous)
					a.add(Move{Source: previous.Source, Destination: destination, Condition: previous.Condition, Epsilon: previous.Epsilon})
				}
			case len(a.out[source]) == 1 && (a.IsFinal(source) || !a.IsFinal(destination)):
				for _, next := range append([]Move(nil), a.out[destination]...) {
					a.add(Move{Source: source, Destination: next.Destination, Condition: next.Condition, Epsilon: next.Epsilon})
				}
			default:
				index++
				continue
			}
			if !removedByBranch {
				a.removeMove(move)
			}
		}
	}
	a.trimTrailingStates()
	a.sinkifyDeadStates()
}
