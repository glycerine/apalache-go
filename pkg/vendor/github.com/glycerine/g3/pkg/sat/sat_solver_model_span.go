package sat

// Source: src/sat/sat_types.h:138; src/util/lbool.h; src/util/vector.h:407–420; src/sat/sat_model_converter.cpp:35–114.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Concrete model& borrow. Native reads resolve the current member backing at
// every operator[]; the preexisting caller-owned slice route is unchanged.
// The source lbool enum is signed32, unlike the solver's char assignments.
type satSolverModelSpan struct {
	slice  Model
	column *satUintColumn
}

func (m satSolverModelSpan) read(v BoolVar) (lbool.LBool, error) {
	if m.column != nil {
		x, e := m.column.Read(uint32(v))
		return lbool.LBool(int32(x)), e
	}
	if uint64(v) >= uint64(len(m.slice)) {
		return lbool.Undef, arena.ErrBounds
	}
	return m.slice[v], nil
}
func (m satSolverModelSpan) write(v BoolVar, x lbool.LBool) error {
	if m.column != nil {
		return m.column.Write(uint32(v), uint32(int32(x)))
	}
	if uint64(v) >= uint64(len(m.slice)) {
		return arena.ErrBounds
	}
	m.slice[v] = x
	return nil
}
func (m satSolverModelSpan) value(l Literal) (lbool.LBool, error) {
	v, e := m.read(l.variable())
	if e != nil {
		return lbool.Undef, e
	}
	if l.sign() {
		v = v.Negated()
	}
	return v, nil
}
func satSolverResizeModel(c *satUintColumn, n uint32) error {
	return c.resize(n, uint32(lbool.Undef))
}
