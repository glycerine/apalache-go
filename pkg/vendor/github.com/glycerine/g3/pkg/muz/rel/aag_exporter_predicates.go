package rel

import "github.com/glycerine/g3/pkg/ast"

func (x *AAGExporter) initializePredicates() error {
	seen := map[ast.ASTHandle]bool{}
	order := make([]ast.ASTHandle, 0)
	n, e := x.rules.Len()
	if e != nil {
		return e
	}
	for i := 0; i < n; i++ {
		r, e := x.rules.Rule(i)
		if e != nil {
			return e
		}
		d, e := r.HeadDeclaration()
		r.Close()
		if e != nil {
			return e
		}
		if !seen[d] {
			seen[d] = true
			order = append(order, d)
		}
	}
	for _, f := range x.facts {
		if !seen[f.Declaration] {
			seen[f.Declaration] = true
			order = append(order, f.Declaration)
		}
	}
	id := uint32(1)
	for _, d := range order {
		x.predicateIDs[d] = id
		id++
	}
	bits := uint32(0)
	for capacity := uint32(1); capacity < id; capacity <<= 1 {
		bits++
	}
	for i := uint32(0); i < bits; i++ {
		a, e := x.fresh("rule_id")
		if e != nil {
			return e
		}
		b, e := x.fresh("rule_id_p")
		if e != nil {
			return e
		}
		x.current = append(x.current, a)
		x.next = append(x.next, b)
	}
	return nil
}
func (x *AAGExporter) predicateTerms(decl ast.ASTHandle, vars []ast.ASTHandle) ([]ast.ASTHandle, error) {
	id := uint32(0)
	if !decl.IsNull() {
		id = x.predicateIDs[decl]
	}
	out := make([]ast.ASTHandle, len(vars))
	for i, v := range vars {
		if id&(1<<uint(i)) != 0 {
			out[i] = v
		} else {
			n, e := x.manager.MkNot(v)
			if e != nil {
				return nil, e
			}
			out[i] = n
		}
	}
	return out, nil
}
