package sat

// Source: src/sat/tactic/goal2sat.cpp:574-576,658-661;
// src/sat/sat_model_converter.cpp:253-262.

import "github.com/glycerine/g3/pkg/arena"

// ModelConverter is the concrete SAT elimination log. Copies own their entries
// independently of the solver that produced them.
type ModelConverter = satModelConverter

func NewModelConverter(b *arena.Budget) (*ModelConverter, error) {
	return newSATModelConverter(b)
}

func (s *Solver) CopyModelConverter(dst *ModelConverter) error {
	if err := s.open(); err != nil {
		return err
	}
	return dst.Copy(s.converter)
}

// AppendVariables preserves source collect_vars membership and first-seen
// order, using native Go storage rather than exporting the C++ set container.
func (m *satModelConverter) AppendVariables(dst []BoolVar) ([]BoolVar, error) {
	if err := m.open(); err != nil {
		return dst, err
	}
	seen := make(map[BoolVar]bool)
	for _, v := range dst {
		seen[v] = true
	}
	for _, entry := range m.entries {
		v := BoolVar(entry.varKind & 0x7fffffff)
		if !seen[v] {
			dst = append(dst, v)
			seen[v] = true
		}
	}
	return dst, nil
}
