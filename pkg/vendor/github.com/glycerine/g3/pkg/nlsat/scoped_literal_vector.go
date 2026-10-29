package nlsat

// Source: src/nlsat/nlsat_scoped_literal_vector.h:27-75.
// Concrete retained solver literal ownership; storage is a native Go slice.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

type scopedLiteralVector struct {
	factory                                          *atomFactory
	result                                           []sat.Literal
	resetIndex                                       int
	resetting, shrinking, executing                  bool
	targetSize                                       int
	retiringAtom                                     ArithmeticAtom
	setActive, setOldRead, setOldDone, incomingOwned bool
	setIndex                                         uint32
	incoming                                         sat.Literal
	setRetiring, incomingRetiring                    ArithmeticAtom
}

func (v *scopedLiteralVector) push(l sat.Literal) error {
	if v.executing || v.resetting || v.setActive {
		return ErrSearchPending
	}
	if v.factory == nil {
		return ErrSearchOwner
	}
	if err := v.factory.incRef(l.Variable()); err != nil {
		return err
	}
	v.result = append(v.result, l)
	return nil
}

// drop records only an actual completed decrement, including a last decrement
// whose owning atom retirement stopped. A refused call acquires no authority.
func (v *scopedLiteralVector) drop(l sat.Literal) (done bool, pending ArithmeticAtom, err error) {
	f := v.factory
	b := l.Variable()
	var a ArithmeticAtom
	var before uint32
	if uint64(b) < uint64(len(f.bools.atoms)) {
		a = f.bools.atoms[b]
	}
	if h := arithmeticAtomHeader(a); h != nil {
		before = h.refs
	}
	err = f.decRef(b)
	if err == nil {
		return true, nil, nil
	}
	if h := arithmeticAtomHeader(a); h != nil && before > 0 && h.refs == before-1 {
		done = true
		if f.retiring == a {
			pending = a
		}
	}
	return
}
func (v *scopedLiteralVector) finishAtom(a ArithmeticAtom) error {
	f := v.factory
	if f.retiring != nil && f.retiring != a {
		return ErrAtomCachePending
	}
	if f.retiring == a {
		return f.cleanup()
	}
	return nil
}
func (v *scopedLiteralVector) set(i uint32, l sat.Literal) error {
	if v.executing || v.resetting || v.setActive {
		return ErrSearchPending
	}
	if v.factory == nil {
		return ErrSearchOwner
	}
	v.executing = true
	defer func() { v.executing = false }()
	if err := v.factory.incRef(l.Variable()); err != nil {
		return err
	}
	v.setActive, v.incomingOwned = true, true
	v.incoming, v.setIndex = l, i
	v.setOldRead, v.setOldDone = false, false
	if uint64(i) >= uint64(len(v.result)) {
		return arena.ErrBounds
	}
	v.setOldRead = true
	done, pending, err := v.drop(v.result[i])
	v.setOldDone, v.setRetiring = done, pending
	if err != nil {
		return err
	}
	v.result[i] = l
	v.incomingOwned, v.setActive = false, false
	return nil
}
func (v *scopedLiteralVector) abortSet() error {
	if !v.setActive {
		return nil
	}
	if v.setRetiring != nil {
		if err := v.finishAtom(v.setRetiring); err != nil {
			return err
		}
		v.setRetiring = nil
	}
	if v.setOldRead {
		if !v.setOldDone {
			done, pending, err := v.drop(v.result[v.setIndex])
			v.setOldDone, v.setRetiring = done, pending
			if err != nil {
				return err
			}
		}
		// The old lease has actually been consumed; no incoming lease was
		// published. Record that empty owning slot without repairing a counter.
		v.result[v.setIndex] = sat.DefaultLiteral()
	}
	if v.incomingRetiring != nil {
		if err := v.finishAtom(v.incomingRetiring); err != nil {
			return err
		}
		v.incomingRetiring = nil
	}
	if v.incomingOwned {
		done, pending, err := v.drop(v.incoming)
		v.incomingOwned = !done
		v.incomingRetiring = pending
		if err != nil {
			return err
		}
	}
	v.setActive = false
	return nil
}
func (v *scopedLiteralVector) finishRemoval() error {
	for v.resetIndex < len(v.result) || v.retiringAtom != nil {
		if v.retiringAtom != nil {
			if err := v.finishAtom(v.retiringAtom); err != nil {
				return err
			}
			v.retiringAtom = nil
			continue
		}
		done, pending, err := v.drop(v.result[v.resetIndex])
		if done {
			v.resetIndex++
		}
		v.retiringAtom = pending
		if err != nil {
			return err
		}
	}
	v.result = v.result[:v.targetSize]
	v.resetIndex = 0
	v.resetting, v.shrinking = false, false
	return nil
}
func (v *scopedLiteralVector) reset() error {
	if v.executing || v.setActive || v.shrinking {
		return ErrSearchPending
	}
	if v.factory == nil {
		return ErrSearchOwner
	}
	v.executing = true
	defer func() { v.executing = false }()
	if !v.resetting {
		v.resetting = true
		v.resetIndex = 0
		v.targetSize = 0
	}
	return v.finishRemoval()
}
func (v *scopedLiteralVector) shrink(n uint32) error {
	if v.executing || v.resetting || v.setActive {
		return ErrSearchPending
	}
	if v.factory == nil {
		return ErrSearchOwner
	}
	if err := atomAssert("nlsat_scoped_literal_vector.h", 54, "new_sz <= m_lits.size()", uint64(n) <= uint64(len(v.result))); err != nil {
		return err
	}
	if uint64(n) > uint64(len(v.result)) {
		return arena.ErrBounds
	}
	if int(n) == len(v.result) {
		return nil
	}
	v.executing = true
	defer func() { v.executing = false }()
	v.resetting, v.shrinking = true, true
	v.resetIndex, v.targetSize = int(n), int(n)
	return v.finishRemoval()
}
func (v *scopedLiteralVector) cleanup() error {
	if v.executing {
		return ErrSearchPending
	}
	if v.factory == nil {
		return ErrSearchOwner
	}
	v.executing = true
	defer func() { v.executing = false }()
	if err := v.abortSet(); err != nil {
		return err
	}
	if v.resetting {
		return v.finishRemoval()
	}
	return nil
}
func (v *scopedLiteralVector) close() error {
	if err := v.cleanup(); err != nil {
		return err
	}
	return v.reset()
}
