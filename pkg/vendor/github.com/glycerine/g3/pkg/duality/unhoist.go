package duality

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// Unhoist undoes initialization-expression hoisting for source simple loops:
// exactly two defining clauses, one a single-child self-loop.
// Source: duality_rpfp.cpp, UnhoistLoop/Unhoist.
func (g *RPFP) Unhoist() error {
	if err := g.open(); err != nil {
		return err
	}
	outgoing := make(map[*Node][]*Edge)
	for _, e := range g.edges {
		outgoing[e.Parent] = append(outgoing[e.Parent], e)
	}
	for _, node := range g.nodes {
		edges := outgoing[node]
		if len(edges) != 2 {
			continue
		}
		for i, e := range edges {
			if len(e.Children) == 1 && e.Children[0] == node {
				if err := g.unhoistLoop(e, edges[1-i]); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func (g *RPFP) unhoistLoop(loop, init *Edge) (result error) {
	m := g.context.manager
	var expressions []*Expr
	hold := func(e *Expr, err error) (*Expr, error) {
		if e != nil {
			expressions = append(expressions, e)
		}
		return e, err
	}
	defer func() {
		for _, e := range expressions {
			result = errors.Join(result, e.Close())
		}
	}()
	for _, e := range []*Edge{loop, init} {
		formula, err := hold(e.F.Formula())
		if err != nil {
			return err
		}
		normalized, err := hold(IneqToEq(formula))
		if err != nil {
			return err
		}
		h, err := normalized.Raw()
		if err != nil {
			return err
		}
		if err := m.SetRef(e.F.formula, h); err != nil {
			return err
		}
	}
	pre, err := hold(g.GetRel(loop, 0))
	if err != nil {
		return err
	}
	if pre == nil {
		return nil
	}
	preRaw, err := pre.Raw()
	if err != nil {
		return err
	}
	pull := &unhoistPull{context: g.context, constantParams: make(map[ast.ASTHandle]ast.ASTHandle), inverse: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() { result = errors.Join(result, pull.close()) }()
	var work []ast.ASTHandle
	for i, ref := range loop.F.individualParams {
		parameter, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		arg, err := m.GetArg(preRaw, uint32(i))
		if err != nil {
			return err
		}
		if arg == parameter {
			initial, err := m.RefGet(init.F.individualParams[i])
			if err != nil {
				return err
			}
			pull.constantParams[arg] = initial
			pull.inverse[initial] = arg
			work = append(work, arg)
		}
	}
	formula, err := hold(init.F.Formula())
	if err != nil {
		return err
	}
	defs, err := GetDefs(formula)
	if err != nil {
		return err
	}
	defer func() {
		for _, d := range defs {
			result = errors.Join(result, d.Close())
		}
	}()
	pull.definitions = defs
	substitutions := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, v := range work {
		def, err := pull.pull(pull.constantParams[v])
		if err != nil {
			return err
		}
		if def != v {
			substitutions[v] = def
		}
	}
	if len(substitutions) == 0 {
		return nil
	}
	substitutions[preRaw] = preRaw
	original, err := m.RefGet(loop.F.formula)
	if err != nil {
		return err
	}
	rewritten, err := substituteExpression(g.context, original, substitutions)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&rewritten)) }()
	h, err := m.RefGet(rewritten)
	if err != nil {
		return err
	}
	if err := m.SetRef(loop.F.formula, h); err != nil {
		return err
	}
	for _, e := range []*Edge{loop, init} {
		formula, err := hold(e.F.Formula())
		if err != nil {
			return err
		}
		expanded, err := hold(ElimIte(formula))
		if err != nil {
			return err
		}
		h, err := expanded.Raw()
		if err != nil {
			return err
		}
		if err := m.SetRef(e.F.formula, h); err != nil {
			return err
		}
	}
	if len(pull.newParams) == 0 {
		return nil
	}
	params := make([]*Expr, 0, len(pull.newParams))
	for _, h := range pull.newParams {
		e, err := hold(NewExpr(g.context, h))
		if err != nil {
			return err
		}
		params = append(params, e)
	}
	parent := loop.Parent
	if err := g.AddParamsToNode(parent, params); err != nil {
		return err
	}
	if err := loop.F.AddParams(params); err != nil {
		return err
	}
	if err := init.F.AddParams(params); err != nil {
		return err
	}
	newDecl, err := m.RefGet(parent.Name)
	if err != nil {
		return err
	}
	decl, err := NewFuncDecl(g.context, newDecl)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, decl.Close()) }()
	// Incoming contains one entry per child position. Rewrite each edge once;
	// its inner loop already handles every position, including repeated children.
	processed := make(map[*Edge]bool)
	for _, incoming := range parent.Incoming {
		if processed[incoming] {
			continue
		}
		processed[incoming] = true
		for j, child := range incoming.Children {
			if child != parent {
				continue
			}
			lit, err := hold(g.GetRel(incoming, j))
			if err != nil {
				return err
			}
			if lit == nil {
				return ErrWrapperOwner
			}
			app, err := hold(AddParamsToApp(lit, decl, params))
			if err != nil {
				return err
			}
			appRaw, err := app.Raw()
			if err != nil {
				return err
			}
			name, err := m.GetASTName(newDecl)
			if err != nil {
				return err
			}
			symbol, err := newSymbol(g.context, name)
			if err != nil {
				return err
			}
			text, err := symbol.String()
			if err != nil {
				return err
			}
			suffix, err := m.Symbols.Intern(text + "_" + strconv.Itoa(j))
			if err != nil {
				return err
			}
			n, err := m.GetNumArgs(appRaw)
			if err != nil {
				return err
			}
			domain := make([]ast.ASTHandle, n)
			args := make([]ast.ASTHandle, n)
			for i := range args {
				args[i], err = m.GetArg(appRaw, uint32(i))
				if err != nil {
					return err
				}
				domain[i], err = m.GetSort(args[i])
				if err != nil {
					return err
				}
			}
			sort, err := m.GetSort(appRaw)
			if err != nil {
				return err
			}
			fd, err := m.MkFuncDecl(suffix, domain, sort, ast.ASTFuncDeclInfo{})
			if err != nil {
				return err
			}
			if err := m.SetRef(incoming.F.relationParams[j], fd); err != nil {
				return err
			}
			replacement, err := m.MkApp(fd, args)
			if err != nil {
				return err
			}
			_, err = hold(NewExpr(g.context, replacement))
			if err != nil {
				return err
			}
			old, err := lit.Raw()
			if err != nil {
				return err
			}
			formula, err := m.RefGet(incoming.F.formula)
			if err != nil {
				return err
			}
			ref, err := substituteExpression(g.context, formula, map[ast.ASTHandle]ast.ASTHandle{old: replacement})
			if err != nil {
				return err
			}
			updated, readErr := m.RefGet(ref)
			if readErr == nil {
				readErr = m.SetRef(incoming.F.formula, updated)
			}
			if err := errors.Join(readErr, m.DelRef(&ref)); err != nil {
				return err
			}
		}
	}
	return nil
}
