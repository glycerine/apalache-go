package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp lemma.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const InfinityLevel = ^uint32(0)

type Lemma struct {
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	body     ast.ASTRef
	cube     []ast.ASTRef
	bindings []ast.ASTRef
	level    uint32
	closed   bool
}

func NewLemma(owner *Manager, formula ast.ASTHandle, level uint32) (_ *Lemma, resultErr error) {
	if owner == nil || owner.open() != nil || owner.contexts[0] == nil {
		return nil, ErrManagerOwner
	}
	config := owner.contexts[0].config
	body, err := normalizeFormula(owner.manager, config.Params, config.Globals, formula, true, false)
	if err != nil {
		return nil, err
	}
	return &Lemma{manager: owner.manager, context: config.Params, globals: config.Globals, body: body, level: level}, nil
}

func NewLemmaFromProofObligation(obligation *ProofObligation, cube []ast.ASTHandle, level uint32) (_ *Lemma, resultErr error) {
	if obligation == nil || obligation.valid() != nil {
		return nil, ErrManagerOwner
	}
	p := obligation.transformer
	manager := p.manager
	config := p.owner.contexts[0].config
	if cube == nil {
		post, err := obligation.Post()
		if err != nil {
			return nil, err
		}
		cube, err = manager.FlattenAnd([]ast.ASTHandle{post})
		if err != nil {
			return nil, err
		}
	}
	if len(cube) == 0 {
		truth, err := manager.MkTrue()
		if err != nil {
			return nil, err
		}
		cube = append(cube, truth)
	}
	cubeRefs, err := refsFromHandles(manager, cube)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, cubeRefs))
		}
	}()
	conjunction, err := manager.MkAndArray(cube)
	if err != nil {
		return nil, err
	}
	clause, err := manager.PushNot(conjunction)
	if err != nil {
		return nil, err
	}
	body, err := normalizeFormula(manager, config.Params, config.Globals, clause, true, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, manager.DelRef(&body))
		}
	}()
	bindings, err := obligation.Bindings()
	if err != nil {
		return nil, err
	}
	if len(bindings) != 0 {
		skolem, err := obligation.Skolems()
		if err != nil {
			return nil, err
		}
		for left, right := 0, len(skolem)-1; left < right; left, right = left+1, right-1 {
			skolem[left], skolem[right] = skolem[right], skolem[left]
		}
		handle, err := manager.RefGet(body)
		if err != nil {
			return nil, err
		}
		abstractor, err := ast.NewExprAbstractor(manager)
		if err != nil {
			return nil, err
		}
		if err = abstractor.Abstract(0, skolem, handle, body); err != nil {
			return nil, errors.Join(err, abstractor.Close())
		}
		if err = abstractor.Close(); err != nil {
			return nil, err
		}
		sorts := make([]ast.ASTHandle, len(skolem))
		names := make([]usymbol.Symbol, len(skolem))
		for i := range skolem {
			sorts[i], err = manager.GetSort(skolem[i])
			if err != nil {
				return nil, err
			}
			decl, declErr := manager.GetDecl(skolem[i])
			if declErr != nil {
				return nil, declErr
			}
			names[i], err = manager.GetASTName(decl)
			if err != nil {
				return nil, err
			}
		}
		abstracted, err := manager.RefGet(body)
		if err != nil {
			return nil, err
		}
		id, err := manager.GetASTID(abstracted)
		if err != nil {
			return nil, err
		}
		quantifier, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: abstracted, QID: usymbol.Numerical(id)})
		if err != nil {
			return nil, err
		}
		if err = manager.SetRef(body, quantifier); err != nil {
			return nil, err
		}
	}
	bindingRefs, err := refsFromHandles(manager, bindings)
	if err != nil {
		return nil, err
	}
	result := &Lemma{manager: manager, context: config.Params, globals: config.Globals, body: body, cube: cubeRefs, bindings: bindingRefs, level: level}
	committed = true
	return result, nil
}

func (l *Lemma) open() error {
	if l == nil || l.closed || l.manager == nil || l.context == nil || l.globals == nil || l.body == (ast.ASTRef{}) {
		return ErrManagerOwner
	}
	return nil
}

func (l *Lemma) Body() (ast.ASTHandle, error) {
	if err := l.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return l.manager.RefGet(l.body)
}

func (l *Lemma) Level() (uint32, error) {
	if err := l.open(); err != nil {
		return 0, err
	}
	return l.level, nil
}

func (l *Lemma) setLevel(level uint32) error {
	if err := l.open(); err != nil {
		return err
	}
	l.level = level
	return nil
}

func (l *Lemma) makeCube() (resultErr error) {
	if len(l.cube) != 0 {
		return nil
	}
	body, err := l.Body()
	if err != nil {
		return err
	}
	kind, err := l.manager.GetKind(body)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	negated, err := l.manager.PushNot(body)
	if err != nil {
		return err
	}
	parts, err := l.manager.FlattenAnd([]ast.ASTHandle{negated})
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		truth, trueErr := l.manager.MkTrue()
		if trueErr != nil {
			return trueErr
		}
		parts = append(parts, truth)
	}
	l.cube, err = refsFromHandles(l.manager, parts)
	if err != nil {
		return err
	}
	if err = normalizeLess(l.manager, l.cube); err != nil {
		resultErr = errors.Join(err, releaseNormalizeRefs(l.manager, l.cube))
		l.cube = nil
		return resultErr
	}
	return nil
}

func (l *Lemma) Cube() ([]ast.ASTHandle, error) {
	if err := l.open(); err != nil {
		return nil, err
	}
	if err := l.makeCube(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(l.cube))
	for i := range l.cube {
		handle, err := l.manager.RefGet(l.cube[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (l *Lemma) IsFalse() (bool, error) {
	if err := l.open(); err != nil {
		return false, err
	}
	if len(l.cube) == 1 {
		handle, err := l.manager.RefGet(l.cube[0])
		return l.manager.IsTrue(handle), err
	}
	body, err := l.Body()
	return l.manager.IsFalse(body), err
}

func (l *Lemma) AddBindings(bindings []ast.ASTHandle) error {
	if err := l.open(); err != nil {
		return err
	}
	refs, err := refsFromHandles(l.manager, bindings)
	if err != nil {
		return err
	}
	l.bindings = append(l.bindings, refs...)
	return nil
}

func (l *Lemma) bindingHandles() ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(l.bindings))
	for i := range l.bindings {
		handle, err := l.manager.RefGet(l.bindings[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (l *Lemma) Instances() (_ []ast.ASTRef, resultErr error) {
	body, err := l.Body()
	if err != nil {
		return nil, err
	}
	return l.instancesOf(body)
}

func (l *Lemma) instancesOf(body ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	kind, err := l.manager.GetKind(body)
	if err != nil || kind != ast.ASTQuantifier || len(l.bindings) == 0 {
		return nil, err
	}
	count, err := l.manager.GetNumDeclarations(body)
	if err != nil || count == 0 || len(l.bindings)%int(count) != 0 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	bindings, err := l.bindingHandles()
	if err != nil {
		return nil, err
	}
	var result []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(l.manager, result))
		}
	}()
	for offset := 0; offset < len(bindings); offset += int(count) {
		instance, newErr := l.manager.NewRef(ast.ASTHandle{})
		if newErr != nil {
			return nil, newErr
		}
		if newErr = astrewriter.InstantiateQuantifierWithOrder(l.manager, body, bindings[offset:offset+int(count)], false, instance); newErr != nil {
			_ = l.manager.DelRef(&instance)
			return nil, newErr
		}
		result = append(result, instance)
	}
	return result, nil
}

func (l *Lemma) Close() (result error) {
	if l == nil {
		return ErrManagerOwner
	}
	if l.closed {
		return nil
	}
	if l.manager == nil {
		return ErrManagerOwner
	}
	result = errors.Join(releaseNormalizeRefs(l.manager, l.bindings), releaseNormalizeRefs(l.manager, l.cube), l.manager.DelRef(&l.body))
	if result == nil {
		l.bindings = nil
		l.cube = nil
		l.manager = nil
		l.context = nil
		l.globals = nil
		l.closed = true
	}
	return result
}
