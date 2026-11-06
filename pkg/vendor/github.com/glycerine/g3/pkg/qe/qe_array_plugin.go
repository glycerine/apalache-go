package qe

// Source: src/qe/qe_array_plugin.cpp, array-plugin factory and chain recognition.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// ArrayPlugin borrows its context and recognizes source store/select chains.
type ArrayPlugin struct {
	self    *ArrayPlugin
	context *SolverContext
	manager *ast.ASTManager
	family  ast.FamilyID
	closed  bool
}

func NewArrayPlugin(context *SolverContext) (*ArrayPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, e := context.Manager()
	if e != nil {
		return nil, e
	}
	family, e := m.RegisterArrayPlugin()
	if e != nil {
		return nil, e
	}
	p := &ArrayPlugin{context: context, manager: m, family: family}
	p.self = p
	return p, nil
}
func (p *ArrayPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.context.open()
}
func (p *ArrayPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}
func (p *ArrayPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if e := p.open(); e != nil {
		return ast.ASTHandle{}, false, e
	}
	return ast.ASTHandle{}, false, nil
}
func (p *ArrayPlugin) IsUninterpreted(h ast.ASTHandle) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	k, e := p.manager.GetKind(h)
	if e != nil {
		return false, e
	}
	if k != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return true, nil
}
func (p *ArrayPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}

// arrayChain implements the recursive is_array_app_of overload used by
// is_select and is_store_update. Arguments are borrowed in outermost-first
// order. Every index/value and rhs must be free of the selected variable.
func (p *ArrayPlugin) arrayChain(term, rhs ast.ASTHandle, op int32) (uint32, [][]ast.ASTHandle, bool, error) {
	if e := p.open(); e != nil {
		return 0, nil, false, e
	}
	if op != ast.ASTArrayOpSelect && op != ast.ASTArrayOpStore {
		return 0, nil, false, ast.ErrASTDomain
	}
	var args [][]ast.ASTHandle
	for {
		kind, e := p.manager.GetKind(term)
		if e != nil {
			return 0, nil, false, e
		}
		if kind != ast.ASTApp {
			return 0, nil, false, nil
		}
		idx, found, e := p.context.IsVariable(term)
		if e != nil {
			return 0, nil, false, e
		}
		if found {
			if len(args) == 0 {
				return 0, nil, false, nil
			}
			contains, e := p.context.Contains(idx)
			if e != nil {
				return 0, nil, false, e
			}
			occurs, e := contains.Check(rhs)
			if e != nil {
				return 0, nil, false, e
			}
			if occurs {
				return 0, nil, false, nil
			}
			for _, level := range args {
				for _, arg := range level {
					occurs, e = contains.Check(arg)
					if e != nil {
						return 0, nil, false, e
					}
					if occurs {
						return 0, nil, false, nil
					}
				}
			}
			return idx, args, true, nil
		}
		family, e := p.manager.GetFamilyID(term)
		if e != nil {
			return 0, nil, false, e
		}
		if family != p.family {
			return 0, nil, false, nil
		}
		kindOp, e := p.manager.GetDeclKind(term)
		if e != nil {
			return 0, nil, false, e
		}
		if kindOp != op {
			return 0, nil, false, nil
		}
		n, e := p.manager.GetNumArgs(term)
		if e != nil {
			return 0, nil, false, e
		}
		if n == 0 {
			return 0, nil, false, nil
		}
		level := make([]ast.ASTHandle, 0, n-1)
		for i := uint32(1); i < n; i++ {
			arg, e := p.manager.GetArg(term, i)
			if e != nil {
				return 0, nil, false, e
			}
			level = append(level, arg)
		}
		args = append(args, level)
		term, e = p.manager.GetArg(term, 0)
		if e != nil {
			return 0, nil, false, e
		}
	}
}
