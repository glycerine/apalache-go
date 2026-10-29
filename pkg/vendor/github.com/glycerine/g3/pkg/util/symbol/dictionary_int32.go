package symbol

// Source: src/util/dictionary.h; src/util/map.h.

import "github.com/glycerine/g3/pkg/arena"

// DictionaryInt32 is the concrete integer dictionary used by AST and
// tactic registries. Its consumers only perform lookup and mutation, so Go map
// order is sufficient.
type DictionaryInt32 struct {
	symbols *Manager
	values  map[Symbol]int32
}

func (d *DictionaryInt32) Init(s *Manager) error {
	if s == nil {
		return ErrOwner
	}
	if _, e := s.Hash(Symbol{}); e != nil {
		return e
	}
	d.symbols = s
	d.values = make(map[Symbol]int32)
	return nil
}

func (d *DictionaryInt32) validate(k Symbol) error {
	if d == nil || d.symbols == nil || d.values == nil {
		return arena.ErrClosed
	}
	_, e := d.symbols.Hash(k)
	return e
}

func (d *DictionaryInt32) Find(k Symbol) (int32, bool, error) {
	if e := d.validate(k); e != nil {
		return 0, false, e
	}
	v, ok := d.values[k]
	return v, ok, nil
}

func (d *DictionaryInt32) Insert(k Symbol, value int32) error {
	if e := d.validate(k); e != nil {
		return e
	}
	d.values[k] = value
	return nil
}

func (d *DictionaryInt32) Remove(k Symbol) error {
	if e := d.validate(k); e != nil {
		return e
	}
	delete(d.values, k)
	return nil
}

func (d *DictionaryInt32) Reset() error {
	if d == nil || d.symbols == nil || d.values == nil {
		return arena.ErrClosed
	}
	clear(d.values)
	return nil
}

func (d *DictionaryInt32) Finalize() error { return d.Reset() }

func (d *DictionaryInt32) Close() {
	if d == nil {
		return
	}
	d.symbols = nil
	d.values = nil
}

func (d *DictionaryInt32) Size() uint32 {
	if d == nil {
		return 0
	}
	return uint32(len(d.values))
}
