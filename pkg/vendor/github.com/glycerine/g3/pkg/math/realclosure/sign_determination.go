package realclosure

// Source: src/math/realclosure/realclosure.cpp:254-281, 1743-1775,
// 1967-1978, and 2018-2090.

// signCondition is one conjunct in the persistent sign assignment chain.
// Several surviving columns may share an antecedent.
type signCondition struct {
	polynomial int
	sign       int
	previous   *signCondition
	deleted    bool
}

// signDetermination holds the concrete matrix, Tarski results, and columns
// of the real-closure root-discrimination algorithm. Polynomial coefficients
// and their value ownership will join this state with the value port.
type signDetermination struct {
	matrix      signMatrix
	tarski      []int
	conditions  []*signCondition
	polynomials int
	numRoots    int
	store       *realValueStore
	prs, qs     []realPolynomial
}

func newSignDetermination(numRoots int) *signDetermination {
	if numRoots < 2 {
		panic("realclosure: sign determination requires multiple roots")
	}
	s := &signDetermination{tarski: []int{numRoots}, conditions: []*signCondition{nil}, numRoots: numRoots}
	s.matrix.make(1, 1)
	s.matrix.at(0, 0).SetInt64(1)
	return s
}

// keepSignAssignment follows imp::keep_new_sc_assignment. At least two
// realizable children of the same old condition must survive.
func keepSignAssignment(cardinalities []int, useSquare bool) bool {
	step := 2
	if useSquare {
		step = 3
	}
	if len(cardinalities)%step != 0 {
		panic("realclosure: malformed sign cardinalities")
	}
	for i := 0; i < len(cardinalities); i += step {
		positive := 0
		for j := 0; j < step; j++ {
			if cardinalities[i+j] > 0 {
				positive++
			}
		}
		if positive >= 2 {
			return true
		}
	}
	return false
}

// advance consumes the Tarski-query results for one derivative q. The
// caller computes those results from the polynomial sequence; this method
// performs the source matrix/cardinality/column transition. A rejected q
// leaves the state intact. The returned flags are kept and all-one.
func (s *signDetermination) advance(equal, positive, negative int, expandedTarski []int) (bool, bool) {
	kept, allOne, _ := s.advanceCore(equal, positive, negative, expandedTarski)
	return kept, allOne
}

func (s *signDetermination) advanceCore(equal, positive, negative int, expandedTarski []int) (bool, bool, []int) {
	qMatrix, usefulSigns := signDetMatrix(equal, positive, negative)
	if !usefulSigns {
		return false, false, nil
	}
	var expanded signMatrix
	expanded.tensorProduct(&s.matrix, &qMatrix)
	if len(expandedTarski) != expanded.rows {
		panic("realclosure: Tarski result count differs from sign matrix")
	}
	cardinalities, solvable := expanded.solveInt(expandedTarski)
	if !solvable {
		panic("realclosure: sign matrix has no integer cardinalities")
	}
	for _, count := range cardinalities {
		if count < 0 || count > s.numRoots {
			panic("realclosure: invalid sign cardinality")
		}
	}
	useSquare := qMatrix.cols == 3
	if !keepSignAssignment(cardinalities, useSquare) {
		return false, false, nil
	}
	columns := make([]int, 0, expanded.cols)
	conditions := make([]*signCondition, 0, expanded.cols)
	allOne := true
	for col, count := range cardinalities {
		if count == 0 {
			continue
		}
		old := col / qMatrix.cols
		sign := int(qMatrix.at(1, col%qMatrix.cols).Int64())
		conditions = append(conditions, &signCondition{polynomial: s.polynomials, sign: sign, previous: s.conditions[old]})
		columns = append(columns, col)
		if count > 1 {
			allOne = false
		}
	}
	var selected signMatrix
	selected.filterColumns(&expanded, columns)
	rows := selected.independentRows(&selected)
	if len(rows) != len(columns) {
		panic("realclosure: surviving sign columns are dependent")
	}
	tarski := make([]int, len(rows))
	for i, row := range rows {
		tarski[i] = expandedTarski[row]
	}
	s.matrix = selected
	s.tarski = tarski
	s.conditions = conditions
	s.polynomials++
	return true, allOne, rows
}
