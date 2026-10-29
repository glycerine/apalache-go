package sat

// Source: src/sat/sat_solver.cpp:2773-2779,3652-3714; src/sat/sat_solver.h:159,164.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func collectSolverParamDescrs(d *uparams.ParamDescrs) error {
	if e := collectSATConfigParamDescrs(d); e != nil {
		return e
	}
	if e := collectSimplifierParamDescrs(d); e != nil {
		return e
	}
	if e := collectAsymmBranchParamDescrs(d); e != nil {
		return e
	}
	satProbingCollectParamDescriptions(d)
	return collectSCCParamDescrs(d)
}
func satStatusUint(out *utext.NumericTextStream, key string, n uint32) error {
	if e := out.AppendCString(key); e != nil {
		return e
	}
	if e := out.AppendUint32(n); e != nil {
		return e
	}
	return out.AppendCString("\n")
}
func (s *Solver) displayStatus(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	var numBin, numExt, numLits uint32
	for idx := 0; idx < len(s.Variables.watches.lists); idx++ {
		l := Literal(uint32(idx)).Negated()
		list, e := s.Variables.watches.Get(uint32(idx))
		if e != nil {
			return e
		}
		r, e := list.ref()
		if e != nil {
			return e
		}
		it, e := s.Variables.watches.store.Iter(r)
		if e != nil {
			return e
		}
		for {
			w, yes, e := it.Next()
			if e != nil {
				return e
			}
			if !yes {
				break
			}
			switch w.Kind() {
			case 0:
				l2, e := w.Literal()
				if e != nil {
					return e
				}
				if l.index() < l2.index() {
					numLits += 2
					numBin++
				}
			case 3:
				numExt++
			}
		}
	}
	_ = numExt // actual source computes but never prints this counter.
	var numElim uint32
	for v := uint32(0); ; v++ {
		n, e := s.Variables.numVarsChecked()
		if e != nil {
			return e
		}
		if v >= n {
			break
		}
		x, e := s.Variables.wasEliminated(BoolVar(v))
		if e != nil {
			return e
		}
		if x {
			numElim++
		}
	}
	var numTer, numCls uint32
	for _, cs := range [2][]StoredClauseRef{s.Lifecycle.Original, s.Lifecycle.learned} {
		for _, c := range cs {
			n, e := s.Clauses.Size(c)
			if e != nil {
				return e
			}
			if n == 3 {
				numTer++
			} else {
				numCls++
			}
			n, e = s.Clauses.Size(c)
			if e != nil {
				return e
			}
			numLits += n
		}
	}
	total := numCls + numTer + numBin
	bytes, e := s.runtime.memory.AllocationSize()
	if e != nil {
		return e
	}
	mem := float64(bytes) / float64(1024*1024)
	if e = out.AppendCString("(sat-status\n  :inconsistent    "); e != nil {
		return e
	}
	text := "false"
	if s.propagation.inconsistent {
		text = "true"
	}
	if e = out.AppendCString(text); e != nil {
		return e
	}
	if e = out.AppendCString("\n  :vars            "); e != nil {
		return e
	}
	n, e := s.Variables.numVarsChecked()
	if e != nil {
		return e
	}
	if e = out.AppendUint32(n); e != nil {
		return e
	}
	if e = out.AppendCString("\n"); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :elim-vars       ", numElim); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :lits            ", numLits); e != nil {
		return e
	}
	if e = out.AppendCString("  :assigned        "); e != nil {
		return e
	}
	n, e = s.Variables.trail.Size(s.Variables.trailRef)
	if e != nil {
		return e
	}
	if e = out.AppendUint32(n); e != nil {
		return e
	}
	if e = out.AppendCString("\n"); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :binary-clauses  ", numBin); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :ternary-clauses ", numTer); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :clauses         ", numCls); e != nil {
		return e
	}
	if e = satStatusUint(out, "  :del-clause      ", s.Lifecycle.deleted); e != nil {
		return e
	}
	if e = out.AppendCString("  :avg-clause-size "); e != nil {
		return e
	}
	avg := 0.0
	if total != 0 {
		avg = float64(numLits) / float64(total)
	}
	if e = out.AppendFloat64(avg); e != nil {
		return e
	}
	if e = out.AppendCString("\n  :memory          "); e != nil {
		return e
	}
	if e = out.SetFloatMode(utext.NumericFixedFloat); e != nil {
		return e
	}
	if e = out.SetPrecision(2); e != nil {
		return e
	}
	if e = out.AppendFloat64(mem); e != nil {
		return e
	}
	return out.AppendCString(")\n") // std::endl flush has no external sink in the borrowed memory-stream adapter.
}
