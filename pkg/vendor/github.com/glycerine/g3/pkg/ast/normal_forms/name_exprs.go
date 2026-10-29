package normal_forms

// Source: src/ast/normal_forms/name_exprs.cpp:24-151;
// src/ast/normal_forms/name_exprs.h:24-62.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ExprPredicate selects expressions that should be replaced by defined names.
type ExprPredicate interface {
	NameExpression(ast.ASTHandle) (bool, error)
}

type ExprPredicateFunc func(ast.ASTHandle) (bool, error)

func (f ExprPredicateFunc) NameExpression(h ast.ASTHandle) (bool, error) { return f(h) }

type namedExprResult struct {
	value, proof ast.ASTRef
}

type nameExprFrame struct {
	node        ast.ASTHandle
	kind        ast.ASTKind
	next, count uint32
	initialized bool
}

// ExprNamer replaces selected subexpressions with names and appends each new
// definition and optional def-intro proof to caller-owned reference slices.
type ExprNamer struct {
	self      *ExprNamer
	manager   *ast.ASTManager
	names     *DefinedNames
	predicate ExprPredicate
	cache     map[ast.ASTHandle]namedExprResult
	order     []ast.ASTHandle
	root      ast.ASTHandle
	closed    bool
}

func NewExprNamer(m *ast.ASTManager, names *DefinedNames, predicate ExprPredicate) (*ExprNamer, error) {
	if m == nil || names == nil || predicate == nil || names.manager != m || names.valid() != nil {
		return nil, ast.ErrASTHandle
	}
	n := &ExprNamer{manager: m, names: names, predicate: predicate, cache: make(map[ast.ASTHandle]namedExprResult)}
	n.self = n
	return n, nil
}

func NewQuantifierLabelNamer(m *ast.ASTManager, names *DefinedNames) (*ExprNamer, error) {
	return NewExprNamer(m, names, ExprPredicateFunc(func(h ast.ASTHandle) (bool, error) {
		kind, e := m.GetKind(h)
		if e != nil || kind == ast.ASTQuantifier {
			return kind == ast.ASTQuantifier, e
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		var positive bool
		var labels []usymbol.Symbol
		return m.IsLabel(h, &positive, &labels)
	}))
}

func NewNestedFormulaNamer(m *ast.ASTManager, names *DefinedNames) (*ExprNamer, error) {
	if m == nil || names == nil || names.manager != m || names.valid() != nil {
		return nil, ast.ErrASTHandle
	}
	n := &ExprNamer{manager: m, names: names, cache: make(map[ast.ASTHandle]namedExprResult)}
	n.self = n
	n.predicate = ExprPredicateFunc(func(h ast.ASTHandle) (bool, error) {
		kind, e := m.GetKind(h)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTApp {
			family, e := m.GetFamilyID(h)
			if e != nil {
				return false, e
			}
			count, e := m.GetNumArgs(h)
			return family == 0 && count > 0 && h != n.root, e
		}
		if kind == ast.ASTQuantifier {
			return true, nil
		}
		return false, nil
	})
	return n, nil
}

func (n *ExprNamer) open() error {
	if n == nil || n.self != n || n.closed || n.manager == nil || n.names == nil || n.names.valid() != nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (n *ExprNamer) retain(key, value, proof ast.ASTHandle) (result error) {
	r := namedExprResult{}
	var e error
	if r.value, e = n.manager.NewRef(value); e != nil {
		return e
	}
	if !proof.IsNull() {
		if r.proof, e = n.manager.NewRef(proof); e != nil {
			return errors.Join(e, n.manager.DelRef(&r.value))
		}
	}
	n.cache[key] = r
	n.order = append(n.order, key)
	return nil
}

func (n *ExprNamer) cached(key ast.ASTHandle, isRoot bool) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	if isRoot {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	r, found := n.cache[key]
	if !found {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	v, e := n.manager.RefGet(r.value)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	var p ast.ASTHandle
	if r.proof != (ast.ASTRef{}) {
		p, e = n.manager.RefGet(r.proof)
	}
	return v, p, true, e
}

func (n *ExprNamer) makeName(h ast.ASTHandle, definitions, definitionProofs *[]ast.ASTRef) (value, proof ast.ASTHandle, result error) {
	m := n.manager
	refs := [4]ast.ASTRef{}
	defer func() {
		for i := range refs {
			if refs[i] != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&refs[i]))
			}
		}
	}()
	for i := range refs {
		var e error
		refs[i], e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
	}
	created, e := n.names.MkName(h, refs[0], refs[1], refs[2], refs[3])
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	value, e = m.RefGet(refs[2])
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	if proofs {
		proof, e = m.RefGet(refs[3])
	}
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	if created {
		*definitions = append(*definitions, refs[0])
		refs[0] = ast.ASTRef{}
		if proofs {
			*definitionProofs = append(*definitionProofs, refs[1])
			refs[1] = ast.ASTRef{}
		}
	}
	return value, proof, nil
}

// Rewrite appends outputs; the caller owns every appended ASTRef and must
// release it through the manager.
func (n *ExprNamer) Rewrite(root ast.ASTHandle, definitions, definitionProofs *[]ast.ASTRef, out, proofOut ast.ASTRef) (result error) {
	if e := n.open(); e != nil || definitions == nil || definitionProofs == nil {
		if e != nil {
			return e
		}
		return ast.ErrASTHandle
	}
	n.root = root
	proofs, e := n.manager.ProofsEnabled()
	if e != nil {
		return e
	}
	frames := []nameExprFrame{{node: root}}
	local := make(map[ast.ASTHandle]namedExprResult)
	var temporary []ast.ASTRef
	defer func() {
		for i := len(temporary) - 1; i >= 0; i-- {
			result = errors.Join(result, n.manager.DelRef(&temporary[i]))
		}
		n.root = ast.ASTHandle{}
	}()
	holdLocal := func(value, proof ast.ASTHandle) (namedExprResult, error) {
		r := namedExprResult{}
		var x error
		if r.value, x = n.manager.NewRef(value); x != nil {
			return r, x
		}
		temporary = append(temporary, r.value)
		if !proof.IsNull() {
			if r.proof, x = n.manager.NewRef(proof); x != nil {
				return r, x
			}
			temporary = append(temporary, r.proof)
		}
		return r, nil
	}
	for len(frames) != 0 {
		f := &frames[len(frames)-1]
		isRoot := len(frames) == 1
		if !f.initialized {
			if _, _, found, x := n.cached(f.node, isRoot); x != nil {
				return x
			} else if found {
				local[f.node] = n.cache[f.node]
				frames = frames[:len(frames)-1]
				continue
			}
			selected, x := n.predicate.NameExpression(f.node)
			if x != nil {
				return x
			}
			if selected {
				v, p, x := n.makeName(f.node, definitions, definitionProofs)
				if x != nil {
					return x
				}
				if isRoot {
					local[f.node], x = holdLocal(v, p)
				} else {
					x = n.retain(f.node, v, p)
					local[f.node] = n.cache[f.node]
				}
				if x != nil {
					return x
				}
				frames = frames[:len(frames)-1]
				continue
			}
			f.kind, e = n.manager.GetKind(f.node)
			if e != nil {
				return e
			}
			f.initialized = true
			switch f.kind {
			case ast.ASTApp:
				f.count, e = n.manager.GetNumArgs(f.node)
			case ast.ASTQuantifier:
				f.count = 1
			case ast.ASTVar:
				f.count = 0
			default:
				return ast.ErrASTDomain
			}
			if e != nil {
				return e
			}
		}
		if f.next < f.count {
			var child ast.ASTHandle
			if f.kind == ast.ASTQuantifier {
				child, e = n.manager.GetQuantifierBody(f.node)
			} else {
				child, e = n.manager.GetArg(f.node, f.next)
			}
			if e != nil {
				return e
			}
			if _, found := local[child]; !found {
				frames = append(frames, nameExprFrame{node: child})
				continue
			}
			f.next++
			continue
		}
		value, pr, x := n.rebuild(f.node, f.kind, f.count, local, proofs)
		if x != nil {
			return x
		}
		if isRoot {
			local[f.node], x = holdLocal(value, pr)
		} else {
			x = n.retain(f.node, value, pr)
			local[f.node] = n.cache[f.node]
		}
		if x != nil {
			return x
		}
		frames = frames[:len(frames)-1]
	}
	r := local[root]
	value, e := n.manager.RefGet(r.value)
	if e != nil {
		return e
	}
	if e = n.manager.SetRef(out, value); e != nil {
		return e
	}
	if proofs {
		var p ast.ASTHandle
		if r.proof != (ast.ASTRef{}) {
			p, e = n.manager.RefGet(r.proof)
		} else {
			p, e = n.manager.MkProofReflexivity(root)
		}
		if e != nil {
			return e
		}
		return n.manager.SetRef(proofOut, p)
	}
	return nil
}

func (n *ExprNamer) rebuild(node ast.ASTHandle, kind ast.ASTKind, count uint32, local map[ast.ASTHandle]namedExprResult, proofs bool) (ast.ASTHandle, ast.ASTHandle, error) {
	m := n.manager
	if kind == ast.ASTVar || count == 0 {
		return node, ast.ASTHandle{}, nil
	}
	if kind == ast.ASTQuantifier {
		body, e := m.GetQuantifierBody(node)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		r := local[body]
		value, e := m.RefGet(r.value)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if value == body {
			return node, ast.ASTHandle{}, nil
		}
		updated, e := m.UpdateQuantifierBody(node, value)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if !proofs {
			return updated, ast.ASTHandle{}, nil
		}
		p, e := m.RefGet(r.proof)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		pr, e := m.MkProofQuantIntro(node, updated, p)
		return updated, pr, e
	}
	args := make([]ast.ASTHandle, count)
	ps := make([]ast.ASTHandle, 0, count)
	changed := false
	for i := uint32(0); i < count; i++ {
		child, e := m.GetArg(node, i)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		r := local[child]
		args[i], e = m.RefGet(r.value)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		changed = changed || args[i] != child
		if proofs && r.proof != (ast.ASTRef{}) {
			p, x := m.RefGet(r.proof)
			if x != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, x
			}
			ps = append(ps, p)
		}
	}
	if !changed {
		return node, ast.ASTHandle{}, nil
	}
	d, e := m.GetDecl(node)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	updated, e := m.MkApp(d, args)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	if !proofs {
		return updated, ast.ASTHandle{}, nil
	}
	pr, e := m.MkProofCongruence(node, updated, ps)
	return updated, pr, e
}

func (n *ExprNamer) Reset() error {
	if e := n.open(); e != nil {
		return e
	}
	for len(n.order) != 0 {
		i := len(n.order) - 1
		key := n.order[i]
		r := n.cache[key]
		if r.proof != (ast.ASTRef{}) {
			if e := n.manager.DelRef(&r.proof); e != nil {
				n.cache[key] = r
				return e
			}
		}
		if r.value != (ast.ASTRef{}) {
			if e := n.manager.DelRef(&r.value); e != nil {
				n.cache[key] = r
				return e
			}
		}
		delete(n.cache, key)
		n.order = n.order[:i]
	}
	return nil
}

func (n *ExprNamer) Close() error {
	if n == nil || n.self != n {
		return ast.ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.Reset(); e != nil {
		return e
	}
	n.closed = true
	return nil
}
