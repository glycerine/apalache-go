package protomodel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ProtoModel) uninterpretedConstant(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if kind, err := p.manager.GetKind(h); err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	if n, err := p.manager.GetNumArgs(h); err != nil || n != 0 {
		return ast.ASTHandle{}, false, err
	}
	d, err := p.manager.GetDecl(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	fid, err := p.manager.GetFamilyID(d)
	return d, fid == ast.NullFamilyID, err
}

func (p *ProtoModel) cleanFunctionElse(fi *model.FunctionInterpretation, found map[ast.ASTHandle]struct{}) (err error) {
	root, err := fi.Else()
	if err != nil {
		return err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var owned []ast.ASTRef
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			err = errors.Join(err, p.manager.DelRef(&owned[i]))
		}
	}()
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		d, constant, e := p.uninterpretedConstant(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if constant {
			v, e := p.core.Core().ConstantInterpretation(d)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if v.IsNull() {
				s, e := p.manager.GetRange(d)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				v, e = p.SomeValue(s)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if e = p.core.Core().RegisterConstant(d, v); e != nil {
					return ast.ASTHandle{}, e
				}
			}
			memo[h] = v
			return v, nil
		}
		kind, e := p.manager.GetKind(h)
		if e != nil || kind != ast.ASTApp {
			memo[h] = h
			return h, e
		}
		d, e = p.manager.GetDecl(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if _, auxiliary := p.aux[d]; auxiliary {
			found[d] = struct{}{}
		}
		n, e := p.manager.GetNumArgs(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args := make([]ast.ASTHandle, n)
		for i := range args {
			a, e := p.manager.GetArg(h, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[i], e = visit(a)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		out, e := p.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		owned = append(owned, out)
		if e = p.rewriter.MakeApp(d, ast.SliceArguments(args), out); e != nil {
			return ast.ASTHandle{}, e
		}
		v, e := p.manager.RefGet(out)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		memo[h] = v
		return v, nil
	}
	cleaned, err := visit(root)
	if err != nil {
		return err
	}
	// Like the source trail, owned must keep every rewritten expression alive
	// until the function interpretation has acquired its fallback reference.
	return fi.SetElse(cleaned)
}

func (p *ProtoModel) Cleanup() error {
	if err := p.open(); err != nil {
		return err
	}
	found := make(map[ast.ASTHandle]struct{})
	count := p.core.Core().NumFunctionDeclarations()
	for i := uint32(0); i < count; i++ {
		d, err := p.core.Core().Function(i)
		if err != nil {
			return err
		}
		fi, err := p.core.Core().FunctionInterpretation(d)
		if err != nil {
			return err
		}
		partial, err := fi.IsPartial()
		if err != nil || partial {
			if err != nil {
				return err
			}
			continue
		}
		if err = p.cleanFunctionElse(fi, found); err != nil {
			return err
		}
	}
	for d := range p.aux {
		if _, used := found[d]; used {
			continue
		}
		if err := p.core.Core().UnregisterDeclaration(d); err != nil {
			return err
		}
		delete(p.aux, d)
	}
	return nil
}

func (p *ProtoModel) FinalModel() (*model.Model, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result, err := model.NewModel(p.manager)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*model.Model, error) { return nil, errors.Join(e, result.Close()) }
	if err = result.Core().CopyConstantInterpretations(p.core.Core()); err != nil {
		return fail(err)
	}
	if err = result.Core().CopyFunctionInterpretations(p.core.Core()); err != nil {
		return fail(err)
	}
	for _, s := range p.user.Sorts() {
		if err = result.RegisterUniverse(s, p.user.KnownUniverse(s)); err != nil {
			return fail(err)
		}
	}
	if err = p.Close(); err != nil {
		return fail(err)
	}
	return result, nil
}

func (p *ProtoModel) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.closePhase == 0 {
		p.closePhase = 1
	}
	if p.closePhase == 1 {
		if p.evaluator != nil {
			if err := p.evaluator.Close(); err != nil {
				return err
			}
			p.evaluator = nil
		}
		p.closePhase = 2
	}
	if p.closePhase == 2 {
		if p.rewriter != nil {
			if err := p.rewriter.Close(); err != nil {
				return err
			}
			p.rewriter = nil
		}
		p.closePhase = 3
	}
	if p.closePhase == 3 {
		for len(p.factoryList) != 0 {
			i := len(p.factoryList) - 1
			if err := p.factoryList[i].Close(); err != nil {
				return err
			}
			p.factoryList[i] = nil
			p.factoryList = p.factoryList[:i]
		}
		p.factories = nil
		p.user = nil
		p.closePhase = 4
	}
	if p.closePhase == 4 {
		if p.core != nil {
			if err := p.core.Close(); err != nil {
				return err
			}
			p.core = nil
		}
		p.closePhase = 5
	}
	p.aux = nil
	p.closed = true
	return nil
}
