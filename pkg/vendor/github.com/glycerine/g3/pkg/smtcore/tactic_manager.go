package smtcore

// Source: src/cmd_context/tactic_manager.cpp:22-66;
// src/cmd_context/tactic_manager.h:24-43; src/util/dictionary.h:25-29.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The dictionaries hold scalar indices into native descriptor slices.
// Finalization follows insertion order, never hash order.

type tacticManager struct {
	self                      *tacticManager
	symbols                   *usymbol.Manager
	budget                    *arena.Budget
	nameToTactic, nameToProbe usymbol.DictionaryInt32
	tactics                   []*tacticCmd
	probes                    []*tacticProbeInfo
	ownerBytes                uint64
	closed                    bool
}

func newTacticManager(s *usymbol.Manager, b *arena.Budget) (*tacticManager, error) {
	if s == nil {
		return nil, usymbol.ErrOwner
	}
	if _, e := s.Hash(usymbol.Symbol{}); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(tacticManager{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	r := &tacticManager{symbols: s, budget: b, ownerBytes: n}
	r.self = r
	if e := r.nameToTactic.Init(s); e != nil {
		b.Release(n)
		return nil, e
	}
	if e := r.nameToProbe.Init(s); e != nil {
		r.nameToTactic.Close()
		b.Release(n)
		return nil, e
	}
	return r, nil
}
func (r *tacticManager) open() error {
	if r == nil || r.self != r || r.closed {
		return errTacticRegistryOwner
	}
	_, e := r.symbols.Hash(usymbol.Symbol{})
	return e
}
func (r *tacticManager) InsertTactic(cmd *tacticCmd) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := cmd.open(); e != nil {
		return e
	}
	if cmd.params.Symbols() != r.symbols || cmd.registered != nil {
		return errTacticRegistryOwner
	}
	if smtcoreSourceDebug {
		if _, exists, e := r.nameToTactic.Find(cmd.name); e != nil {
			return e
		} else if exists {
			return ast.ErrASTDomain
		}
	}
	if uint64(len(r.tactics)) >= math.MaxUint32 {
		return arena.ErrBounds
	}
	if e := r.nameToTactic.Insert(cmd.name, int32(len(r.tactics))); e != nil {
		return e
	}
	r.tactics = append(r.tactics, cmd)
	cmd.registered = r
	return nil
}
func (r *tacticManager) InsertProbe(info *tacticProbeInfo) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := info.open(); e != nil {
		return e
	}
	if info.symbols != r.symbols || info.registered != nil {
		return errTacticRegistryOwner
	}
	if smtcoreSourceDebug {
		if _, exists, e := r.nameToProbe.Find(info.name); e != nil {
			return e
		} else if exists {
			return ast.ErrASTDomain
		}
	}
	if uint64(len(r.probes)) >= math.MaxUint32 {
		return arena.ErrBounds
	}
	if e := r.nameToProbe.Insert(info.name, int32(len(r.probes))); e != nil {
		return e
	}
	r.probes = append(r.probes, info)
	info.registered = r
	return nil
}
func (r *tacticManager) FindTactic(name usymbol.Symbol) (*tacticCmd, error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	bits, ok, e := r.nameToTactic.Find(name)
	if e != nil || !ok {
		return nil, e
	}
	i := uint32(bits) // dictionary's concrete scalar carries all handle bits
	if uint64(i) < uint64(len(r.tactics)) {
		return r.tactics[i], nil
	}
	return nil, errTacticRegistryOwner
}
func (r *tacticManager) FindProbe(name usymbol.Symbol) (*tacticProbeInfo, error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	bits, ok, e := r.nameToProbe.Find(name)
	if e != nil || !ok {
		return nil, e
	}
	i := uint32(bits)
	if uint64(i) < uint64(len(r.probes)) {
		return r.probes[i], nil
	}
	return nil, errTacticRegistryOwner
}
func (r *tacticManager) NumTactics() (uint32, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	return uint32(len(r.tactics)), nil
}
func (r *tacticManager) NumProbes() (uint32, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	return uint32(len(r.probes)), nil
}
func (r *tacticManager) Tactic(i uint32) (*tacticCmd, error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(r.tactics)) {
		return nil, arena.ErrBounds
	}
	return r.tactics[i], nil
}
func (r *tacticManager) Probe(i uint32) (*tacticProbeInfo, error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(r.probes)) {
		return nil, arena.ErrBounds
	}
	return r.probes[i], nil
}
func (r *tacticManager) FinalizeTactics() error {
	if e := r.open(); e != nil {
		return e
	}
	for _, cmd := range r.tactics {
		if e := cmd.Close(); e != nil {
			return e
		}
	}
	r.tactics = r.tactics[:0] // source reset retains allocated backing
	if e := r.nameToTactic.Reset(); e != nil {
		return e
	}
	return nil
}
func (r *tacticManager) FinalizeProbes() error {
	if e := r.open(); e != nil {
		return e
	}
	for _, info := range r.probes {
		if e := info.Close(); e != nil {
			return e
		}
	}
	r.probes = r.probes[:0]
	if e := r.nameToProbe.Reset(); e != nil {
		return e
	}
	return nil
}
func (r *tacticManager) Close() error {
	if r == nil || r.self != r {
		return errTacticRegistryOwner
	}
	if r.closed {
		return nil
	}
	if e := r.FinalizeTactics(); e != nil {
		return e
	}
	if e := r.FinalizeProbes(); e != nil {
		return e
	}
	// Source members unwind in reverse order after the destructor body.
	r.probes = nil
	r.tactics = nil
	r.nameToProbe.Close()
	r.nameToTactic.Close()
	r.budget.Release(r.ownerBytes)
	r.ownerBytes = 0
	r.closed = true
	return nil
}
