package normal_forms

// Source: src/ast/normal_forms/nnf.cpp:34-930;
// src/ast/normal_forms/nnf.h:26-49.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type NNFMode uint8

const (
	NNFSkolem NNFMode = iota
	NNFQuantifiers
	NNFFull
)

type NNFOptions struct {
	Mode         NNFMode
	IgnoreLabels bool
	Skolemize    bool
	SkolemHack   bool
	MaxMemoryMB  uint32
}

func DefaultNNFOptions() NNFOptions {
	return NNFOptions{Mode: NNFSkolem, Skolemize: true, MaxMemoryMB: ^uint32(0)}
}

type nnfResult struct{ value, proof ast.ASTHandle }
type nnfCacheEntry struct {
	key                  ast.ASTHandle
	keyRef, value, proof ast.ASTRef
}

// NNF performs polarity-aware negation-normal-form conversion and
// skolemization, retaining the source's four (polarity, quantifier-scope)
// caches and delayed processing of definitions introduced by expression names.
type NNF struct {
	self       *NNF
	manager    *ast.ASTManager
	names      *DefinedNames
	skolemizer *Skolemizer
	nested     *ExprNamer
	quant      *ExprNamer
	options    NNFOptions
	maxMemory  uint64
	caches     [4]map[ast.ASTHandle]nnfCacheEntry
	scratch    []ast.ASTRef
	todoDefs   []ast.ASTRef
	todoProofs []ast.ASTRef
	closed     bool
}

func NewNNF(m *ast.ASTManager, names *DefinedNames, options NNFOptions) (*NNF, error) {
	if m == nil || names == nil || names.manager != m || names.valid() != nil || options.Mode > NNFFull {
		return nil, ast.ErrASTHandle
	}
	n := &NNF{manager: m, names: names}
	n.self = n
	for i := range n.caches {
		n.caches[i] = make(map[ast.ASTHandle]nnfCacheEntry)
	}
	var e error
	n.skolemizer, e = NewSkolemizer(m)
	if e != nil {
		return n, e
	}
	n.nested, e = NewNestedFormulaNamer(m, names)
	if e != nil {
		return n, e
	}
	n.quant, e = NewQuantifierLabelNamer(m, names)
	if e != nil {
		return n, e
	}
	if e = n.UpdateOptions(options); e != nil {
		return n, e
	}
	return n, nil
}

func NewNNFFromParams(m *ast.ASTManager, names *DefinedNames, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*NNF, error) {
	params, e := NewNNFParams(c, g, p)
	if e != nil {
		return nil, e
	}
	options, e := nnfOptionsFromParams(params)
	e = errors.Join(e, params.Close())
	if e != nil {
		return nil, e
	}
	return NewNNF(m, names, options)
}

func nnfOptionsFromParams(p *NNFParams) (NNFOptions, error) {
	o := DefaultNNFOptions()
	mode, e := p.Mode()
	if e != nil {
		return o, e
	}
	spelling, e := p.context.SymbolsField.Display(mode)
	if e != nil {
		return o, e
	}
	switch spelling {
	case "skolem":
		o.Mode = NNFSkolem
	case "quantifiers":
		o.Mode = NNFQuantifiers
	case "full":
		o.Mode = NNFFull
	default:
		return o, ast.ErrASTDomain
	}
	if o.IgnoreLabels, e = p.IgnoreLabels(); e != nil {
		return o, e
	}
	if o.Skolemize, e = p.Skolemize(); e != nil {
		return o, e
	}
	if o.SkolemHack, e = p.SkHack(); e != nil {
		return o, e
	}
	if o.MaxMemoryMB, e = p.MaxMemory(); e != nil {
		return o, e
	}
	return o, nil
}

func (n *NNF) open() error {
	if n == nil || n.self != n || n.closed || n.manager == nil || n.names == nil || n.names.valid() != nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (n *NNF) UpdateOptions(options NNFOptions) error {
	if n == nil || n.self != n || n.closed {
		return ast.ErrASTHandle
	}
	if options.Mode > NNFFull {
		return ast.ErrASTDomain
	}
	n.options = options
	n.maxMemory = memory.MegabytesToBytes(options.MaxMemoryMB)
	return n.skolemizer.SetSkolemHack(options.SkolemHack)
}

func (n *NNF) checkpoint() error {
	if n.manager.Budget().Used() > n.maxMemory {
		return z3exception.NewDefaultException(messages.MaxMemory)
	}
	canceled, e := n.manager.Canceled()
	if e != nil {
		return e
	}
	if canceled {
		return z3exception.NewDefaultException(messages.Canceled)
	}
	return nil
}

func nnfCacheIndex(polarity, inQuantifier bool) int {
	i := 0
	if polarity {
		i++
	}
	if inQuantifier {
		i += 2
	}
	return i
}

func (n *NNF) hold(h ast.ASTHandle) error {
	if h.IsNull() {
		return nil
	}
	r, e := n.manager.NewRef(h)
	if e != nil {
		return e
	}
	n.scratch = append(n.scratch, r)
	return nil
}

func (n *NNF) cacheResult(key ast.ASTHandle, polarity, inQuantifier bool, r nnfResult) error {
	i := nnfCacheIndex(polarity, inQuantifier)
	if _, found := n.caches[i][key]; found {
		return nil
	}
	e := nnfCacheEntry{key: key}
	var x error
	if e.keyRef, x = n.manager.NewRef(key); x != nil {
		return x
	}
	if e.value, x = n.manager.NewRef(r.value); x != nil {
		return errors.Join(x, n.manager.DelRef(&e.keyRef))
	}
	if !r.proof.IsNull() {
		if e.proof, x = n.manager.NewRef(r.proof); x != nil {
			return errors.Join(x, n.manager.DelRef(&e.value), n.manager.DelRef(&e.keyRef))
		}
	}
	n.caches[i][key] = e
	return nil
}

func (n *NNF) cached(key ast.ASTHandle, polarity, inQuantifier bool) (nnfResult, bool, error) {
	e, found := n.caches[nnfCacheIndex(polarity, inQuantifier)][key]
	if !found {
		return nnfResult{}, false, nil
	}
	v, x := n.manager.RefGet(e.value)
	if x != nil {
		return nnfResult{}, false, x
	}
	var p ast.ASTHandle
	if e.proof != (ast.ASTRef{}) {
		p, x = n.manager.RefGet(e.proof)
	}
	return nnfResult{v, p}, true, x
}

func (n *NNF) skip(t ast.ASTHandle, polarity, proofs bool) (nnfResult, error) {
	r := t
	var e error
	if !polarity {
		r, e = n.manager.MkNot(t)
		if e != nil {
			return nnfResult{}, e
		}
		if e = n.hold(r); e != nil {
			return nnfResult{}, e
		}
	}
	var p ast.ASTHandle
	if proofs {
		p, e = n.manager.MkProofOEqReflexivity(r)
		if e != nil {
			return nnfResult{}, e
		}
		if e = n.hold(p); e != nil {
			return nnfResult{}, e
		}
	}
	return nnfResult{r, p}, nil
}

func (n *NNF) mkProof(polarity bool, old, value ast.ASTHandle, parents []ast.ASTHandle) (ast.ASTHandle, error) {
	if polarity {
		od, e := n.manager.GetDecl(old)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		nd, e := n.manager.GetDecl(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if od == nd {
			return n.manager.MkProofOEqCongruence(old, value, parents)
		}
		return n.manager.MkProofNNFPos(old, value, parents)
	}
	return n.manager.MkProofNNFNeg(old, value, parents)
}

func (n *NNF) convert(t ast.ASTHandle, polarity, inQuantifier, proofs bool) (nnfResult, error) {
	if e := n.checkpoint(); e != nil {
		return nnfResult{}, e
	}
	m := n.manager
	if n.options.Mode == NNFSkolem || (n.options.Mode == NNFQuantifiers && !inQuantifier) {
		hasQ, e := m.HasQuantifiers(t)
		if e != nil {
			return nnfResult{}, e
		}
		hasL, e := m.HasLabels(t)
		if e != nil {
			return nnfResult{}, e
		}
		if !hasQ && !hasL {
			return n.skip(t, polarity, proofs)
		}
	}
	rc, e := m.GetRefCount(t)
	if e != nil {
		return nnfResult{}, e
	}
	cache := rc > 1
	if cache {
		if r, found, x := n.cached(t, polarity, inQuantifier); x != nil || found {
			return r, x
		}
	}
	kind, e := m.GetKind(t)
	if e != nil {
		return nnfResult{}, e
	}
	var r nnfResult
	switch kind {
	case ast.ASTVar:
		r, e = n.skip(t, polarity, proofs)
	case ast.ASTQuantifier:
		r, e = n.convertQuantifier(t, polarity, inQuantifier, proofs)
	case ast.ASTApp:
		count, x := m.GetNumArgs(t)
		if x != nil {
			return nnfResult{}, x
		}
		if count == 0 {
			r, e = n.skip(t, polarity, proofs)
		} else {
			r, e = n.convertApp(t, polarity, inQuantifier, proofs)
		}
	default:
		e = ast.ErrASTDomain
	}
	if e != nil {
		return nnfResult{}, e
	}
	if e = n.hold(r.value); e != nil {
		return nnfResult{}, e
	}
	if e = n.hold(r.proof); e != nil {
		return nnfResult{}, e
	}
	if cache {
		e = n.cacheResult(t, polarity, inQuantifier, r)
	}
	return r, e
}

func (n *NNF) child(t ast.ASTHandle, i uint32, polarity, inQ, proofs bool) (nnfResult, error) {
	h, e := n.manager.GetArg(t, i)
	if e != nil {
		return nnfResult{}, e
	}
	return n.convert(h, polarity, inQ, proofs)
}

func (n *NNF) proofParents(rs []nnfResult) []ast.ASTHandle {
	ps := make([]ast.ASTHandle, len(rs))
	for i := range rs {
		ps[i] = rs[i].proof
	}
	return ps
}

func (n *NNF) convertApp(t ast.ASTHandle, polarity, inQ, proofs bool) (nnfResult, error) {
	m := n.manager
	family, e := m.GetFamilyID(t)
	if e != nil {
		return nnfResult{}, e
	}
	op, e := m.GetDeclKind(t)
	if e != nil {
		return nnfResult{}, e
	}
	if family == 0 {
		switch op {
		case ast.ASTOpAnd, ast.ASTOpOr:
			count, x := m.GetNumArgs(t)
			if x != nil {
				return nnfResult{}, x
			}
			children := make([]nnfResult, count)
			args := make([]ast.ASTHandle, count)
			for i := uint32(0); i < count; i++ {
				children[i], x = n.child(t, i, polarity, inQ, proofs)
				if x != nil {
					return nnfResult{}, x
				}
				args[i] = children[i].value
			}
			isAnd := op == ast.ASTOpAnd
			var value ast.ASTHandle
			if isAnd == polarity {
				value, x = m.MkAndArray(args)
			} else {
				value, x = m.MkOrArray(args)
			}
			if x != nil {
				return nnfResult{}, x
			}
			var proof ast.ASTHandle
			if proofs {
				proof, x = n.mkProof(polarity, t, value, n.proofParents(children))
			}
			return nnfResult{value, proof}, x
		case ast.ASTOpNot:
			child, x := n.child(t, 0, !polarity, inQ, proofs)
			if x != nil {
				return nnfResult{}, x
			}
			if proofs && !polarity {
				child.proof, x = m.MkProofNNFNeg(t, child.value, []ast.ASTHandle{child.proof})
			}
			return child, x
		case ast.ASTOpImplies:
			left, x := n.child(t, 0, !polarity, inQ, proofs)
			if x != nil {
				return nnfResult{}, x
			}
			right, x := n.child(t, 1, polarity, inQ, proofs)
			if x != nil {
				return nnfResult{}, x
			}
			args := []ast.ASTHandle{left.value, right.value}
			var value ast.ASTHandle
			if polarity {
				value, x = m.MkOrArray(args)
			} else {
				value, x = m.MkAndArray(args)
			}
			if x != nil {
				return nnfResult{}, x
			}
			var proof ast.ASTHandle
			if proofs {
				proof, x = n.mkProof(polarity, t, value, []ast.ASTHandle{left.proof, right.proof})
			}
			return nnfResult{value, proof}, x
		case ast.ASTOpIte:
			return n.convertITE(t, polarity, inQ, proofs)
		case ast.ASTOpIff, ast.ASTOpXor:
			return n.convertIffXor(t, polarity, inQ, proofs, op == ast.ASTOpIff)
		case ast.ASTOpEq:
			arg, x := m.GetArg(t, 0)
			if x != nil {
				return nnfResult{}, x
			}
			sort, x := m.GetSort(arg)
			if x != nil {
				return nnfResult{}, x
			}
			b, x := m.MkBoolSort()
			if x != nil {
				return nnfResult{}, x
			}
			if sort == b {
				return n.convertIffXor(t, polarity, inQ, proofs, true)
			}
		}
	}
	var positive bool
	var labels []usymbol.Symbol
	if labeled, x := m.IsLabel(t, &positive, &labels); x != nil {
		return nnfResult{}, x
	} else if labeled {
		return n.convertLabel(t, polarity, inQ, proofs, positive, labels)
	}
	return n.convertDefault(t, polarity, inQ, proofs)
}

func (n *NNF) convertITE(t ast.ASTHandle, polarity, inQ, proofs bool) (nnfResult, error) {
	a, e := n.child(t, 0, true, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	na, e := n.child(t, 0, false, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	b, e := n.child(t, 1, polarity, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	c, e := n.child(t, 2, polarity, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	x, e := n.manager.MkOr(na.value, b.value)
	if e != nil {
		return nnfResult{}, e
	}
	y, e := n.manager.MkOr(a.value, c.value)
	if e != nil {
		return nnfResult{}, e
	}
	value, e := n.manager.MkAnd(x, y)
	if e != nil {
		return nnfResult{}, e
	}
	var proof ast.ASTHandle
	if proofs {
		proof, e = n.mkProof(polarity, t, value, []ast.ASTHandle{a.proof, na.proof, b.proof, c.proof})
	}
	return nnfResult{value, proof}, e
}

func (n *NNF) convertIffXor(t ast.ASTHandle, polarity, inQ, proofs, equality bool) (nnfResult, error) {
	l, e := n.child(t, 0, true, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	nl, e := n.child(t, 0, false, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	r, e := n.child(t, 1, true, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	nr, e := n.child(t, 1, false, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	var a, b ast.ASTHandle
	if equality == polarity {
		a, e = n.manager.MkOr(nl.value, r.value)
		if e == nil {
			b, e = n.manager.MkOr(l.value, nr.value)
		}
	} else {
		a, e = n.manager.MkOr(l.value, r.value)
		if e == nil {
			b, e = n.manager.MkOr(nl.value, nr.value)
		}
	}
	if e != nil {
		return nnfResult{}, e
	}
	value, e := n.manager.MkAnd(a, b)
	if e != nil {
		return nnfResult{}, e
	}
	var proof ast.ASTHandle
	if proofs {
		proof, e = n.mkProof(polarity, t, value, []ast.ASTHandle{l.proof, nl.proof, r.proof, nr.proof})
	}
	return nnfResult{value, proof}, e
}

func (n *NNF) convertDefault(t ast.ASTHandle, polarity, inQ, proofs bool) (nnfResult, error) {
	hasQ, e := n.manager.HasQuantifiers(t)
	if e != nil {
		return nnfResult{}, fmt.Errorf("name candidate quantifiers: %w", e)
	}
	hasL, e := n.manager.HasLabels(t)
	if e != nil {
		return nnfResult{}, fmt.Errorf("name candidate labels: %w", e)
	}
	if n.options.Mode != NNFFull && !hasQ && !hasL {
		return n.skip(t, polarity, proofs)
	}
	out, e := n.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return nnfResult{}, e
	}
	n.scratch = append(n.scratch, out)
	pr, e := n.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return nnfResult{}, e
	}
	n.scratch = append(n.scratch, pr)
	namer := n.quant
	if n.options.Mode == NNFFull || (n.options.Mode != NNFSkolem && inQ) {
		namer = n.nested
	}
	if e = namer.Rewrite(t, &n.todoDefs, &n.todoProofs, out, pr); e != nil {
		return nnfResult{}, fmt.Errorf("name nested expression: %w", e)
	}
	value, e := n.manager.RefGet(out)
	if e != nil {
		return nnfResult{}, fmt.Errorf("named expression result: %w", e)
	}
	var proof ast.ASTHandle
	if proofs {
		proof, e = n.manager.RefGet(pr)
		if e != nil {
			return nnfResult{}, fmt.Errorf("named expression proof: %w", e)
		}
	}
	if !polarity {
		oldNot, x := n.manager.MkNot(t)
		if x != nil {
			return nnfResult{}, x
		}
		value, x = n.manager.MkNot(value)
		if x != nil {
			return nnfResult{}, x
		}
		if proofs {
			proof, x = n.manager.MkProofOEqCongruence(oldNot, value, []ast.ASTHandle{proof})
		}
		if x != nil {
			return nnfResult{}, x
		}
	}
	return nnfResult{value, proof}, nil
}

func (n *NNF) convertLabel(t ast.ASTHandle, polarity, inQ, proofs, positive bool, labels []usymbol.Symbol) (nnfResult, error) {
	arg, e := n.child(t, 0, polarity, inQ, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	if n.options.IgnoreLabels && !proofs {
		return arg, nil
	}
	body, e := n.manager.GetArg(t, 0)
	if e != nil {
		return nnfResult{}, e
	}
	if polarity != positive {
		if !proofs {
			return arg, nil
		}
		rw, e := n.manager.MkProofRewrite(t, body)
		if e != nil {
			return nnfResult{}, e
		}
		rw, e = n.manager.MkProofIffOEq(rw)
		if e != nil {
			return nnfResult{}, e
		}
		pr, e := n.manager.MkProofTransitivity(rw, arg.proof)
		return nnfResult{arg.value, pr}, e
	}
	lit, e := n.manager.MkLabelLit(labels)
	if e != nil {
		return nnfResult{}, e
	}
	value, e := n.manager.MkAnd(arg.value, lit)
	if e != nil {
		return nnfResult{}, e
	}
	if !proofs {
		return nnfResult{value: value}, nil
	}
	aux, e := n.manager.MkLabel(true, labels, arg.value)
	if e != nil {
		return nnfResult{}, e
	}
	p1, e := n.mkProof(polarity, t, aux, []ast.ASTHandle{arg.proof})
	if e != nil {
		return nnfResult{}, e
	}
	p2, e := n.manager.MkProofRewrite(aux, value)
	if e != nil {
		return nnfResult{}, e
	}
	p2, e = n.manager.MkProofIffOEq(p2)
	if e != nil {
		return nnfResult{}, e
	}
	proof, e := n.manager.MkProofTransitivity(p1, p2)
	return nnfResult{value, proof}, e
}

func (n *NNF) convertQuantifier(q ast.ASTHandle, polarity, inQ, proofs bool) (nnfResult, error) {
	forall, e := n.manager.IsForall(q)
	if e != nil {
		return nnfResult{}, e
	}
	if forall != polarity && n.options.Skolemize {
		valueRef, e := n.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return nnfResult{}, e
		}
		n.scratch = append(n.scratch, valueRef)
		proofRef, e := n.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return nnfResult{}, e
		}
		n.scratch = append(n.scratch, proofRef)
		if e = n.skolemizer.Apply(q, valueRef, proofRef); e != nil {
			return nnfResult{}, e
		}
		value, e := n.manager.RefGet(valueRef)
		if e != nil {
			return nnfResult{}, e
		}
		child, e := n.convert(value, !forall, inQ, proofs)
		if e != nil {
			return nnfResult{}, e
		}
		if proofs {
			p, x := n.manager.RefGet(proofRef)
			if x != nil {
				return nnfResult{}, x
			}
			child.proof, e = n.manager.MkProofTransitivity(p, child.proof)
		}
		return child, e
	}
	body, e := n.manager.GetQuantifierBody(q)
	if e != nil {
		return nnfResult{}, e
	}
	child, e := n.convert(body, polarity, true, proofs)
	if e != nil {
		return nnfResult{}, e
	}
	var patterns []ast.ASTHandle
	if forall == polarity {
		count, x := n.manager.GetNumPatterns(q)
		if x != nil {
			return nnfResult{}, x
		}
		for i := uint32(0); i < count; i++ {
			p, x := n.manager.GetPattern(q, i)
			if x != nil {
				return nnfResult{}, x
			}
			hack, x := n.skolemizer.IsSkolemHack(p)
			if x != nil {
				return nnfResult{}, x
			}
			if !hack {
				patterns = append(patterns, p)
			}
		}
	}
	var value ast.ASTHandle
	if polarity {
		value, e = n.manager.UpdateQuantifierPatterns(q, patterns, child.value)
	} else {
		value, e = n.manager.UpdateQuantifierKindPatterns(q, !forall, patterns, child.value)
	}
	if e != nil {
		return nnfResult{}, e
	}
	var proof ast.ASTHandle
	if proofs {
		if polarity {
			proof, e = n.manager.MkProofNNFPos(q, value, []ast.ASTHandle{child.proof})
		} else {
			proof, e = n.manager.MkProofNNFNeg(q, value, []ast.ASTHandle{child.proof})
		}
	}
	return nnfResult{value, proof}, e
}

func (n *NNF) clearScratch() (result error) {
	for len(n.scratch) != 0 {
		i := len(n.scratch) - 1
		if e := n.manager.DelRef(&n.scratch[i]); e != nil {
			return e
		}
		n.scratch = n.scratch[:i]
	}
	for len(n.todoProofs) != 0 {
		i := len(n.todoProofs) - 1
		if e := n.manager.DelRef(&n.todoProofs[i]); e != nil {
			return e
		}
		n.todoProofs = n.todoProofs[:i]
	}
	for len(n.todoDefs) != 0 {
		i := len(n.todoDefs) - 1
		if e := n.manager.DelRef(&n.todoDefs[i]); e != nil {
			return e
		}
		n.todoDefs = n.todoDefs[:i]
	}
	return nil
}

// Transform appends normalized introduced definitions and their proofs. The
// caller owns all appended references.
func (n *NNF) Transform(root ast.ASTHandle, definitions, definitionProofs *[]ast.ASTRef, out, proofOut ast.ASTRef) (result error) {
	if e := n.open(); e != nil {
		return e
	}
	if definitions == nil || definitionProofs == nil {
		return ast.ErrASTHandle
	}
	if e := n.clearScratch(); e != nil {
		return e
	}
	defer func() { result = errors.Join(result, n.clearScratch()) }()
	proofs, e := n.manager.ProofsEnabled()
	if e != nil {
		return e
	}
	r, e := n.convert(root, true, false, proofs)
	if e != nil {
		return e
	}
	if e = n.manager.SetRef(out, r.value); e != nil {
		return e
	}
	if proofs {
		p := r.proof
		if p.IsNull() {
			p, e = n.manager.MkProofReflexivity(root)
		}
		if e != nil {
			return e
		}
		if e = n.manager.SetRef(proofOut, p); e != nil {
			return e
		}
	}
	startDefs, startProofs := len(*definitions), len(*definitionProofs)
	for i := 0; i < len(n.todoDefs); i++ {
		definition, x := n.manager.RefGet(n.todoDefs[i])
		if x != nil {
			return fmt.Errorf("nnf definition %d input: %w", i, x)
		}
		dr, x := n.convert(definition, true, false, proofs)
		if x != nil {
			return fmt.Errorf("nnf definition %d: %w", i, x)
		}
		ref, x := n.manager.NewRef(dr.value)
		if x != nil {
			return x
		}
		*definitions = append(*definitions, ref)
		if proofs {
			intro, x := n.manager.RefGet(n.todoProofs[i])
			if x != nil {
				return fmt.Errorf("nnf definition %d introduction: %w", i, x)
			}
			p, x := n.manager.MkProofModusPonens(intro, dr.proof)
			if x != nil {
				return fmt.Errorf("nnf definition %d proof: %w", i, x)
			}
			ref, x = n.manager.NewRef(p)
			if x != nil {
				return x
			}
			*definitionProofs = append(*definitionProofs, ref)
		}
	}
	for i, j := startDefs, len(*definitions)-1; i < j; i, j = i+1, j-1 {
		(*definitions)[i], (*definitions)[j] = (*definitions)[j], (*definitions)[i]
	}
	for i, j := startProofs, len(*definitionProofs)-1; i < j; i, j = i+1, j-1 {
		(*definitionProofs)[i], (*definitionProofs)[j] = (*definitionProofs)[j], (*definitionProofs)[i]
	}
	return nil
}

func (n *NNF) Reset() error {
	if e := n.open(); e != nil {
		return e
	}
	return n.clearScratch()
}

func (n *NNF) ResetCache() error {
	if e := n.open(); e != nil {
		return e
	}
	for i := range n.caches {
		for key, entry := range n.caches[i] {
			if entry.proof != (ast.ASTRef{}) {
				if e := n.manager.DelRef(&entry.proof); e != nil {
					n.caches[i][key] = entry
					return e
				}
			}
			if entry.value != (ast.ASTRef{}) {
				if e := n.manager.DelRef(&entry.value); e != nil {
					n.caches[i][key] = entry
					return e
				}
			}
			if entry.keyRef != (ast.ASTRef{}) {
				if e := n.manager.DelRef(&entry.keyRef); e != nil {
					n.caches[i][key] = entry
					return e
				}
			}
			delete(n.caches[i], key)
		}
	}
	return nil
}

func (n *NNF) Close() error {
	if n == nil || n.self != n {
		return ast.ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.clearScratch(); e != nil {
		return e
	}
	if e := n.ResetCache(); e != nil {
		return e
	}
	if n.quant != nil {
		if e := n.quant.Close(); e != nil {
			return e
		}
	}
	if n.nested != nil {
		if e := n.nested.Close(); e != nil {
			return e
		}
	}
	if n.skolemizer != nil {
		if e := n.skolemizer.Close(); e != nil {
			return e
		}
	}
	n.closed = true
	return nil
}
