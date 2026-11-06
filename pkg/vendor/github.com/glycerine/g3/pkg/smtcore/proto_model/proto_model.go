package protomodel

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

type ProtoModel struct {
	self        *ProtoModel
	manager     *ast.ASTManager
	core        *model.Model
	evaluator   *model.ModelEvaluator
	rewriter    *rewriter.TheoryRewriter
	factories   map[ast.FamilyID]ValueFactory
	factoryList []ValueFactory
	user        *userSortFactory
	aux         map[ast.ASTHandle]struct{}
	partial     bool
	closePhase  uint8
	closed      bool
}

func NewProtoModel(m *ast.ASTManager, config model.ModelEvaluatorConfig) (p *ProtoModel, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	p = &ProtoModel{manager: m, factories: make(map[ast.FamilyID]ValueFactory), aux: make(map[ast.ASTHandle]struct{})}
	p.self = p
	p.core, err = model.NewModel(m)
	if err != nil {
		return nil, err
	}
	// Named returns may set p to nil before this deferred unwind runs.
	// Keep the actual allocated owner until its cleanup finishes.
	owner := p
	defer func() {
		if err != nil {
			err = errors.Join(err, owner.Close())
		}
	}()
	if err = p.core.Core().SetValueProvider(p); err != nil {
		return nil, err
	}
	p.evaluator, err = model.NewModelEvaluator(p.core.Core(), config)
	if err != nil {
		return nil, err
	}
	p.rewriter, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	if err != nil {
		return nil, err
	}
	params, err := model.NewModelParams(config.Context, config.Globals, config.Params)
	if err != nil {
		return nil, err
	}
	p.partial, err = params.Partial()
	err = errors.Join(err, params.Close())
	if err != nil {
		return nil, err
	}
	if err = p.RegisterFactory(newBasicFactory(m)); err != nil {
		return nil, err
	}
	p.user = newUserSortFactory(m)
	if err = p.RegisterFactory(p.user); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *ProtoModel) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed || p.closePhase != 0 {
		return arena.ErrClosed
	}
	return p.core.Core().Check()
}
func (p *ProtoModel) Core() *model.ModelCore { return p.core.Core() }
func (p *ProtoModel) GetSomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	return p.SomeValue(s)
}
func (p *ProtoModel) RegisterFactory(f ValueFactory) error {
	if err := p.open(); err != nil {
		return err
	}
	if f == nil {
		return ast.ErrASTHandle
	}
	if _, found := p.factories[f.FamilyID()]; found {
		return ast.ErrASTDomain
	}
	p.factories[f.FamilyID()] = f
	p.factoryList = append(p.factoryList, f)
	return nil
}
func (p *ProtoModel) Factory(fid ast.FamilyID) ValueFactory { return p.factories[fid] }
func (p *ProtoModel) isUserSort(s ast.ASTHandle) (bool, error) {
	fid, err := p.manager.GetFamilyID(s)
	return fid == 4, err
}
func (p *ProtoModel) SomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	user, err := p.isUserSort(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if user {
		return p.user.SomeValue(s)
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if f := p.factories[fid]; f != nil {
		return f.SomeValue(s)
	}
	return p.user.SomeValue(s)
}
func (p *ProtoModel) SomeValues(s ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	user, err := p.isUserSort(s)
	if err != nil {
		return false, err
	}
	if user {
		return p.user.SomeValues(s, first, second)
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	if f := p.factories[fid]; f != nil {
		return f.SomeValues(s, first, second)
	}
	return false, nil
}
func (p *ProtoModel) FreshValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	user, err := p.isUserSort(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if user {
		return p.user.FreshValue(s)
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if f := p.factories[fid]; f != nil {
		return f.FreshValue(s)
	}
	return p.user.FreshValue(s)
}
func (p *ProtoModel) RegisterValue(v ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	s, err := p.manager.GetSort(v)
	if err != nil {
		return err
	}
	user, err := p.isUserSort(s)
	if err != nil {
		return err
	}
	if user {
		return p.user.RegisterValue(v)
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil {
		return err
	}
	if f := p.factories[fid]; f != nil {
		return f.RegisterValue(v)
	}
	return nil
}
func (p *ProtoModel) FreezeUniverse(s ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	user, err := p.isUserSort(s)
	if err != nil {
		return err
	}
	if !user {
		return ast.ErrASTDomain
	}
	return p.user.Freeze(s)
}
func (p *ProtoModel) IsFinite(s ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	user, err := p.isUserSort(s)
	return user && p.user.IsFinite(s), err
}
func (p *ProtoModel) KnownUniverse(s ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return p.user.KnownUniverse(s), nil
}
func (p *ProtoModel) NumUninterpretedSorts() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return uint32(len(p.user.Sorts())), nil
}
func (p *ProtoModel) UninterpretedSort(i uint32) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := p.user.Sorts()
	if uint64(i) >= uint64(len(sorts)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return sorts[i], nil
}
func (p *ProtoModel) RegisterAuxFunction(d ast.ASTHandle, fi *model.FunctionInterpretation) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.core.Core().RegisterFunction(d, fi); err != nil {
		return err
	}
	p.aux[d] = struct{}{}
	return nil
}
func (p *ProtoModel) RegisterAuxDeclaration(d ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	p.aux[d] = struct{}{}
	return nil
}
func (p *ProtoModel) ReregisterFunction(d ast.ASTHandle, replacement *model.FunctionInterpretation, auxiliary ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	old, err := p.core.Core().FunctionInterpretation(d)
	if err != nil {
		return err
	}
	if old != nil && !auxiliary.IsNull() {
		copy, err := old.Copy()
		if err != nil {
			return err
		}
		if err = p.core.Core().RegisterFunction(auxiliary, copy); err != nil {
			_ = copy.Close()
			return err
		}
		p.aux[auxiliary] = struct{}{}
	}
	return p.core.Core().RegisterFunction(d, replacement)
}
func (p *ProtoModel) Eval(term ast.ASTHandle, out ast.ASTRef, completion bool) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if err := p.evaluator.SetModelCompletion(completion); err != nil {
		return false, err
	}
	if err := p.evaluator.SetExpandArrayEqualities(false); err != nil {
		return false, err
	}
	if err := p.evaluator.Evaluate(term, out); err != nil {
		return false, err
	}
	return true, nil
}
func (p *ProtoModel) Compress() error {
	if err := p.open(); err != nil {
		return err
	}
	for i := uint32(0); i < p.core.Core().NumFunctionDeclarations(); i++ {
		d, err := p.core.Core().Function(i)
		if err != nil {
			return err
		}
		fi, err := p.core.Core().FunctionInterpretation(d)
		if err != nil {
			return err
		}
		if err = fi.Compress(); err != nil {
			return err
		}
	}
	return nil
}
func (p *ProtoModel) CompletePartialFunction(d ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	fi, err := p.core.Core().FunctionInterpretation(d)
	if err != nil || fi == nil {
		return err
	}
	partial, err := fi.IsPartial()
	if err != nil || !partial {
		return err
	}
	v, err := fi.MaxOccurrenceResult()
	if err != nil {
		return err
	}
	if v.IsNull() {
		s, err := p.manager.GetRange(d)
		if err != nil {
			return err
		}
		v, err = p.SomeValue(s)
		if err != nil {
			return err
		}
	}
	return fi.SetElse(v)
}
func (p *ProtoModel) CompletePartialFunctions() error {
	if err := p.open(); err != nil || p.partial {
		return err
	}
	for i := uint32(0); i < p.core.Core().NumFunctionDeclarations(); i++ {
		d, err := p.core.Core().Function(i)
		if err != nil {
			return err
		}
		if err = p.CompletePartialFunction(d); err != nil {
			return err
		}
	}
	return nil
}
