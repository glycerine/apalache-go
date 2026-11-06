package automata

// Source: src/math/automata/automaton.h:600-635 and 454-467.

func (a *Automaton) sinkifyDeadStates() {
	dead := make(map[uint32]bool)
	for state := uint32(0); state < uint32(len(a.out)); state++ {
		if !a.IsFinal(state) {
			dead[state] = true
		}
	}
	for changed := true; changed; {
		changed = false
		var remove []uint32
		for state := range dead {
			for _, move := range a.out[state] {
				if !dead[move.Destination] {
					remove = append(remove, state)
					break
				}
			}
		}
		for _, state := range remove {
			delete(dead, state)
			changed = true
		}
	}
	for state := range dead {
		a.out[state] = nil
	}
}

func (a *Automaton) trimTrailingStates() {
	for len(a.out) != 0 {
		state := uint32(len(a.out) - 1)
		if state == a.initial || len(a.in[state]) != 0 {
			return
		}
		a.RemoveFinal(state)
		a.out = a.out[:state]
	}
}
