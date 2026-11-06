package smtcore

// Source: src/smt/theory_utvpi_def.h add_ineq, enable_edge,
// check_z_consistency, is_parity_ok and enforce_parity (rdl_ext).

import "math/big"

// A source UTVPI variable has two graph vertices: positive 2*v and negative
// 2*v+1. Its value is half their potential difference. Edge coefficients are
// kept separately from literals so unary constraints retain their factor two
// in conflict explanations.
type smtUTVPIGraph struct {
	graph        *SMTInfRationalDiffGraph
	integer      []bool
	coefficients []uint32
	scopes       []int
}

type smtUTVPIGraphTerm struct {
	variable int32
	negative bool
}

func newSMTUTVPIGraph() *smtUTVPIGraph {
	return &smtUTVPIGraph{graph: NewSMTInfRationalDiffGraph()}
}

func (g *smtUTVPIGraph) addVariable(integer bool) (int32, error) {
	v := int32(len(g.integer))
	if err := g.graph.InitVariable(2 * v); err != nil {
		return -1, err
	}
	if err := g.graph.InitVariable(2*v + 1); err != nil {
		return -1, err
	}
	g.integer = append(g.integer, integer)
	return v, nil
}

// addInequality encodes sum(terms)+weight <= 0 as two adjacent edges.
// Terms have already been combined and checked by the UTVPI linearizer.
func (g *smtUTVPIGraph) addInequality(terms []smtUTVPIGraphTerm, weight *smtInfRational, literal SMTLiteral) (int32, error) {
	if weight == nil || len(terms) < 1 || len(terms) > 2 {
		return -1, errSMTDiffGraphDomain
	}
	for _, term := range terms {
		if term.variable < 0 || int(term.variable) >= len(g.integer) {
			return -1, errSMTDiffGraphDomain
		}
	}
	if len(terms) == 2 && terms[0].variable == terms[1].variable {
		return -1, errSMTDiffGraphDomain
	}
	first := 2 * terms[0].variable
	if terms[0].negative {
		first++
	}
	var edgeWeight smtInfRational
	edgeWeight.Neg(weight)
	source, target := first^1, first
	otherSource, otherTarget := source, target
	coefficient := uint32(2)
	if len(terms) == 1 {
		edgeWeight.Add(&edgeWeight, &edgeWeight)
	} else {
		second := 2 * terms[1].variable
		if terms[1].negative {
			second++
		}
		source, target = second^1, first
		otherSource, otherTarget = first^1, second
		if terms[0].negative && terms[1].negative {
			source, target, otherSource, otherTarget = otherSource, otherTarget, source, target
		}
		coefficient = 1
	}
	// Popped graph edges can reuse their IDs; discard matching stale weights.
	if len(g.coefficients) > len(g.graph.edges) {
		g.coefficients = g.coefficients[:len(g.graph.edges)]
	}
	for len(g.coefficients) < len(g.graph.edges) {
		// Model normalization can add unconditional, zero-weight edges.
		g.coefficients = append(g.coefficients, 0)
	}
	id, err := g.graph.AddEdge(source, target, &edgeWeight, literal)
	if err != nil {
		return -1, err
	}
	g.coefficients = append(g.coefficients, coefficient)
	if _, err := g.graph.AddEdge(otherSource, otherTarget, &edgeWeight, literal); err != nil {
		return -1, err
	}
	g.coefficients = append(g.coefficients, coefficient)
	return id, nil
}

func (g *smtUTVPIGraph) enablePair(id int32) (bool, error) {
	if !g.graph.validEdge(id) || !g.graph.validEdge(id+1) {
		return false, errSMTDiffGraphDomain
	}
	ok, err := g.graph.EnableEdge(id)
	if err != nil || !ok {
		return ok, err
	}
	return g.graph.EnableEdge(id + 1)
}

func (g *smtUTVPIGraph) parityOK(variable int32) (bool, error) {
	if variable < 0 || int(variable) >= len(g.integer) {
		return false, errSMTDiffGraphDomain
	}
	first := &g.graph.assignment[2*variable].rational
	second := &g.graph.assignment[2*variable+1].rational
	// rational::is_even is false for non-integral rationals. Real vertices
	// can be visited while repairing a neighboring integer's parity.
	firstEven := first.IsInt() && first.Num().Bit(0) == 0
	secondEven := second.IsInt() && second.Num().Bit(0) == 0
	return firstEven == secondEven, nil
}

// parityConflict returns the two shortest tight paths witnessing an odd
// signed pair in one SCC. A feasible rational graph can still be integer UNSAT.
func (g *smtUTVPIGraph) parityConflict() ([]int32, error) {
	components, err := g.graph.ComputeZeroEdgeSCC()
	if err != nil {
		return nil, err
	}
	for variable, integer := range g.integer {
		if !integer {
			continue
		}
		if !g.graph.assignment[2*variable].rational.IsInt() || !g.graph.assignment[2*variable+1].rational.IsInt() {
			return nil, errSMTDiffGraphDomain
		}
		ok, err := g.parityOK(int32(variable))
		if err != nil {
			return nil, err
		}
		first := int32(2 * variable)
		second := first + 1
		if ok || components[first] < 0 || components[first] != components[second] {
			continue
		}
		forward, found, err := g.graph.FindShortestZeroEdgePath(first, second)
		if err != nil || !found {
			return nil, errSMTDiffGraphDomain
		}
		backward, found, err := g.graph.FindShortestZeroEdgePath(second, first)
		if err != nil || !found {
			return nil, errSMTDiffGraphDomain
		}
		return append(forward, backward...), nil
	}
	return nil, nil
}

// enforceParity requires successful parityConflict first. Decrementing the
// entire tight successor set preserves every enabled graph inequality.
func (g *smtUTVPIGraph) enforceParity() error {
	todo := []int32{}
	for variable, integer := range g.integer {
		if !integer {
			continue
		}
		ok, err := g.parityOK(int32(variable))
		if err != nil {
			return err
		}
		if !ok {
			todo = append(todo, int32(variable))
		}
	}
	var decrement smtInfRational
	decrement.rational.SetInt64(-1)
	for len(todo) > 0 {
		variable := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		ok, err := g.parityOK(variable)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		first, second := 2*variable, 2*variable+1
		successors, err := g.graph.ComputeZeroSuccessors(first)
		if err != nil {
			return err
		}
		for _, vertex := range successors {
			if vertex == second {
				successors, err = g.graph.ComputeZeroSuccessors(second)
				if err != nil {
					return err
				}
				break
			}
		}
		for _, vertex := range successors {
			if err := g.graph.IncrementAssignment(vertex, &decrement); err != nil {
				return err
			}
			ok, err := g.parityOK(vertex / 2)
			if err != nil {
				return err
			}
			if !ok {
				todo = append(todo, vertex/2)
			}
		}
	}
	if !g.graph.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	return nil
}

func (g *smtUTVPIGraph) push() error {
	if err := g.graph.Push(); err != nil {
		return err
	}
	g.scopes = append(g.scopes, len(g.integer))
	return nil
}

func (g *smtUTVPIGraph) pop(count uint32) error {
	if count == 0 || int(count) > len(g.scopes) {
		return errSMTDiffGraphDomain
	}
	level := len(g.scopes) - int(count)
	if err := g.graph.Pop(count); err != nil {
		return err
	}
	g.integer = g.integer[:g.scopes[level]]
	g.scopes = g.scopes[:level]
	if len(g.coefficients) > len(g.graph.edges) {
		g.coefficients = g.coefficients[:len(g.graph.edges)]
	}
	return nil
}

// modelValues implements the graph part of init_model, compute_delta and
// mk_value. The distinguished zero variable is created by the theory owner.
// Values own their rationals. Source only debug-asserts integer-valued Int
// assignments here; its arithmetic factory performs the release rejection.
func (g *smtUTVPIGraph) modelValues(zeroVariable int32) ([]big.Rat, error) {
	if zeroVariable < 0 || int(zeroVariable) >= len(g.integer) {
		return nil, errSMTDiffGraphDomain
	}
	conflict, err := g.parityConflict()
	if err != nil {
		return nil, err
	}
	if len(conflict) != 0 {
		return nil, errSMTDiffGraphDomain
	}
	if err := g.enforceParity(); err != nil {
		return nil, err
	}
	if err := g.graph.SetPairToZero(2*zeroVariable, 2*zeroVariable+1); err != nil {
		return nil, err
	}
	delta := big.NewRat(1, 1)
	for i := range g.graph.edges {
		edge := &g.graph.edges[i]
		if !edge.enabled {
			continue
		}
		var bound smtInfRational
		bound.Sub(&g.graph.assignment[edge.target], &g.graph.assignment[edge.source])
		bound.Sub(&bound, &edge.weight)
		if bound.Sign() > 0 {
			return nil, errSMTDiffGraphDomain
		}
		if bound.infinitesimal.Sign() > 0 {
			limit := new(big.Rat).Neg(&bound.rational)
			limit.Quo(limit, &bound.infinitesimal)
			if limit.Cmp(delta) < 0 {
				delta.Set(limit)
			}
		}
	}
	values := make([]big.Rat, len(g.integer))
	for variable := range g.integer {
		var difference smtInfRational
		difference.Sub(&g.graph.assignment[2*variable], &g.graph.assignment[2*variable+1])
		values[variable].Mul(delta, &difference.infinitesimal)
		values[variable].Add(&values[variable], &difference.rational)
		values[variable].Quo(&values[variable], big.NewRat(2, 1))
	}
	return values, nil
}
