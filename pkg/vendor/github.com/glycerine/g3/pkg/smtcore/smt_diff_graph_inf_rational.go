package smtcore

// Source: src/smt/diff_logic.h concrete inf_int_rational graph fragment used
// by real difference logic.

import "math/big"

// smtInfRational represents a + b*epsilon, ordered lexicographically with a
// positive infinitesimal epsilon. Values own their big.Rat components.
type smtInfRational struct {
	rational, infinitesimal big.Rat
}

func newSMTInfRational(rational, infinitesimal *big.Rat) smtInfRational {
	var result smtInfRational
	if rational != nil {
		result.rational.Set(rational)
	}
	if infinitesimal != nil {
		result.infinitesimal.Set(infinitesimal)
	}
	return result
}

func (v *smtInfRational) Set(other *smtInfRational) *smtInfRational {
	v.rational.Set(&other.rational)
	v.infinitesimal.Set(&other.infinitesimal)
	return v
}

func (v *smtInfRational) Add(left, right *smtInfRational) *smtInfRational {
	v.rational.Add(&left.rational, &right.rational)
	v.infinitesimal.Add(&left.infinitesimal, &right.infinitesimal)
	return v
}

func (v *smtInfRational) Sub(left, right *smtInfRational) *smtInfRational {
	v.rational.Sub(&left.rational, &right.rational)
	v.infinitesimal.Sub(&left.infinitesimal, &right.infinitesimal)
	return v
}

func (v *smtInfRational) Neg(other *smtInfRational) *smtInfRational {
	v.rational.Neg(&other.rational)
	v.infinitesimal.Neg(&other.infinitesimal)
	return v
}

func (v *smtInfRational) Cmp(other *smtInfRational) int {
	if result := v.rational.Cmp(&other.rational); result != 0 {
		return result
	}
	return v.infinitesimal.Cmp(&other.infinitesimal)
}

func (v *smtInfRational) Sign() int {
	var zero smtInfRational
	return v.Cmp(&zero)
}

type smtInfRationalDiffEdge struct {
	source, target int32
	weight         smtInfRational
	explanation    SMTLiteral
	timestamp      uint32
	enabled        bool
}

type smtInfRationalDiffAssignmentTrail struct {
	variable int32
	oldValue smtInfRational
}

type smtInfRationalDiffScope struct {
	edges, enabled int
	timestamp      uint32
}

// SMTInfRationalDiffGraph is the retained infinitesimal-rational dl_graph
// instantiation. It contains only operations used by real difference logic.
type SMTInfRationalDiffGraph struct {
	assignment      []smtInfRational
	assignmentTrail []smtInfRationalDiffAssignmentTrail
	edges           []smtInfRationalDiffEdge
	outEdges        [][]int32
	scopes          []smtInfRationalDiffScope
	gamma           []smtInfRational
	mark            []smtDiffSearchMark
	parent          []int32
	visited         []int32
	heap            smtInfRationalDiffHeap
	lastEnabledEdge int32
	enabledEdges    []int32
	propagationCost uint32
	timestamp       uint32
}

func NewSMTInfRationalDiffGraph() *SMTInfRationalDiffGraph {
	g := &SMTInfRationalDiffGraph{lastEnabledEdge: -1}
	g.heap.graph = g
	return g
}

func (g *SMTInfRationalDiffGraph) validVariable(v int32) bool {
	return g != nil && v >= 0 && int(v) < len(g.assignment)
}

func (g *SMTInfRationalDiffGraph) validEdge(id int32) bool {
	return g != nil && id >= 0 && int(id) < len(g.edges)
}

func (g *SMTInfRationalDiffGraph) InitVariable(v int32) error {
	if g == nil || v < 0 {
		return errSMTDiffGraphDomain
	}
	for int(v) >= len(g.assignment) {
		g.assignment = append(g.assignment, smtInfRational{})
		g.outEdges = append(g.outEdges, nil)
		g.gamma = append(g.gamma, smtInfRational{})
		g.mark = append(g.mark, smtDiffUnmarked)
		g.parent = append(g.parent, -1)
	}
	if len(g.outEdges[v]) != 0 || g.mark[v] != smtDiffUnmarked {
		return errSMTDiffGraphDomain
	}
	g.assignment[v] = smtInfRational{}
	return nil
}

func (g *SMTInfRationalDiffGraph) AddEdge(source, target int32, weight *smtInfRational, explanation SMTLiteral) (int32, error) {
	if !g.validVariable(source) || !g.validVariable(target) || weight == nil {
		return -1, errSMTDiffGraphDomain
	}
	id := int32(len(g.edges))
	edge := smtInfRationalDiffEdge{source: source, target: target, explanation: explanation, timestamp: g.timestamp}
	edge.weight.Set(weight)
	g.edges = append(g.edges, edge)
	g.outEdges[source] = append(g.outEdges[source], id)
	return id, nil
}

func (g *SMTInfRationalDiffGraph) edgeFeasible(edge *smtInfRationalDiffEdge) bool {
	if !edge.enabled {
		return true
	}
	var difference smtInfRational
	difference.Sub(&g.assignment[edge.target], &g.assignment[edge.source])
	return difference.Cmp(&edge.weight) <= 0
}

func (g *SMTInfRationalDiffGraph) IsFeasible() bool {
	if g == nil {
		return false
	}
	for index := range g.edges {
		if !g.edgeFeasible(&g.edges[index]) {
			return false
		}
	}
	return true
}

func (g *SMTInfRationalDiffGraph) setGamma(edge *smtInfRationalDiffEdge, result *smtInfRational) {
	result.Sub(&g.assignment[edge.source], &g.assignment[edge.target])
	result.Add(result, &edge.weight)
}

func (g *SMTInfRationalDiffGraph) accumulateAssignment(variable int32, increment *smtInfRational) {
	trail := smtInfRationalDiffAssignmentTrail{variable: variable}
	trail.oldValue.Set(&g.assignment[variable])
	g.assignmentTrail = append(g.assignmentTrail, trail)
	g.assignment[variable].Add(&g.assignment[variable], increment)
}

func (g *SMTInfRationalDiffGraph) undoAssignments() {
	for index := len(g.assignmentTrail); index > 0; index-- {
		trail := &g.assignmentTrail[index-1]
		g.assignment[trail.variable].Set(&trail.oldValue)
	}
	g.assignmentTrail = g.assignmentTrail[:0]
}

func (g *SMTInfRationalDiffGraph) resetMarks() {
	for _, variable := range g.visited {
		g.mark[variable] = smtDiffUnmarked
	}
	g.visited = g.visited[:0]
}

func (g *SMTInfRationalDiffGraph) makeFeasible(id int32) bool {
	edge := &g.edges[id]
	root, target := edge.source, edge.target
	g.gamma[root] = smtInfRational{}
	g.setGamma(edge, &g.gamma[target])
	g.mark[target] = smtDiffProcessed
	g.parent[target] = id
	g.visited = append(g.visited, target)
	g.accumulateAssignment(target, &g.gamma[target])

	source := target
	for {
		g.propagationCost++
		if g.mark[root] != smtDiffUnmarked {
			g.heap.reset()
			g.resetMarks()
			g.undoAssignments()
			return false
		}
		for _, edgeID := range g.outEdges[source] {
			candidate := &g.edges[edgeID]
			if !candidate.enabled {
				continue
			}
			var gamma smtInfRational
			g.setGamma(candidate, &gamma)
			if gamma.Sign() >= 0 {
				continue
			}
			next := candidate.target
			switch g.mark[next] {
			case smtDiffUnmarked:
				g.gamma[next].Set(&gamma)
				g.mark[next] = smtDiffFound
				g.parent[next] = edgeID
				g.visited = append(g.visited, next)
				g.heap.insert(next)
			case smtDiffFound:
				if gamma.Cmp(&g.gamma[next]) < 0 {
					g.gamma[next].Set(&gamma)
					g.parent[next] = edgeID
					g.heap.decreased(next)
				}
			}
		}
		if g.heap.empty() {
			g.resetMarks()
			g.assignmentTrail = g.assignmentTrail[:0]
			return true
		}
		source = g.heap.eraseMin()
		g.mark[source] = smtDiffProcessed
		g.accumulateAssignment(source, &g.gamma[source])
	}
}

func (g *SMTInfRationalDiffGraph) EnableEdge(id int32) (bool, error) {
	if !g.validEdge(id) {
		return false, errSMTDiffGraphDomain
	}
	edge := &g.edges[id]
	if edge.enabled {
		return true, nil
	}
	edge.enabled = true
	edge.timestamp = g.timestamp
	g.timestamp++
	g.lastEnabledEdge = id
	feasible := true
	if !g.edgeFeasible(edge) {
		feasible = g.makeFeasible(id)
	}
	g.enabledEdges = append(g.enabledEdges, id)
	return feasible, nil
}

func (g *SMTInfRationalDiffGraph) NegativeCycleEdges() ([]int32, error) {
	if !g.validEdge(g.lastEnabledEdge) || g.edgeFeasible(&g.edges[g.lastEnabledEdge]) {
		return nil, errSMTDiffGraphDomain
	}
	last := g.lastEnabledEdge
	result := make([]int32, 0, 4)
	id := last
	for steps := 0; ; steps++ {
		if steps > len(g.edges) || !g.validEdge(id) {
			return nil, errSMTDiffGraphDomain
		}
		result = append(result, id)
		id = g.parent[g.edges[id].source]
		if id == last {
			break
		}
	}
	return result, nil
}

func (g *SMTInfRationalDiffGraph) NegativeCycleExplanations() ([]SMTLiteral, error) {
	edges, err := g.negativeCycleEdgesWithShortcuts()
	if err != nil {
		return nil, err
	}
	result := make([]SMTLiteral, len(edges))
	for i, id := range edges {
		result[i] = g.edges[id].explanation
	}
	return result, nil
}

func (g *SMTInfRationalDiffGraph) Push() error {
	if g == nil {
		return errSMTDiffGraphDomain
	}
	g.scopes = append(g.scopes, smtInfRationalDiffScope{len(g.edges), len(g.enabledEdges), g.timestamp})
	return nil
}

func (g *SMTInfRationalDiffGraph) Pop(count uint32) error {
	if g == nil || count == 0 || int(count) > len(g.scopes) {
		return errSMTDiffGraphDomain
	}
	level := len(g.scopes) - int(count)
	scope := g.scopes[level]
	for index := len(g.enabledEdges); index > scope.enabled; index-- {
		g.edges[g.enabledEdges[index-1]].enabled = false
	}
	g.enabledEdges = g.enabledEdges[:scope.enabled]
	g.timestamp = scope.timestamp
	for len(g.edges) > scope.edges {
		id := int32(len(g.edges) - 1)
		edge := &g.edges[id]
		g.outEdges[edge.source] = g.outEdges[edge.source][:len(g.outEdges[edge.source])-1]
		g.edges = g.edges[:id]
	}
	g.scopes = g.scopes[:level]
	return nil
}

func (g *SMTInfRationalDiffGraph) Assignment(variable int32) (*smtInfRational, error) {
	if !g.validVariable(variable) {
		return nil, errSMTDiffGraphDomain
	}
	return new(smtInfRational).Set(&g.assignment[variable]), nil
}

func (g *SMTInfRationalDiffGraph) SetToZero(variable int32) error {
	if !g.validVariable(variable) || !g.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	if g.assignment[variable].Sign() == 0 {
		return nil
	}
	var offset smtInfRational
	offset.Set(&g.assignment[variable])
	for index := range g.assignment {
		g.assignment[index].Sub(&g.assignment[index], &offset)
	}
	return nil
}

func (g *SMTInfRationalDiffGraph) NumNodes() int {
	if g == nil {
		return 0
	}
	return len(g.assignment)
}

func (g *SMTInfRationalDiffGraph) NumEdges() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

type smtInfRationalDiffHeap struct {
	graph     *SMTInfRationalDiffGraph
	variables []int32
	positions []int
}

func (h *smtInfRationalDiffHeap) empty() bool { return len(h.variables) == 0 }
func (h *smtInfRationalDiffHeap) ensure(v int32) {
	for int(v) >= len(h.positions) {
		h.positions = append(h.positions, -1)
	}
}
func (h *smtInfRationalDiffHeap) less(i, j int) bool {
	return h.graph.gamma[h.variables[i]].Cmp(&h.graph.gamma[h.variables[j]]) < 0
}
func (h *smtInfRationalDiffHeap) swap(i, j int) {
	h.variables[i], h.variables[j] = h.variables[j], h.variables[i]
	h.positions[h.variables[i]], h.positions[h.variables[j]] = i, j
}
func (h *smtInfRationalDiffHeap) up(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if !h.less(index, parent) {
			break
		}
		h.swap(index, parent)
		index = parent
	}
}
func (h *smtInfRationalDiffHeap) down(index int) {
	for {
		child := index*2 + 1
		if child >= len(h.variables) {
			return
		}
		if child+1 < len(h.variables) && h.less(child+1, child) {
			child++
		}
		if !h.less(child, index) {
			return
		}
		h.swap(index, child)
		index = child
	}
}
func (h *smtInfRationalDiffHeap) insert(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		return
	}
	h.positions[v] = len(h.variables)
	h.variables = append(h.variables, v)
	h.up(len(h.variables) - 1)
}
func (h *smtInfRationalDiffHeap) decreased(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		h.up(h.positions[v])
	}
}
func (h *smtInfRationalDiffHeap) eraseMin() int32 {
	result := h.variables[0]
	last := len(h.variables) - 1
	h.swap(0, last)
	h.variables = h.variables[:last]
	h.positions[result] = -1
	if len(h.variables) != 0 {
		h.down(0)
	}
	return result
}
func (h *smtInfRationalDiffHeap) reset() {
	for _, variable := range h.variables {
		h.positions[variable] = -1
	}
	h.variables = h.variables[:0]
}
