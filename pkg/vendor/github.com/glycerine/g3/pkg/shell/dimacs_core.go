package shell

// Source: src/shell/dimacs_frontend.cpp:61-108.
import (
	"fmt"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

func (f *DIMACSFrontend) trackClause(dst *sat.Solver, body []sat.Literal, assumptions *[]sat.Literal, tracking *[][]sat.Literal) error {
	v, err := dst.MkVar(true, false)
	if err != nil {
		return err
	}
	for len(*tracking) <= int(v) {
		*tracking = append(*tracking, nil)
	}
	(*tracking)[v] = append([]sat.Literal(nil), body...)
	clause := append([]sat.Literal(nil), body...)
	clause = append(clause, sat.MakeLiteral(v, true))
	if err = dst.MkClause(clause); err != nil {
		return err
	}
	*assumptions = append(*assumptions, sat.MakeLiteral(v, false))
	return nil
}
func (f *DIMACSFrontend) trackedSolver() (*sat.Solver, [][]sat.Literal, []sat.Literal, error) {
	dst, err := sat.NewSATSolver(sat.SolverConfig{ParamsContext: f.globals.Context, Params: f.params, RLimit: f.limit, Budget: f.globals.Context.Budget(), Runtime: f.runtime})
	if err != nil {
		return nil, nil, nil, err
	}
	fail := func(e error) (*sat.Solver, [][]sat.Literal, []sat.Literal, error) {
		_ = dst.Close()
		return nil, nil, nil, e
	}
	n, err := f.solver.SourceNumVars()
	if err != nil {
		return fail(err)
	}
	for v := uint32(0); v < n; v++ {
		if _, err = dst.MkVar(false, true); err != nil {
			return fail(err)
		}
	}
	var assumptions []sat.Literal
	var tracking [][]sat.Literal
	for v := uint32(1); v < n; v++ {
		value, e := f.solver.SourceValueVar(sat.BoolVar(v))
		if e != nil {
			return fail(e)
		}
		if value != lbool.Undef {
			body := []sat.Literal{sat.MakeLiteral(sat.BoolVar(v), value == lbool.False)}
			if e = f.trackClause(dst, body, &assumptions, &tracking); e != nil {
				return fail(e)
			}
		}
	}
	a := f.solver.ClauseAllocator()
	for _, ref := range f.solver.ClauseReferences(false) {
		sz, e := a.Size(ref)
		if e != nil {
			return fail(e)
		}
		body := make([]sat.Literal, 0, sz)
		for i := uint32(0); i < sz; i++ {
			lit, e := a.Literal(ref, i)
			if e != nil {
				return fail(e)
			}
			body = append(body, lit)
		}
		if e = f.trackClause(dst, body, &assumptions, &tracking); e != nil {
			return fail(e)
		}
	}
	pairs, err := f.solver.CollectBinClauses(nil, false)
	if err != nil {
		return fail(err)
	}
	for _, pair := range pairs {
		body := []sat.Literal{sat.Literal(pair.FirstIndex()), sat.Literal(pair.SecondIndex())}
		if err = f.trackClause(dst, body, &assumptions, &tracking); err != nil {
			return fail(err)
		}
	}
	return dst, tracking, assumptions, nil
}
func (f *DIMACSFrontend) writeCore(s *sat.Solver, tracking [][]sat.Literal) error {
	if _, err := io.WriteString(f.output, "core\n"); err != nil {
		return err
	}
	core, err := s.AppendCore(nil)
	if err != nil {
		return err
	}
	for _, marker := range core {
		v := marker.Variable()
		if uint64(v) >= uint64(len(tracking)) {
			return sat.ErrSATAssertion
		}
		for _, lit := range tracking[v] {
			if _, err = fmt.Fprintf(f.output, "%s ", lit.String()); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(f.output, "\n"); err != nil {
			return err
		}
	}
	return nil
}
