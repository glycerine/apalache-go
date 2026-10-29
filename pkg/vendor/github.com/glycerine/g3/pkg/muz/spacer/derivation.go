package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp derivation
// and derivation::premise ownership and state updates.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
)

type DerivationPremise struct {
	transformer *PredicateTransformer
	oldIndex    uint32
	summary     ast.ASTRef
	variables   []ast.ASTRef
	must        bool
}

func newDerivationPremise(transformer *PredicateTransformer, oldIndex uint32, summary ast.ASTHandle, must bool, auxiliaries []ast.ASTHandle) (_ *DerivationPremise, resultErr error) {
	if transformer == nil || transformer.open() != nil {
		return nil, ErrManagerOwner
	}
	premise := &DerivationPremise{transformer: transformer, oldIndex: oldIndex, must: must}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, premise.close())
		}
	}()
	var err error
	premise.summary, err = transformer.manager.NewRef(summary)
	if err != nil {
		return nil, err
	}
	if err = premise.rebuildVariables(auxiliaries); err != nil {
		return nil, err
	}
	return premise, nil
}

func (p *DerivationPremise) rebuildVariables(auxiliaries []ast.ASTHandle) (resultErr error) {
	if p == nil || p.transformer == nil {
		return ErrManagerOwner
	}
	manager := p.transformer.manager
	var next []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, next))
		}
	}()
	signature, err := p.transformer.Signature()
	if err != nil {
		return err
	}
	for _, declaration := range signature {
		origin, err := p.transformer.owner.OldToOld(declaration, 0, p.oldIndex)
		if err != nil {
			return err
		}
		value, err := manager.MkApp(origin, nil)
		if err != nil {
			return err
		}
		reference, err := manager.NewRef(value)
		if err != nil {
			return err
		}
		next = append(next, reference)
	}
	for _, auxiliary := range auxiliaries {
		declaration, err := manager.GetDecl(auxiliary)
		if err != nil {
			return err
		}
		origin, err := p.transformer.owner.NewToOld(declaration, p.oldIndex)
		if err != nil {
			return err
		}
		value, err := manager.MkApp(origin, nil)
		if err != nil {
			return err
		}
		reference, err := manager.NewRef(value)
		if err != nil {
			return err
		}
		next = append(next, reference)
	}
	if err = releaseNormalizeRefs(manager, p.variables); err != nil {
		return err
	}
	p.variables = next
	next = nil
	return nil
}

func (p *DerivationPremise) SetSummary(summary ast.ASTHandle, must bool, auxiliaries []ast.ASTHandle) error {
	if p == nil || p.transformer == nil {
		return ErrManagerOwner
	}
	converted, err := p.transformer.owner.FormulaNewToOld(summary, p.oldIndex, false)
	if err != nil {
		return err
	}
	if err = p.rebuildVariables(auxiliaries); err != nil {
		return errors.Join(err, p.transformer.manager.DelRef(&converted))
	}
	if err = p.transformer.manager.DelRef(&p.summary); err != nil {
		return errors.Join(err, p.transformer.manager.DelRef(&converted))
	}
	p.summary = converted
	p.must = must
	return nil
}

func (p *DerivationPremise) Summary() (ast.ASTHandle, error) {
	if p == nil || p.transformer == nil {
		return ast.ASTHandle{}, ErrManagerOwner
	}
	return p.transformer.manager.RefGet(p.summary)
}

func (p *DerivationPremise) Variables() ([]ast.ASTHandle, error) {
	if p == nil || p.transformer == nil {
		return nil, ErrManagerOwner
	}
	result := make([]ast.ASTHandle, len(p.variables))
	for i := range p.variables {
		var err error
		result[i], err = p.transformer.manager.RefGet(p.variables[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *DerivationPremise) Transformer() *PredicateTransformer { return p.transformer }
func (p *DerivationPremise) OldIndex() uint32                   { return p.oldIndex }
func (p *DerivationPremise) Must() bool                         { return p.must }

func (p *DerivationPremise) close() error {
	if p == nil || p.transformer == nil {
		return nil
	}
	manager := p.transformer.manager
	result := releaseNormalizeRefs(manager, p.variables)
	if p.summary != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&p.summary))
	}
	if result == nil {
		p.variables = nil
		p.transformer = nil
	}
	return result
}

type Derivation struct {
	parent       *ProofObligation
	rule         *base.Rule
	premises     []*DerivationPremise
	active       uint32
	transition   ast.ASTRef
	existentials []ast.ASTRef
	closed       bool
}

func NewDerivation(parent *ProofObligation, rule *base.Rule, transition ast.ASTHandle, existentials []ast.ASTHandle) (_ *Derivation, resultErr error) {
	if parent == nil || parent.valid() != nil || rule == nil {
		return nil, ErrManagerOwner
	}
	ownedRule, err := rule.Retain()
	if err != nil {
		return nil, err
	}
	result := &Derivation{parent: parent, rule: ownedRule}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	result.transition, err = parent.transformer.manager.NewRef(transition)
	if err != nil {
		return nil, err
	}
	result.existentials, err = refsFromHandles(parent.transformer.manager, existentials)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Derivation) open() error {
	if d == nil || d.closed || d.parent == nil || d.parent.valid() != nil || d.rule == nil {
		return ErrManagerOwner
	}
	return nil
}

func (d *Derivation) AddPremise(transformer *PredicateTransformer, oldIndex uint32, summary ast.ASTHandle, must bool, auxiliaries []ast.ASTHandle) error {
	if err := d.open(); err != nil {
		return err
	}
	if transformer == nil || transformer.owner != d.parent.transformer.owner {
		return ErrManagerOwner
	}
	premise, err := newDerivationPremise(transformer, oldIndex, summary, must, auxiliaries)
	if err != nil {
		return err
	}
	d.premises = append(d.premises, premise)
	return nil
}

func (d *Derivation) Premises() ([]*DerivationPremise, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	return append([]*DerivationPremise(nil), d.premises...), nil
}

func (d *Derivation) Parent() (*ProofObligation, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	return d.parent, nil
}

func (d *Derivation) Rule() (*base.Rule, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	return d.rule.Retain()
}

func (d *Derivation) Transition() (ast.ASTHandle, error) {
	if err := d.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return d.parent.transformer.manager.RefGet(d.transition)
}

func (d *Derivation) project(force bool, md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (result ast.ASTRef, remaining []ast.ASTRef, resultErr error) {
	manager := d.parent.transformer.manager
	defer func() {
		if resultErr != nil {
			if result != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, manager.DelRef(&result))
			}
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, remaining))
			remaining = nil
		}
	}()
	forms, err := manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	formRefs, err := refsFromHandles(manager, forms)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, formRefs)) }()
	variableRefs, err := refsFromHandles(manager, variables)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	committedVariables := false
	defer func() {
		if !committedVariables {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, variableRefs))
		}
	}()
	config := d.parent.transformer.owner.contexts[0].config.ModelEvaluator
	if !d.parent.transformer.owner.useNativeMBP {
		conjunction, err := manager.MkAndArray(forms)
		if err != nil {
			return ast.ASTRef{}, nil, err
		}
		projector, err := qe.NewLegacyProjection(manager, qe.Config{Context: config.Context, Globals: config.Globals, Cooperation: config.Cooperation, Trace: config.Trace})
		if err != nil {
			return ast.ASTRef{}, nil, err
		}
		projected, err := projector.Project(md, variables, conjunction, true, !force)
		if err != nil {
			return ast.ASTRef{}, nil, errors.Join(err, projector.Close())
		}
		projectedFormula, err := projected.Formula()
		if err == nil {
			result, err = manager.NewRef(projectedFormula)
		}
		var projectedVariables []ast.ASTHandle
		if err == nil {
			projectedVariables, err = projected.Variables()
		}
		if err == nil {
			remaining, err = refsFromHandles(manager, projectedVariables)
		}
		err = errors.Join(err, projected.Close(), projector.Close())
		if err != nil {
			if result != (ast.ASTRef{}) {
				err = errors.Join(err, manager.DelRef(&result))
			}
			err = errors.Join(err, releaseNormalizeRefs(manager, remaining))
			remaining = nil
			return ast.ASTRef{}, nil, err
		}
		return result, remaining, nil
	}
	projector, err := qe.NewModelBasedProjection(manager, qe.Config{Context: config.Context, Globals: config.Globals, Cooperation: config.Cooperation, Trace: config.Trace})
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, projector.Close()) }()
	if len(variableRefs) != 0 {
		if err = projector.Project(force, &variableRefs, md, &formRefs); err != nil {
			return ast.ASTRef{}, nil, err
		}
	}
	projected := make([]ast.ASTHandle, len(formRefs))
	for i := range formRefs {
		projected[i], err = manager.RefGet(formRefs[i])
		if err != nil {
			return ast.ASTRef{}, nil, err
		}
	}
	conjunction, err := manager.MkAndArray(projected)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	result, err = manager.NewRef(conjunction)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	committedVariables = true
	remaining = variableRefs
	return result, remaining, nil
}

func (d *Derivation) appendExistentials(values []ast.ASTRef) {
	d.existentials = append(d.existentials, values...)
}

func previousSpacerLevel(level uint32) uint32 {
	if level == 0 || level == InfinityLevel {
		return level
	}
	return level - 1
}

// CreateFirstChild uses md to skip must premises and construct the first may
// premise's projected proof obligation.
func (d *Derivation) CreateFirstChild(md *model.ModelCore) (*ProofObligation, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	if md == nil || md.Manager() != d.parent.transformer.manager {
		return nil, ast.ErrASTHandle
	}
	if len(d.premises) == 0 {
		return nil, nil
	}
	d.active = 0
	return d.createProjectedChild(md)
}

func (d *Derivation) createProjectedChild(md *model.ModelCore) (_ *ProofObligation, resultErr error) {
	manager := d.parent.transformer.manager
	var summaries []ast.ASTHandle
	var variables []ast.ASTHandle
	for d.active < uint32(len(d.premises)) && d.premises[d.active].must {
		premise := d.premises[d.active]
		summary, err := premise.Summary()
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
		values, err := premise.Variables()
		if err != nil {
			return nil, err
		}
		variables = append(variables, values...)
		d.active++
	}
	if d.active >= uint32(len(d.premises)) {
		return nil, nil
	}
	transition, err := manager.RefGet(d.transition)
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, transition)
	preFormula, err := manager.MkAndArray(summaries)
	if err != nil {
		return nil, err
	}
	preimage, remaining, err := d.project(d.parent.transformer.groundCTI, md, variables, preFormula)
	if err != nil {
		return nil, err
	}
	if err = manager.DelRef(&d.transition); err != nil {
		return nil, errors.Join(err, manager.DelRef(&preimage), releaseNormalizeRefs(manager, remaining))
	}
	d.transition = preimage
	d.appendExistentials(remaining)
	active := d.premises[d.active]
	activeSummary, err := active.Summary()
	if err != nil {
		return nil, err
	}
	config := d.parent.transformer.owner.contexts[0].config.ModelEvaluator
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	evaluated, err := evaluator.EvaluateValue(activeSummary)
	if err != nil {
		return nil, err
	}
	value, err := manager.RefGet(evaluated)
	isTrue := err == nil && manager.IsTrue(value)
	err = errors.Join(err, manager.DelRef(&evaluated))
	if err != nil {
		return nil, err
	}
	if !isTrue {
		return nil, nil
	}
	summaries = summaries[:0]
	variables = variables[:0]
	for index := d.active + 1; index < uint32(len(d.premises)); index++ {
		premise := d.premises[index]
		summary, err := premise.Summary()
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
		values, err := premise.Variables()
		if err != nil {
			return nil, err
		}
		variables = append(variables, values...)
	}
	transition, err = manager.RefGet(d.transition)
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, transition)
	postFormula, err := manager.MkAndArray(summaries)
	if err != nil {
		return nil, err
	}
	postRef, remaining, err := d.project(d.parent.transformer.groundCTI, md, variables, postFormula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&postRef)) }()
	d.appendExistentials(remaining)
	post, err := manager.RefGet(postRef)
	if err != nil {
		return nil, err
	}
	currentRef, err := d.parent.transformer.owner.FormulaOldToNew(post, active.oldIndex, len(d.existentials) == 0)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&currentRef)) }()
	current, err := manager.RefGet(currentRef)
	if err != nil {
		return nil, err
	}
	bindings := make([]ast.ASTHandle, len(d.existentials))
	for i := range d.existentials {
		bindings[i], err = manager.RefGet(d.existentials[i])
		if err != nil {
			return nil, err
		}
	}
	level, err := d.parent.Level()
	if err != nil {
		return nil, err
	}
	depth, err := d.parent.Depth()
	if err != nil {
		return nil, err
	}
	return active.transformer.NewProofObligation(d.parent, previousSpacerLevel(level), depth, current, bindings)
}

func (d *Derivation) modelImplicant(md *model.ModelCore, formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	manager := d.parent.transformer.manager
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
		}
	}()
	if md == nil || md.DerivedModel() == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	config := d.parent.transformer.owner.contexts[0].config.ModelEvaluator
	implicant, err := model.NewModelImplicant(manager, config)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, implicant.Close()) }()
	literals, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md.DerivedModel())
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, literals)) }()
	handles := make([]ast.ASTHandle, len(literals))
	for i := range literals {
		handles[i], err = manager.RefGet(literals[i])
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	conjunction, err := manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, resultErr = manager.NewRef(conjunction)
	return
}

// CreateNextChild strengthens the active premise with a selected must fact,
// projects its current-state variables, and advances to the next may premise.
func (d *Derivation) CreateNextChild() (_ *ProofObligation, resultErr error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	if d.active+1 >= uint32(len(d.premises)) {
		return nil, nil
	}
	manager := d.parent.transformer.manager
	active := d.premises[d.active]
	var summaries []ast.ASTHandle
	for index := d.active + 1; index < uint32(len(d.premises)); index++ {
		summary, err := d.premises[index].Summary()
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	transition, err := manager.RefGet(d.transition)
	if err != nil {
		return nil, err
	}
	activeTransitionRef, err := d.parent.transformer.owner.FormulaOldToNew(transition, active.oldIndex, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&activeTransitionRef)) }()
	activeTransition, err := manager.RefGet(activeTransitionRef)
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, activeTransition)
	state, err := manager.MkAndArray(summaries)
	if err != nil {
		return nil, err
	}
	reachable, md, err := active.transformer.IsMustReachable(state, true)
	if err != nil || !reachable || md == nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	fact, found, err := active.transformer.UsedReachFact(md, true)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, fact.Close()) }()
	factFormula, err := fact.Fact()
	if err != nil {
		return nil, err
	}
	implicantRef, err := d.modelImplicant(md, factFormula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&implicantRef)) }()
	implicant, err := manager.RefGet(implicantRef)
	if err != nil {
		return nil, err
	}
	auxiliaries, err := fact.Auxiliaries()
	if err != nil {
		return nil, err
	}
	if err = active.SetSummary(implicant, true, auxiliaries); err != nil {
		return nil, err
	}
	combined, err := manager.MkAnd(implicant, activeTransition)
	if err != nil {
		return nil, err
	}
	variables := append([]ast.ASTHandle(nil), auxiliaries...)
	signature, err := active.transformer.Signature()
	if err != nil {
		return nil, err
	}
	for _, declaration := range signature {
		current, err := active.transformer.owner.OldToNew(declaration, 0)
		if err != nil {
			return nil, err
		}
		value, err := manager.MkApp(current, nil)
		if err != nil {
			return nil, err
		}
		variables = append(variables, value)
	}
	projected, remaining, err := d.project(d.parent.transformer.groundCTI, md, variables, combined)
	if err != nil {
		return nil, err
	}
	if err = manager.DelRef(&d.transition); err != nil {
		return nil, errors.Join(err, manager.DelRef(&projected), releaseNormalizeRefs(manager, remaining))
	}
	d.transition = projected
	d.appendExistentials(remaining)
	d.active++
	return d.createProjectedChild(md)
}

func (d *Derivation) Close() (result error) {
	if d == nil || d.closed {
		return nil
	}
	manager := d.parent.transformer.manager
	for i := len(d.premises) - 1; i >= 0; i-- {
		result = errors.Join(result, d.premises[i].close())
	}
	result = errors.Join(result, releaseNormalizeRefs(manager, d.existentials))
	if d.transition != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&d.transition))
	}
	result = errors.Join(result, d.rule.Close())
	if result == nil {
		d.premises = nil
		d.existentials = nil
		d.rule = nil
		d.parent = nil
		d.closed = true
	}
	return result
}
