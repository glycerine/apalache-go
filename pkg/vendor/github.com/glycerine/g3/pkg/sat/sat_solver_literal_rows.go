package sat

// Source: src/sat/sat_solver.cpp:3119-3150,3252-3636; src/util/vector.h:389-435.

import (
	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	"reflect"
)

// Result rows own independent copies of borrowed, pointer-free literal indices.
// Native Go slices own both levels of storage, outside Runtime accounting.
type satSolverLiteralRows struct {
	self              *satSolverLiteralRows
	runtime           *umemory.Runtime
	budget            *arena.Budget
	rows              [][]Literal
	ownerBytes, epoch uint64
	closed            bool
}

func newSATSolverLiteralRows(r *umemory.Runtime, b *arena.Budget) (*satSolverLiteralRows, error) {
	if r == nil || r.Budget() != b {
		return nil, ErrSATOwner
	}
	if e := r.CheckOpen(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(satSolverLiteralRows{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	o := &satSolverLiteralRows{runtime: r, budget: b, ownerBytes: bytes}
	o.self = o
	return o, nil
}
func (o *satSolverLiteralRows) open() error {
	if o == nil || o.self != o {
		return ErrSATOwner
	}
	if o.closed {
		return arena.ErrClosed
	}
	return o.runtime.CheckOpen()
}
func (o *satSolverLiteralRows) checkRuntime(r *umemory.Runtime) error {
	if e := o.open(); e != nil {
		return e
	}
	if r != o.runtime {
		return ErrSATOwner
	}
	return nil
}
func (o *satSolverLiteralRows) rowCount() (uint32, error) {
	if e := o.open(); e != nil {
		return 0, e
	}
	return uint32(len(o.rows)), nil
}
func (o *satSolverLiteralRows) appendLiterals(input []Literal) error {
	if e := o.open(); e != nil {
		return e
	}
	if uint64(len(o.rows)) >= uint64(^uint32(0)) || uint64(len(input)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	o.rows = append(o.rows, append([]Literal(nil), input...))
	return nil
}
func (o *satSolverLiteralRows) appendUnsigned(input []uint32) error {
	if e := o.open(); e != nil {
		return e
	}
	if uint64(len(o.rows)) >= uint64(^uint32(0)) || uint64(len(input)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	row := make([]Literal, len(input))
	for i, v := range input {
		row[i] = Literal(v)
	}
	o.rows = append(o.rows, row)
	return nil
}
func (o *satSolverLiteralRows) appendColumn(src *satUintColumn) error {
	if e := o.open(); e != nil {
		return e
	}
	if src == nil || src.runtime != o.runtime {
		return ErrSATOwner
	}
	n, e := src.Size()
	if e != nil {
		return e
	}
	if uint64(len(o.rows)) >= uint64(^uint32(0)) || uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	// Grow only after successful source reads; a malformed Runtime size must
	// not trigger an oversized Go allocation before its first invalid element.
	var row []Literal
	for i := uint32(0); i < n; i++ {
		value, e := src.Read(i)
		if e != nil {
			return e
		}
		row = append(row, Literal(value))
	}
	o.rows = append(o.rows, row)
	return nil
}
func (o *satSolverLiteralRows) SourceFinalize() error {
	if e := o.open(); e != nil {
		return e
	}
	if o.epoch == ^uint64(0) {
		return arena.ErrBounds
	}
	o.rows = nil
	o.epoch++
	return nil
}
func (o *satSolverLiteralRows) CloseChecked() error {
	if o == nil || o.self != o {
		return ErrSATOwner
	}
	if o.closed {
		return nil
	}
	o.rows = nil
	o.budget.Release(o.ownerBytes)
	o.ownerBytes = 0
	o.closed = true
	return nil
}

type satSolverLiteralRowSpan struct {
	owner  *satSolverLiteralRows
	epoch  uint64
	values []Literal
	count  uint32
}

func (v satSolverLiteralRowSpan) read(i uint32) (Literal, error) {
	if v.owner == nil || v.owner.self != v.owner {
		return 0, ErrSATOwner
	}
	if v.owner.closed {
		return 0, arena.ErrClosed
	}
	if v.epoch != v.owner.epoch {
		return 0, arena.ErrInvalidSlot
	}
	if i >= v.count {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return v.values[i], nil
}
func (o *satSolverLiteralRows) captureRow(i uint32) (satSolverLiteralRowSpan, error) {
	if e := o.open(); e != nil {
		return satSolverLiteralRowSpan{}, e
	}
	if uint64(i) >= uint64(len(o.rows)) {
		if satStorageSourceDebug {
			return satSolverLiteralRowSpan{}, ErrSATAssertion
		}
		return satSolverLiteralRowSpan{}, arena.ErrBounds
	}
	row := o.rows[i]
	return satSolverLiteralRowSpan{o, o.epoch, row, uint32(len(row))}, nil
}
