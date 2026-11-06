package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h mk_var, add_edge,
// update_cells, get_antecedents, restore_cells and init_model.

// Dense closure stores a source edge explaining each improved distance.
// Zero is the self-edge sentinel; negative one means disconnected.
type smtDenseDiffCell struct {
	edge     int
	distance smtInfRational
}

type smtDenseDiffEdge struct {
	source, target int
	offset         smtInfRational
	literal        SMTLiteral
}

type smtDenseDiffTrail struct {
	source, target int
	old            smtDenseDiffCell
}

type smtDenseDiffScope struct{ variables, edges, trail int }

type smtDenseDiffMatrix struct {
	owner  *SMTDenseDiffTheory
	rows   [][]smtDenseDiffCell
	edges  []smtDenseDiffEdge
	trail  []smtDenseDiffTrail
	scopes []smtDenseDiffScope
}

func newSMTDenseDiffMatrix() *smtDenseDiffMatrix {
	return &smtDenseDiffMatrix{edges: []smtDenseDiffEdge{{source: -1, target: -1, literal: SMTNullLiteral}}}
}

func (g *smtDenseDiffMatrix) addVariable() int {
	v := len(g.rows)
	for i := range g.rows {
		g.rows[i] = append(g.rows[i], smtDenseDiffCell{edge: -1})
	}
	row := make([]smtDenseDiffCell, v+1)
	for i := range row {
		row[i].edge = -1
	}
	row[v].edge = 0
	g.rows = append(g.rows, row)
	return v
}

func (g *smtDenseDiffMatrix) valid(v int) bool {
	return g != nil && v >= 0 && v < len(g.rows)
}

func (g *smtDenseDiffMatrix) distance(source, target int) (smtInfRational, bool) {
	var value smtInfRational
	if !g.valid(source) || !g.valid(target) || g.rows[source][target].edge < 0 {
		return value, false
	}
	value.Set(&g.rows[source][target].distance)
	return value, true
}

func (g *smtDenseDiffMatrix) antecedents(source, target int) ([]SMTLiteral, error) {
	if !g.valid(source) || !g.valid(target) || g.rows[source][target].edge < 0 {
		return nil, errSMTDiffGraphDomain
	}
	type pair struct{ source, target int }
	var todo []pair
	if source != target {
		todo = append(todo, pair{source, target})
	}
	var result []SMTLiteral
	for len(todo) > 0 {
		i := len(todo) - 1
		p := todo[i]
		todo = todo[:i]
		id := g.rows[p.source][p.target].edge
		if id <= 0 || id >= len(g.edges) {
			return nil, errSMTDiffGraphDomain
		}
		e := &g.edges[id]
		if e.literal != SMTNullLiteral {
			result = append(result, e.literal)
		}
		if p.source != e.source {
			todo = append(todo, pair{p.source, e.source})
		}
		if e.target != p.target {
			todo = append(todo, pair{e.target, p.target})
		}
	}
	return result, nil
}

// A rejected edge reports the source's reverse-path antecedents followed by
// the new literal, without changing the matrix or edge list.
func (g *smtDenseDiffMatrix) addEdge(source, target int, offset *smtInfRational, literal SMTLiteral) (bool, []SMTLiteral, error) {
	if !g.valid(source) || !g.valid(target) || offset == nil {
		return false, nil, errSMTDiffGraphDomain
	}
	inverse := &g.rows[target][source]
	var negative smtInfRational
	negative.Neg(&inverse.distance)
	if inverse.edge >= 0 && negative.Cmp(offset) > 0 {
		antecedents, err := g.antecedents(target, source)
		if err != nil {
			return false, nil, err
		}
		if literal != SMTNullLiteral {
			antecedents = append(antecedents, literal)
		}
		return false, antecedents, nil
	}
	cell := &g.rows[source][target]
	if cell.edge < 0 || offset.Cmp(&cell.distance) < 0 {
		e := smtDenseDiffEdge{source: source, target: target, literal: literal}
		e.offset.Set(offset)
		g.edges = append(g.edges, e)
		if err := g.updateCells(); err != nil {
			return false, nil, err
		}
	}
	return true, nil, nil
}

func (g *smtDenseDiffMatrix) updateCells() error {
	id := len(g.edges) - 1
	e := &g.edges[id]
	s, t := e.source, e.target
	type forward struct {
		target   int
		distance smtInfRational
	}
	var targets []forward
	for x := range g.rows {
		c := &g.rows[t][x]
		if c.edge >= 0 && x != s {
			var value smtInfRational
			value.Add(&e.offset, &c.distance)
			old := &g.rows[s][x]
			if old.edge < 0 || value.Cmp(&old.distance) < 0 {
				targets = append(targets, forward{x, value})
			}
		}
	}
	for y := range g.rows {
		if y == t {
			continue
		}
		c := &g.rows[y][s]
		if c.edge < 0 {
			continue
		}
		for i := range targets {
			x := targets[i].target
			if x == y {
				continue
			}
			var value smtInfRational
			value.Add(&c.distance, &targets[i].distance)
			cell := &g.rows[y][x]
			if cell.edge < 0 || value.Cmp(&cell.distance) < 0 {
				old := smtDenseDiffCell{edge: cell.edge}
				old.distance.Set(&cell.distance)
				g.trail = append(g.trail, smtDenseDiffTrail{y, x, old})
				cell.edge = id
				cell.distance.Set(&value)
				if g.owner != nil {
					if err := g.owner.propagateCell(y, x); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (g *smtDenseDiffMatrix) push() {
	g.scopes = append(g.scopes, smtDenseDiffScope{len(g.rows), len(g.edges), len(g.trail)})
}

func (g *smtDenseDiffMatrix) pop(count uint32) error {
	if g == nil || uint64(count) > uint64(len(g.scopes)) {
		return errSMTDiffGraphDomain
	}
	if count == 0 {
		return nil
	}
	level := len(g.scopes) - int(count)
	scope := g.scopes[level]
	for i := len(g.trail) - 1; i >= scope.trail; i-- {
		t := &g.trail[i]
		cell := &g.rows[t.source][t.target]
		cell.edge = t.old.edge
		cell.distance.Set(&t.old.distance)
	}
	clear(g.trail[scope.trail:])
	g.trail = g.trail[:scope.trail]
	clear(g.edges[scope.edges:])
	g.edges = g.edges[:scope.edges]
	for i := range g.rows {
		clear(g.rows[i][scope.variables:])
		g.rows[i] = g.rows[i][:scope.variables]
	}
	clear(g.rows[scope.variables:])
	g.rows = g.rows[:scope.variables]
	g.scopes = g.scopes[:level]
	return nil
}

func (g *smtDenseDiffMatrix) assignment() []smtInfRational {
	values := make([]smtInfRational, len(g.rows))
	for i := range g.rows {
		for j := range g.rows[i] {
			c := &g.rows[i][j]
			if i != j && c.edge >= 0 && c.distance.Cmp(&values[i]) < 0 {
				values[i].Set(&c.distance)
			}
		}
		values[i].Neg(&values[i])
	}
	return values
}
