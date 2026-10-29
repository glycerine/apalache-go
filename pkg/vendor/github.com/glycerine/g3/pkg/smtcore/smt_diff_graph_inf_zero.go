package smtcore

// Source: src/smt/diff_logic.h tight-edge SCC, successors, path search,
// assignment adjustment and paired zero normalization used by UTVPI.

// ComputeZeroEdgeSCC runs the source Gabow-style DFS over enabled edges whose
// normalized weight is zero. Singleton components retain the source -1 ID.
func (g *SMTInfRationalDiffGraph) ComputeZeroEdgeSCC() ([]int32, error) {
	if g == nil || !g.IsFeasible() {
		return nil, errSMTDiffGraphDomain
	}
	count := len(g.assignment)
	dfsTime := make([]int32, count)
	component := make([]int32, count)
	unfinishedSet := make([]bool, count)
	for index := range dfsTime {
		dfsTime[index], component[index] = -1, -1
	}
	roots := make([]int32, 0, count)
	unfinished := make([]int32, 0, count)
	nextTime, nextComponent := int32(0), int32(0)
	var visit func(int32)
	visit = func(variable int32) {
		dfsTime[variable] = nextTime
		nextTime++
		unfinishedSet[variable] = true
		unfinished = append(unfinished, variable)
		roots = append(roots, variable)
		for _, edgeID := range g.outEdges[variable] {
			edge := &g.edges[edgeID]
			if !edge.enabled {
				continue
			}
			var gamma smtInfRational
			g.setGamma(edge, &gamma)
			if gamma.Sign() != 0 {
				continue
			}
			target := edge.target
			if dfsTime[target] == -1 {
				visit(target)
			} else if unfinishedSet[target] {
				for dfsTime[roots[len(roots)-1]] > dfsTime[target] {
					roots = roots[:len(roots)-1]
				}
			}
		}
		if variable == roots[len(roots)-1] {
			size := 0
			last := int32(-1)
			for last != variable {
				last = unfinished[len(unfinished)-1]
				unfinished = unfinished[:len(unfinished)-1]
				unfinishedSet[last] = false
				component[last] = nextComponent
				size++
			}
			if size == 1 {
				component[last] = -1
			} else {
				nextComponent++
			}
			roots = roots[:len(roots)-1]
		}
	}
	for variable := int32(0); int(variable) < count; variable++ {
		if dfsTime[variable] == -1 {
			visit(variable)
		}
	}
	return component, nil
}

func (g *SMTInfRationalDiffGraph) ComputeZeroSuccessors(variable int32) ([]int32, error) {
	if !g.validVariable(variable) {
		return nil, errSMTDiffGraphDomain
	}
	seen := make([]bool, len(g.assignment))
	seen[variable] = true
	result := []int32{variable}
	for index := 0; index < len(result); index++ {
		current := result[index]
		for _, edgeID := range g.outEdges[current] {
			edge := &g.edges[edgeID]
			if !edge.enabled {
				continue
			}
			var gamma smtInfRational
			g.setGamma(edge, &gamma)
			if gamma.Sign() == 0 && !seen[edge.target] {
				seen[edge.target] = true
				result = append(result, edge.target)
			}
		}
	}
	return result, nil
}

type smtInfRationalDiffBFSElement struct {
	variable    int32
	parentIndex int
	edgeID      int32
}

// FindShortestZeroEdgePath performs the source breadth-first search over
// enabled normalized-zero edges. The retained UTVPI caller uses UINT_MAX, so
// no timestamp filter is needed. Edge identifiers follow the
// source callback order: final edge first, then predecessors toward source.
func (g *SMTInfRationalDiffGraph) FindShortestZeroEdgePath(source, target int32) ([]int32, bool, error) {
	if !g.validVariable(source) || !g.validVariable(target) {
		return nil, false, errSMTDiffGraphDomain
	}
	queue := []smtInfRationalDiffBFSElement{{variable: source, parentIndex: -1, edgeID: -1}}
	marked := make([]bool, len(g.assignment))
	marked[source] = true
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		for _, edgeID := range g.outEdges[current.variable] {
			edge := &g.edges[edgeID]
			if !edge.enabled {
				continue
			}
			var gamma smtInfRational
			g.setGamma(edge, &gamma)
			if gamma.Sign() != 0 {
				continue
			}
			if edge.target == target {
				result := []int32{edgeID}
				parent := head
				for parent >= 0 && queue[parent].edgeID != -1 {
					result = append(result, queue[parent].edgeID)
					parent = queue[parent].parentIndex
				}
				return result, true, nil
			}
			if !marked[edge.target] {
				marked[edge.target] = true
				queue = append(queue, smtInfRationalDiffBFSElement{variable: edge.target, parentIndex: head, edgeID: edgeID})
			}
		}
	}
	return nil, false, nil
}

func (g *SMTInfRationalDiffGraph) IncrementAssignment(variable int32, increment *smtInfRational) error {
	if !g.validVariable(variable) || increment == nil {
		return errSMTDiffGraphDomain
	}
	g.assignment[variable].Add(&g.assignment[variable], increment)
	return nil
}

func (g *SMTInfRationalDiffGraph) SetAssignment(variable int32, value *smtInfRational) error {
	if !g.validVariable(variable) || value == nil {
		return errSMTDiffGraphDomain
	}
	g.assignment[variable].Set(value)
	return nil
}

// SetPairToZero implements the source helper for disconnected variables. If
// translation does not zero both values, zero-weight edges join the components.
func (g *SMTInfRationalDiffGraph) SetPairToZero(first, second int32) error {
	if !g.validVariable(first) || !g.validVariable(second) || !g.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	selected := second
	if g.assignment[first].Sign() != 0 {
		selected = first
	}
	if err := g.SetToZero(selected); err != nil {
		return err
	}
	if g.assignment[first].Sign() == 0 && g.assignment[second].Sign() == 0 {
		return nil
	}
	zero := new(smtInfRational)
	forward, err := g.AddEdge(first, second, zero, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, err := g.EnableEdge(forward); err != nil || !feasible {
		if err != nil {
			return err
		}
		return errSMTDiffGraphDomain
	}
	backward, err := g.AddEdge(second, first, zero, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, err := g.EnableEdge(backward); err != nil || !feasible {
		if err != nil {
			return err
		}
		return errSMTDiffGraphDomain
	}
	return nil
}
