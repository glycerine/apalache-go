package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrCyclicInitialization = errors.New("duality: cyclic initialization definitions")

// unhoistPull is the concrete per-loop UnhoistPullRec state. Definitions and
// existing parameter maps are borrowed for its lifetime. Memo values own every
// rebuilt expression and fresh parameter until the loop rewrite has finished.
// Source: duality_rpfp.cpp, UnhoistPullRec.
type unhoistPull struct {
	context                 *Context
	definitions             map[ast.ASTHandle]*InitializationDefinition
	constantParams, inverse map[ast.ASTHandle]ast.ASTHandle
	memo                    map[ast.ASTHandle]ast.ASTRef
	visiting                map[ast.ASTHandle]bool
	newParams               []ast.ASTHandle
}

func (p *unhoistPull) close() error {
	var result error
	for h, ref := range p.memo {
		err := p.context.manager.DelRef(&ref)
		result = errors.Join(result, err)
		if ref == (ast.ASTRef{}) {
			delete(p.memo, h)
		} else {
			p.memo[h] = ref
		}
	}
	return result
}

func (p *unhoistPull) pull(w ast.ASTHandle) (out ast.ASTHandle, result error) {
	m := p.context.manager
	if ref, ok := p.memo[w]; ok {
		return m.RefGet(ref)
	}
	if p.visiting[w] {
		return ast.ASTHandle{}, ErrCyclicInitialization
	}
	if p.memo == nil {
		p.memo = make(map[ast.ASTHandle]ast.ASTRef)
	}
	if p.visiting == nil {
		p.visiting = make(map[ast.ASTHandle]bool)
	}
	p.visiting[w] = true
	defer delete(p.visiting, w)
	if def := p.definitions[w]; def != nil {
		vars, err := GetVars(def.Value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		defer func() {
			for _, v := range vars {
				result = errors.Join(result, v.Close())
			}
		}()
		replacements := make(map[ast.ASTHandle]ast.ASTHandle, len(vars))
		for _, v := range vars {
			h, err := v.Raw()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			value, err := p.pull(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			replacements[h] = value
		}
		h, err := def.Value.Raw()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := substituteExpression(p.context, h, replacements)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.memo[w] = ref
		return m.RefGet(ref)
	}
	if existing, ok := p.inverse[w]; ok {
		ref, err := m.NewRef(existing)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.memo[w] = ref
		return existing, nil
	}
	decl, err := m.GetDecl(w)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := newSymbol(p.context, name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := symbol.String()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := m.GetSort(w)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, err := m.MkFreshConst(text, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := m.NewRef(fresh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.memo[w] = ref
	if p.constantParams == nil {
		p.constantParams = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	if p.inverse == nil {
		p.inverse = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	p.constantParams[fresh] = w
	p.inverse[w] = fresh
	p.newParams = append(p.newParams, fresh)
	return fresh, nil
}
