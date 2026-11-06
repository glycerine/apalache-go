package lp

// Source: src/util/lp/lar_solver.h:464-473; src/test/lp.cpp:2456-2474.

func (s *LARSolver) TotalIterations() uint32 {
	return s.Core.RPrimal.TotalIterations
}

// GetListOfAllVarIndices returns column indices in heading order, including
// materialized term columns. These are not external variable or term IDs.
func (s *LARSolver) GetListOfAllVarIndices() []uint32 {
	vars := make([]uint32, len(s.Core.RHeading))
	for j := range vars {
		vars[j] = uint32(j)
	}
	return vars
}
