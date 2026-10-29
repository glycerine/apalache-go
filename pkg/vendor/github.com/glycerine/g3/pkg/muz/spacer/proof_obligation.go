package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp pob.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type ProofObligation struct {
	transformer *PredicateTransformer
	parent      *ProofObligation
	post        ast.ASTRef
	bindings    []ast.ASTRef
	newPost     ast.ASTRef
	level       uint32
	depth       uint32
	open        bool
	useFarkas   bool
	weakness    uint32
	children    []*ProofObligation
	derivation  *Derivation
	released    bool
	sequence    uint64
}

func NewProofObligation(parent *ProofObligation, transformer *PredicateTransformer, level, depth uint32, post ast.ASTHandle, bindings []ast.ASTHandle) (_ *ProofObligation, resultErr error) {
	if transformer == nil || transformer.open() != nil {
		return nil, ErrManagerOwner
	}
	if parent != nil && (parent.released || parent.transformer == nil || parent.transformer.owner != transformer.owner) {
		return nil, ErrManagerOwner
	}
	sequence, err := transformer.owner.Unique()
	if err != nil {
		return nil, err
	}
	p := &ProofObligation{transformer: transformer, parent: parent, level: level, depth: depth, open: true, useFarkas: true, sequence: uint64(sequence)}
	if err = p.SetPost(post, bindings); err != nil {
		return nil, errors.Join(err, p.Release())
	}
	if parent != nil {
		parent.children = append(parent.children, p)
	}
	return p, nil
}

func (p *ProofObligation) valid() error {
	if p == nil || p.released || p.transformer == nil || p.transformer.open() != nil || p.post == (ast.ASTRef{}) {
		return ErrManagerOwner
	}
	return nil
}

func (p *ProofObligation) Post() (ast.ASTHandle, error) {
	if err := p.valid(); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.transformer.manager.RefGet(p.post)
}

func (p *ProofObligation) Level() (uint32, error) {
	if err := p.valid(); err != nil {
		return 0, err
	}
	return p.level, nil
}

func (p *ProofObligation) Depth() (uint32, error) {
	if err := p.valid(); err != nil {
		return 0, err
	}
	return p.depth, nil
}

func (p *ProofObligation) Parent() (*ProofObligation, error) {
	if err := p.valid(); err != nil {
		return nil, err
	}
	return p.parent, nil
}

func (p *ProofObligation) SetDerivation(derivation *Derivation) error {
	if err := p.valid(); err != nil {
		return err
	}
	if derivation != nil && (derivation.closed || (derivation.parent != p && p.parent != derivation.parent)) {
		return ErrManagerOwner
	}
	if p.derivation != nil && p.derivation != derivation {
		if err := p.derivation.Close(); err != nil {
			return err
		}
	}
	p.derivation = derivation
	return nil
}

func (p *ProofObligation) Derivation() (*Derivation, bool, error) {
	if err := p.valid(); err != nil {
		return nil, false, err
	}
	return p.derivation, p.derivation != nil, nil
}

func (p *ProofObligation) DetachDerivation() (*Derivation, error) {
	if err := p.valid(); err != nil {
		return nil, err
	}
	result := p.derivation
	p.derivation = nil
	return result, nil
}

func (p *ProofObligation) SetPost(post ast.ASTHandle, bindings []ast.ASTHandle) (resultErr error) {
	if p == nil || p.released || p.transformer == nil || p.transformer.open() != nil {
		return ErrManagerOwner
	}
	manager := p.transformer.manager
	config := p.transformer.owner.contexts[0].config
	normalized, err := normalizeFormula(manager, config.Params, config.Globals, post, p.transformer.simplifyProofObligations, p.transformer.useEqualityClasses)
	if err != nil {
		return err
	}
	newBindings, err := refsFromHandles(manager, bindings)
	if err != nil {
		return errors.Join(err, manager.DelRef(&normalized))
	}
	if err = normalizeLess(manager, newBindings); err != nil {
		return errors.Join(err, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
	}
	if len(newBindings) != 0 {
		replacer, replaceErr := astrewriter.NewExprSafeReplace(manager)
		if replaceErr != nil {
			return errors.Join(replaceErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
		}
		defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
		for i := range newBindings {
			binding, getErr := manager.RefGet(newBindings[i])
			if getErr != nil {
				return errors.Join(getErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
			}
			sort, sortErr := manager.GetSort(binding)
			if sortErr != nil {
				return errors.Join(sortErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
			}
			name, nameErr := manager.Symbols.Intern(fmt.Sprintf("sk!%d", i))
			if nameErr != nil {
				return errors.Join(nameErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
			}
			skolem, skolemErr := manager.MkConst(name, sort)
			if skolemErr != nil {
				return errors.Join(skolemErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
			}
			if insertErr := replacer.Insert(binding, skolem); insertErr != nil {
				return errors.Join(insertErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
			}
		}
		handle, getErr := manager.RefGet(normalized)
		if getErr != nil {
			return errors.Join(getErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
		}
		if replaceErr = replacer.Apply(handle, normalized); replaceErr != nil {
			return errors.Join(replaceErr, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
		}
	}
	if p.post != (ast.ASTRef{}) {
		if err = manager.DelRef(&p.post); err != nil {
			return errors.Join(err, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
		}
	}
	if err = releaseNormalizeRefs(manager, p.bindings); err != nil {
		return errors.Join(err, releaseNormalizeRefs(manager, newBindings), manager.DelRef(&normalized))
	}
	p.post = normalized
	p.bindings = newBindings
	return nil
}

func (p *ProofObligation) Bindings() ([]ast.ASTHandle, error) {
	if err := p.valid(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(p.bindings))
	for i := range p.bindings {
		handle, err := p.transformer.manager.RefGet(p.bindings[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (p *ProofObligation) Skolems() ([]ast.ASTHandle, error) {
	bindings, err := p.Bindings()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(bindings))
	for i, binding := range bindings {
		sort, err := p.transformer.manager.GetSort(binding)
		if err != nil {
			return nil, err
		}
		name, err := p.transformer.manager.Symbols.Intern(fmt.Sprintf("sk!%d", i))
		if err != nil {
			return nil, err
		}
		result[i], err = p.transformer.manager.MkConst(name, sort)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *ProofObligation) IsGround() (bool, error) {
	if err := p.valid(); err != nil {
		return false, err
	}
	return len(p.bindings) == 0, nil
}

func (p *ProofObligation) NewPost(post ast.ASTHandle) error {
	if err := p.valid(); err != nil {
		return err
	}
	current, err := p.Post()
	if err != nil || current == post {
		return err
	}
	if p.newPost != (ast.ASTRef{}) {
		if err = p.transformer.manager.DelRef(&p.newPost); err != nil {
			return err
		}
	}
	p.newPost, err = p.transformer.manager.NewRef(post)
	return err
}

func (p *ProofObligation) Dirty() (bool, error) {
	if err := p.valid(); err != nil {
		return false, err
	}
	return p.newPost != (ast.ASTRef{}), nil
}

func (p *ProofObligation) Clean() error {
	if err := p.valid(); err != nil {
		return err
	}
	if p.newPost == (ast.ASTRef{}) {
		return nil
	}
	manager := p.transformer.manager
	if err := manager.DelRef(&p.post); err != nil {
		return err
	}
	p.post = p.newPost
	p.newPost = ast.ASTRef{}
	return nil
}

func (p *ProofObligation) Reset() error {
	if err := p.Clean(); err != nil {
		return err
	}
	p.open = true
	if p.derivation != nil {
		if err := p.derivation.Close(); err != nil {
			return err
		}
		p.derivation = nil
	}
	return nil
}

func (p *ProofObligation) Close() error {
	if err := p.valid(); err != nil {
		return err
	}
	if !p.open {
		return nil
	}
	if err := p.Reset(); err != nil {
		return err
	}
	p.open = false
	for _, child := range p.children {
		if err := child.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *ProofObligation) IsClosed() (bool, error) {
	if err := p.valid(); err != nil {
		return false, err
	}
	return !p.open, nil
}

func (p *ProofObligation) IncrementLevel() error {
	if err := p.valid(); err != nil {
		return err
	}
	p.level++
	p.depth++
	p.weakness = 0
	return nil
}

func (p *ProofObligation) Weakness() (uint32, error) {
	if err := p.valid(); err != nil {
		return 0, err
	}
	return p.weakness, nil
}

func (p *ProofObligation) BumpWeakness() error {
	if err := p.valid(); err != nil {
		return err
	}
	p.weakness++
	return nil
}

func (p *ProofObligation) UseFarkas() (bool, error) {
	if err := p.valid(); err != nil {
		return false, err
	}
	return p.useFarkas, nil
}

func (p *ProofObligation) SetUseFarkas(value bool) error {
	if err := p.valid(); err != nil {
		return err
	}
	p.useFarkas = value
	return nil
}

func (p *ProofObligation) inherit(other *ProofObligation) error {
	if err := p.valid(); err != nil {
		return err
	}
	if other == nil || other.valid() != nil || p.parent != other.parent || p.transformer != other.transformer || p.newPost != (ast.ASTRef{}) {
		return ErrManagerOwner
	}
	left, err := p.Post()
	if err != nil {
		return err
	}
	right, err := other.Post()
	if err != nil || left != right {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	bindings, err := other.Bindings()
	if err != nil {
		return err
	}
	refs, err := refsFromHandles(p.transformer.manager, bindings)
	if err != nil {
		return err
	}
	if err = releaseNormalizeRefs(p.transformer.manager, p.bindings); err != nil {
		return errors.Join(err, releaseNormalizeRefs(p.transformer.manager, refs))
	}
	p.bindings = refs
	p.level = other.level
	p.depth = other.depth
	p.open = other.open
	p.useFarkas = other.useFarkas
	p.weakness = other.weakness
	return nil
}

func (p *ProofObligation) Release() (result error) {
	if p == nil {
		return ErrManagerOwner
	}
	if p.released {
		return nil
	}
	if p.transformer == nil || p.transformer.manager == nil {
		return ErrManagerOwner
	}
	children := append([]*ProofObligation(nil), p.children...)
	for i := len(children) - 1; i >= 0; i-- {
		result = errors.Join(result, children[i].Release())
	}
	if result != nil {
		return result
	}
	manager := p.transformer.manager
	if p.derivation != nil {
		result = errors.Join(result, p.derivation.Close())
		p.derivation = nil
	}
	if p.newPost != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&p.newPost))
	}
	result = errors.Join(result, releaseNormalizeRefs(manager, p.bindings))
	if p.post != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&p.post))
	}
	if result != nil {
		return result
	}
	if p.parent != nil {
		for i, child := range p.parent.children {
			if child == p {
				copy(p.parent.children[i:], p.parent.children[i+1:])
				p.parent.children = p.parent.children[:len(p.parent.children)-1]
				break
			}
		}
	}
	p.bindings = nil
	p.parent = nil
	p.transformer = nil
	p.released = true
	return nil
}
