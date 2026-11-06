package realclosure

// Source: src/math/realclosure/realclosure.cpp:233-299, 825-856,
// and 1808-1835.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type realExtensionKind uint8

const (
	// Keep the zero value algebraic because existing algebraic fixtures and
	// constructors use zero-initialized extension records.
	realExtensionAlgebraic realExtensionKind = iota
	realExtensionTranscendental
	realExtensionInfinitesimal
)

// algebraicRootOwner holds each source extension-kind registry. Indices remain
// stable after an extension is deleted, matching m_extensions.
type algebraicRootOwner struct {
	values          *realValueStore
	intervals       *numeral.BinaryRationalManager
	api             *APIValues
	roots           []*algebraicRoot
	transcendentals []*algebraicRoot
	infinitesimals  []*algebraicRoot
}

func (o *algebraicRootOwner) registry(kind realExtensionKind) *[]*algebraicRoot {
	if o == nil {
		return nil
	}
	switch kind {
	case realExtensionTranscendental:
		return &o.transcendentals
	case realExtensionInfinitesimal:
		return &o.infinitesimals
	default:
		return &o.roots
	}
}

func (r *algebraicRoot) isRegistered() bool {
	if r == nil || r.deleted || r.owner == nil || r.index < 0 {
		return false
	}
	registry := r.owner.registry(r.kind)
	return registry != nil && r.index < len(*registry) && (*registry)[r.index] == r
}

func (r *algebraicRoot) unregister() {
	if registry := r.owner.registry(r.kind); registry != nil && r.index >= 0 && r.index < len(*registry) && (*registry)[r.index] == r {
		(*registry)[r.index] = nil
	}
}

type algebraicRoot struct {
	owner                   *algebraicRootOwner
	kind                    realExtensionKind
	index                   int
	name                    string
	prettyName              string
	transcendentalStep      uint32
	refs                    uint32
	deleted                 bool
	polynomial              realPolynomial
	interval                *BinaryInterval
	isolating               *BinaryInterval
	signs                   *signDet
	signIndex               int
	infinitesimals          bool
	dependsOnInfinitesimals bool
}

// mkAlgebraic constructs the retained algebraic extension. The source
// extension begins at reference count zero; a rational-function value will
// retain it when that value is published.
func (o *algebraicRootOwner) mkAlgebraic(coefficients []*realValue, enclosure, isolating *BinaryInterval, signs *signDet, signIndex int) (*algebraicRoot, error) {
	if o == nil || o.values == nil || o.intervals == nil || enclosure == nil || isolating == nil || (signs != nil && (signs.store != o.values || signs.deleted || signIndex < 0 || signIndex >= len(signs.conditions))) || (signs == nil && signIndex != -1) {
		return nil, arena.ErrInvalidSlot
	}
	r := &algebraicRoot{owner: o, kind: realExtensionAlgebraic, signIndex: signIndex}
	if err := o.registerAlgebraicRoot(r); err != nil {
		return nil, err
	}
	cleanup := func(err error) (*algebraicRoot, error) {
		_ = r.destroy()
		o.roots[r.index] = nil
		return nil, err
	}
	if err := o.values.setPolynomial(&r.polynomial, coefficients); err != nil {
		return cleanup(err)
	}
	var err error
	r.interval, err = NewBinaryInterval(o.intervals)
	if err != nil {
		return cleanup(err)
	}
	if err = r.interval.SetFrom(enclosure); err != nil {
		return cleanup(err)
	}
	r.isolating, err = NewBinaryInterval(o.intervals)
	if err != nil {
		return cleanup(err)
	}
	if err = r.isolating.SetFrom(isolating); err != nil {
		return cleanup(err)
	}
	if signs != nil {
		if err = signs.retain(); err != nil {
			return cleanup(err)
		}
		r.signs = signs
	}
	for _, coefficient := range r.polynomial.coefficients {
		depends, dependencyErr := o.values.dependsOnInfinitesimalsValue(coefficient)
		if dependencyErr != nil {
			return cleanup(dependencyErr)
		}
		if depends {
			r.dependsOnInfinitesimals = true
			break
		}
	}
	return r, nil
}

func (o *algebraicRootOwner) mkTranscendental(name, prettyName string, enclosure *BinaryInterval) (*algebraicRoot, error) {
	if o == nil || o.values == nil || o.intervals == nil || enclosure == nil || enclosure.numbers != o.intervals || name == "" {
		return nil, arena.ErrInvalidSlot
	}
	r := &algebraicRoot{owner: o, kind: realExtensionTranscendental, name: name, prettyName: prettyName, interval: enclosure, transcendentalStep: 1}
	if err := o.registerExtension(r, realExtensionTranscendental); err != nil {
		return nil, err
	}
	return r, nil
}

func (o *algebraicRootOwner) mkInfinitesimal(name, prettyName string, enclosure *BinaryInterval) (*algebraicRoot, error) {
	if o == nil || o.values == nil || o.intervals == nil || enclosure == nil || enclosure.numbers != o.intervals || name == "" {
		return nil, arena.ErrInvalidSlot
	}
	r := &algebraicRoot{owner: o, kind: realExtensionInfinitesimal, name: name, prettyName: prettyName, interval: enclosure, infinitesimals: true}
	if err := o.registerExtension(r, realExtensionInfinitesimal); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *algebraicRoot) retain() error {
	if !r.isRegistered() || r.refs == ^uint32(0) {
		return arena.ErrInvalidSlot
	}
	r.refs++
	return nil
}

func (r *algebraicRoot) release() error {
	if !r.isRegistered() || r.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if r.refs > 1 {
		r.refs--
		return nil
	}
	if err := r.destroy(); err != nil {
		return err
	}
	r.refs = 0
	r.deleted = true
	r.unregister()
	return nil
}

// discardUnpublished closes the gap between mk_algebraic registration and
// add_root's rational-function publication. It is used only when the latter
// fails before taking the root's first reference.
func (r *algebraicRoot) discardUnpublished() error {
	if !r.isRegistered() || r.refs != 0 {
		return arena.ErrInvalidSlot
	}
	if err := r.destroy(); err != nil {
		return err
	}
	r.deleted = true
	r.unregister()
	return nil
}

// destroy follows del_algebraic: polynomial, approximation interval,
// isolating interval, then the shared sign-determination owner.
func (r *algebraicRoot) destroy() error {
	if err := r.owner.values.resetPolynomial(&r.polynomial); err != nil {
		return err
	}
	if r.interval != nil {
		if err := r.interval.Close(); err != nil {
			return err
		}
		r.interval = nil
	}
	if r.isolating != nil {
		if err := r.isolating.Close(); err != nil {
			return err
		}
		r.isolating = nil
	}
	if r.signs != nil {
		if err := r.signs.release(); err != nil {
			return err
		}
		r.signs = nil
	}
	return nil
}

func (r *algebraicRoot) numRootsInsideInterval() (int, error) {
	if !r.isRegistered() || r.kind != realExtensionAlgebraic {
		return 0, arena.ErrInvalidSlot
	}
	if r.signs == nil {
		return 1, nil
	}
	return len(r.signs.prs), nil
}
