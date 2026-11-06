package shell

import lp "github.com/glycerine/g3/pkg/util/lp"

type LPFrontendResult struct {
	Status    lp.Status
	Objective float64
	Columns   map[string]float64
}

// Result returns a copied solution after Run. Non-optimal results have no
// objective or column map.
func (f *LPFrontend) Result() (result LPFrontendResult, err error) {
	if f == nil || f.closed || f.running || f.solver == nil {
		return result, ErrShellOptionService
	}
	result.Status = f.solver.Status
	if result.Status != lp.StatusOptimal {
		return result, nil
	}
	result.Objective, err = f.solver.GetCurrentCost()
	if err != nil {
		return result, err
	}
	result.Columns = make(map[string]float64, len(f.solver.NamesToColumns))
	for name, column := range f.solver.NamesToColumns {
		value, x := f.solver.GetColumnValue(column)
		if x != nil {
			return LPFrontendResult{}, x
		}
		result.Columns[name] = value
	}
	return result, nil
}
