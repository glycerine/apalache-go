package symbol

// Source: src/util/symbol.h.

import "github.com/glycerine/g3/pkg/arena"

// SymbolWords encodes the pointer-free identity in source aggregate storage.
// DecodeSymbolWords validates the reconstructed identity against this manager.
func (s Symbol) SymbolWords() [5]uint32 {
	return [5]uint32{uint32(s.owner), uint32(s.owner >> 32), s.Value, uint32(s.Generation), s.Tag}
}
func (m *Manager) DecodeSymbolWords(v [5]uint32) (Symbol, error) {
	return m.FromToken(Symbol{owner: uint64(v[0]) | uint64(v[1])<<32, Value: v[2], Generation: arena.Generation(v[3]), Tag: v[4]}.Token())
}
func (s Symbol) IsNull() bool { return s.Tag == 0 && s.Value == 0 }
