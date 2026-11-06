package nlsat

// Source: src/nlsat/nlsat_scoped_literal_vector.h:27-75.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

// LiteralVector owns references to its solver's arithmetic literals. It is
// intended for projected clauses and other results that must remain valid
// after the operation that constructed them returns.
type LiteralVector struct {
	self   *LiteralVector
	solver *Solver
	values scopedLiteralVector
	closed bool
}

func NewLiteralVector(solver *Solver) (*LiteralVector, error) {
	if err := solver.open(); err != nil {
		return nil, err
	}
	v := &LiteralVector{solver: solver}
	v.self = v
	v.values.factory = solver.atoms
	solver.literalVectors++
	return v, nil
}

func (v *LiteralVector) owner() error {
	if v == nil || v.self != v || v.solver == nil {
		return ErrSearchOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	if err := v.solver.open(); err != nil {
		return err
	}
	if v.values.factory != v.solver.atoms {
		return ErrSearchOwner
	}
	return nil
}

func (v *LiteralVector) Len() int {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return len(v.values.result)
}

func (v *LiteralVector) Literal(index uint32) (sat.Literal, error) {
	if err := v.owner(); err != nil {
		return sat.DefaultLiteral(), err
	}
	if uint64(index) >= uint64(len(v.values.result)) {
		return sat.DefaultLiteral(), arena.ErrBounds
	}
	return v.values.result[index], nil
}

// Literals replaces out with a borrowed-value copy. The owning vector must
// remain open while any copied arithmetic literal is used.
func (v *LiteralVector) Literals(out *[]sat.Literal) error {
	if err := v.owner(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = append((*out)[:0], v.values.result...)
	return nil
}

func (v *LiteralVector) Push(literal sat.Literal) error {
	if err := v.owner(); err != nil {
		return err
	}
	return v.values.push(literal)
}

func (v *LiteralVector) Set(index uint32, literal sat.Literal) error {
	if err := v.owner(); err != nil {
		return err
	}
	return v.values.set(index, literal)
}

func (v *LiteralVector) Reset() error {
	if err := v.owner(); err != nil {
		return err
	}
	return v.values.reset()
}

func (v *LiteralVector) Close() error {
	if v == nil || v.self != v {
		return ErrSearchOwner
	}
	if v.closed {
		return nil
	}
	if v.solver == nil || v.values.factory != v.solver.atoms {
		return ErrSearchOwner
	}
	if err := v.solver.open(); err != nil {
		return err
	}
	if err := v.values.close(); err != nil {
		return err
	}
	v.solver.literalVectors--
	v.closed = true
	v.solver = nil
	v.values.factory = nil
	return nil
}
