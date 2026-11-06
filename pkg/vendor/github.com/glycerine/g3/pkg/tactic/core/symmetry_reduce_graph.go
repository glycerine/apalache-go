package core

// Source: src/tactic/core/symmetry_reduce_tactic.cpp live coloring, occurrence,
// range-restriction and term-selection specializations of for_each_expr.

import "github.com/glycerine/g3/pkg/ast"

type symmetryNode struct {
	kind ast.ASTKind
	args []ast.ASTHandle
	// For quantifiers args starts with the body, then patterns/no-patterns.
	sort   ast.ASTHandle
	family ast.FamilyID
}
type symmetryGraph struct {
	nodes       map[ast.ASTHandle]symmetryNode
	apps        []ast.ASTHandle
	occurrences map[ast.ASTHandle]uint32
}

func symmetryContains(values []ast.ASTHandle, value ast.ASTHandle) bool {
	for _, h := range values {
		if h == value {
			return true
		}
	}
	return false
}
func (t *SymmetryReduceTactic) symmetryNode(term ast.ASTHandle) (n symmetryNode, err error) {
	m := t.manager
	n.kind, err = m.GetKind(term)
	if err != nil {
		return n, err
	}
	if n.kind == ast.ASTApp {
		count, e := m.GetNumArgs(term)
		if e != nil {
			return n, e
		}
		n.args = make([]ast.ASTHandle, count)
		for i := range n.args {
			n.args[i], err = m.GetArg(term, uint32(i))
			if err != nil {
				return n, err
			}
		}
		n.sort, err = m.GetSort(term)
		if err != nil {
			return n, err
		}
		n.family, err = m.GetFamilyID(term)
	} else if n.kind == ast.ASTQuantifier {
		body, e := m.GetQuantifierBody(term)
		if e != nil {
			return n, e
		}
		n.args = append(n.args, body)
		count, e := m.GetNumPatterns(term)
		if e != nil {
			return n, e
		}
		for i := uint32(0); i < count; i++ {
			h, e := m.GetPattern(term, i)
			if e != nil {
				return n, e
			}
			n.args = append(n.args, h)
		}
		count, e = m.GetNumNoPatterns(term)
		if e != nil {
			return n, e
		}
		for i := uint32(0); i < count; i++ {
			h, e := m.GetNoPattern(term, i)
			if e != nil {
				return n, e
			}
			n.args = append(n.args, h)
		}
	} else if n.kind != ast.ASTVar {
		err = ast.ErrASTDomain
	}
	return n, err
}

func (t *SymmetryReduceTactic) symmetryGraph(root ast.ASTHandle) (*symmetryGraph, error) {
	g := &symmetryGraph{nodes: make(map[ast.ASTHandle]symmetryNode), occurrences: make(map[ast.ASTHandle]uint32)}
	type visit struct {
		term   ast.ASTHandle
		finish bool
	}
	todo := []visit{{term: root}}
	for len(todo) > 0 {
		top := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if top.finish {
			n := g.nodes[top.term]
			if n.kind == ast.ASTApp {
				g.apps = append(g.apps, top.term)
				if _, found := g.occurrences[top.term]; !found {
					g.occurrences[top.term] = 0
				}
				for _, child := range n.args {
					if g.nodes[child].kind == ast.ASTApp {
						g.occurrences[child]++
					}
				}
			}
			continue
		}
		if _, found := g.nodes[top.term]; found {
			continue
		}
		n, err := t.symmetryNode(top.term)
		if err != nil {
			return nil, err
		}
		g.nodes[top.term] = n
		todo = append(todo, visit{term: top.term, finish: true})
		for i := len(n.args) - 1; i >= 0; i-- {
			todo = append(todo, visit{term: n.args[i]})
		}
	}
	return g, nil
}

func (t *SymmetryReduceTactic) candidatePermutations(root ast.ASTHandle, g *symmetryGraph) ([][]ast.ASTHandle, error) {
	depths := make(map[ast.ASTHandle]uint32)
	type visit struct {
		term  ast.ASTHandle
		depth uint32
	}
	todo := []visit{{term: root}}
	for len(todo) > 0 {
		v := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		n := g.nodes[v.term]
		if n.kind == ast.ASTQuantifier {
			todo = append(todo, visit{n.args[0], v.depth + 1})
			continue
		}
		if n.kind != ast.ASTApp {
			continue
		}
		if old, found := depths[v.term]; found && v.depth <= old {
			continue
		}
		depths[v.term] = v.depth
		for _, child := range n.args {
			todo = append(todo, visit{child, v.depth + 1})
		}
	}
	sortColors := make(map[ast.ASTHandle]uint32)
	colors := make(map[ast.ASTHandle]uint32)
	for _, app := range g.apps {
		sort := g.nodes[app].sort
		color, found := sortColors[sort]
		if !found {
			color = uint32(len(sortColors))
			sortColors[sort] = color
		}
		colors[app] = color
	}
	// Dense IDs preserve partition equivalence; map iteration order is never
	// used as a heuristic. The source obj_map hash order is platform-specific.
	for _, refinement := range []map[ast.ASTHandle]uint32{g.occurrences, depths} {
		pairs := make(map[[2]uint32]uint32)
		for _, app := range g.apps {
			value, found := refinement[app]
			if !found {
				return nil, ast.ErrASTDomain
			} // source VERIFY(colors1.find)
			pair := [2]uint32{colors[app], value}
			color, found := pairs[pair]
			if !found {
				color = uint32(len(pairs))
				pairs[pair] = color
			}
			colors[app] = color
		}
	}
	groupIDs := make(map[uint32]int)
	var groups [][]ast.ASTHandle
	for _, app := range g.apps {
		if g.nodes[app].family != ast.NullFamilyID {
			continue
		}
		color := colors[app]
		i, found := groupIDs[color]
		if !found {
			i = len(groups)
			groupIDs[color] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], app)
	}
	var result [][]ast.ASTHandle
	for _, group := range groups {
		if len(group) < 2 || g.occurrences[group[0]] < 2 {
			continue
		}
		constants := true
		for _, app := range group {
			if len(g.nodes[app].args) != 0 {
				constants = false
				break
			}
		}
		if constants {
			result = append(result, group)
			if err := t.traceCandidates(group); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (t *SymmetryReduceTactic) symmetryOp(term ast.ASTHandle, op int32) (bool, error) {
	m := t.manager
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(term)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	k, err := m.GetDeclKind(term)
	return k == op, err
}
func (t *SymmetryReduceTactic) rangeRestriction(form ast.ASTHandle, constants []ast.ASTHandle) (ast.ASTHandle, error) {
	var term ast.ASTHandle
	ok, err := t.symmetryOp(form, ast.ASTOpOr)
	if err != nil || !ok {
		return term, err
	}
	n, err := t.symmetryNode(form)
	if err != nil {
		return term, err
	}
	for _, eq := range n.args {
		ok, err := t.symmetryOp(eq, ast.ASTOpEq)
		if err != nil || !ok {
			return ast.ASTHandle{}, err
		}
		e, err := t.symmetryNode(eq)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if len(e.args) != 2 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		a, b := e.args[0], e.args[1]
		ak, err := t.manager.GetKind(a)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		bk, err := t.manager.GetKind(b)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if ak != ast.ASTApp || bk != ast.ASTApp {
			return ast.ASTHandle{}, nil
		}
		if symmetryContains(constants, a) && (term.IsNull() || term == b) {
			term = b
		} else if symmetryContains(constants, b) && (term.IsNull() || term == a) {
			term = a
		} else {
			return ast.ASTHandle{}, nil
		}
	}
	return term, nil
}
func (t *SymmetryReduceTactic) selectSymmetryTerms(root ast.ASTHandle, constants []ast.ASTHandle) ([]ast.ASTHandle, error) {
	todo := []ast.ASTHandle{root}
	var terms []ast.ASTHandle
	for len(todo) > 0 {
		form := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		ok, err := t.symmetryOp(form, ast.ASTOpAnd)
		if err != nil {
			return nil, err
		}
		if ok {
			n, err := t.symmetryNode(form)
			if err != nil {
				return nil, err
			}
			todo = append(todo, n.args...)
			continue
		}
		term, err := t.rangeRestriction(form, constants)
		if err != nil {
			return nil, err
		}
		if !term.IsNull() {
			terms = append(terms, term)
		}
	}
	return terms, nil
}
func (t *SymmetryReduceTactic) symmetryUsed(term ast.ASTHandle, used, constants []ast.ASTHandle) ([]ast.ASTHandle, error) {
	g, err := t.symmetryGraph(term)
	if err != nil {
		return nil, err
	}
	for _, app := range g.apps {
		if symmetryContains(constants, app) && !symmetryContains(used, app) {
			used = append(used, app)
		}
	}
	return used, t.traceSupport(term, constants, used)
}
func (t *SymmetryReduceTactic) symmetryDelta(term ast.ASTHandle, used, constants []ast.ASTHandle) (int, error) {
	if len(used) == len(constants) {
		return 0, nil
	}
	result, err := t.symmetryUsed(term, used, constants)
	return len(result) - len(used), err
}
func (t *SymmetryReduceTactic) promisingSymmetryTerm(terms, used, constants []ast.ASTHandle, occurrences map[ast.ASTHandle]uint32) (int, error) {
	best := 0
	weight, found := occurrences[terms[0]]
	if !found {
		return 0, ast.ErrASTDomain
	}
	delta, err := t.symmetryDelta(terms[0], used, constants)
	if err != nil {
		return 0, err
	}
	if err = t.traceWeight(terms[0], weight, delta, 551); err != nil {
		return 0, err
	}
	n, err := t.manager.GetNumArgs(terms[0])
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(terms); i++ {
		weight1, found := occurrences[terms[i]]
		if !found {
			return 0, ast.ErrASTDomain
		}
		n1, err := t.manager.GetNumArgs(terms[i])
		if err != nil {
			return 0, err
		}
		if weight1 < weight && n <= n1 {
			continue
		}
		delta1, err := t.symmetryDelta(terms[i], used, constants)
		if err != nil {
			return 0, err
		}
		if err = t.traceWeight(terms[i], weight1, delta1, 559); err != nil {
			return 0, err
		}
		if n == n1 && (weight1 > weight || delta1 < delta) || n > n1 {
			best = i
			weight = weight1
			delta = delta1
			n = n1
		}
	}
	return best, nil
}
