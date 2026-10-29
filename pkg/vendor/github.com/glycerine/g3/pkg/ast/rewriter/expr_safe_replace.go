package rewriter

// Source: src/ast/rewriter/expr_safe_replace.cpp;
// src/ast/rewriter/expr_safe_replace.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type safeReplacement struct {
	source, destination ast.ASTHandle
}

// ExprSafeReplace performs simultaneous expression replacement and shifts
// substitutions through quantifier binders.
type ExprSafeReplace struct {
	self         *ExprSafeReplace
	manager      *ast.ASTManager
	sourceRefs   []ast.ASTRef
	destRefs     []ast.ASTRef
	replacements []safeReplacement
	substitution map[ast.ASTHandle]ast.ASTHandle
	temporary    []ast.ASTRef
	closed       bool
}

func NewExprSafeReplace(m *ast.ASTManager) (*ExprSafeReplace, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	r := &ExprSafeReplace{manager: m, substitution: make(map[ast.ASTHandle]ast.ASTHandle)}
	r.self = r
	return r, nil
}

func (r *ExprSafeReplace) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *ExprSafeReplace) Insert(source, destination ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	sourceSort, err := r.manager.GetSort(source)
	if err != nil {
		return err
	}
	destinationSort, err := r.manager.GetSort(destination)
	if err != nil {
		return err
	}
	if sourceSort != destinationSort {
		return ast.ErrASTDomain
	}
	sourceRef, err := r.manager.NewRef(source)
	if err != nil {
		return err
	}
	r.sourceRefs = append(r.sourceRefs, sourceRef)
	destinationRef, err := r.manager.NewRef(destination)
	if err != nil {
		return err
	}
	r.destRefs = append(r.destRefs, destinationRef)
	r.replacements = append(r.replacements, safeReplacement{source: source, destination: destination})
	r.substitution[source] = destination
	return nil
}

func (r *ExprSafeReplace) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return len(r.substitution) == 0, nil
}

func (r *ExprSafeReplace) clearTemporary() error {
	for len(r.temporary) != 0 {
		last := len(r.temporary) - 1
		if err := r.manager.DelRef(&r.temporary[last]); err != nil {
			return err
		}
		r.temporary = r.temporary[:last]
	}
	return nil
}

func (r *ExprSafeReplace) scratch() (ast.ASTRef, error) {
	ref, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	r.temporary = append(r.temporary, ref)
	return ref, nil
}

func (r *ExprSafeReplace) retainTemporary(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err != nil {
		return err
	}
	r.temporary = append(r.temporary, ref)
	return nil
}

func (r *ExprSafeReplace) rewriteQuantifier(q ast.ASTHandle) (result ast.ASTHandle, err error) {
	nested, err := NewExprSafeReplace(r.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, nested.Close()) }()
	shifter, err := newVarShifter(r.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, shifter.Close()) }()
	declarations, err := r.manager.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, replacement := range r.replacements {
		sourceOut, e := r.scratch()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = shifter.ShiftBy(replacement.source, declarations, sourceOut); e != nil {
			return ast.ASTHandle{}, e
		}
		destinationOut, e := r.scratch()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = shifter.ShiftBy(replacement.destination, declarations, destinationOut); e != nil {
			return ast.ASTHandle{}, e
		}
		shiftedSource, e := r.manager.RefGet(sourceOut)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		shiftedDestination, e := r.manager.RefGet(destinationOut)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = nested.Insert(shiftedSource, shiftedDestination); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	rewriteChild := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		out, e := r.scratch()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = nested.Apply(h, out); e != nil {
			return ast.ASTHandle{}, e
		}
		return r.manager.RefGet(out)
	}
	patternCount, err := r.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, 0, patternCount)
	for i := uint32(0); i < patternCount; i++ {
		pattern, e := r.manager.GetPattern(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		pattern, e = rewriteChild(pattern)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		patterns = append(patterns, pattern)
	}
	noPatternCount, err := r.manager.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, 0, noPatternCount)
	for i := uint32(0); i < noPatternCount; i++ {
		noPattern, e := r.manager.GetNoPattern(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		noPattern, e = rewriteChild(noPattern)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		noPatterns = append(noPatterns, noPattern)
	}
	body, err := r.manager.GetQuantifierBody(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err = rewriteChild(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.manager.UpdateQuantifierAnnotations(q, patterns, noPatterns, body)
}

func (r *ExprSafeReplace) Apply(root ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.clearTemporary(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.clearTemporary()) }()
	if _, err := r.manager.GetKind(root); err != nil {
		return err
	}
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := []ast.ASTHandle{root}
	args := make([]ast.ASTHandle, 0)
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		if _, found := cache[current]; found {
			todo = todo[:last]
			continue
		}
		if replacement, found := r.substitution[current]; found {
			cache[current] = replacement
			todo = todo[:last]
			continue
		}
		kind, err := r.manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			cache[current] = current
			todo = todo[:last]
		case ast.ASTApp:
			n, e := r.manager.GetNumArgs(current)
			if e != nil {
				return e
			}
			args = args[:0]
			changed := false
			for i := uint32(0); i < n; i++ {
				child, childErr := r.manager.GetArg(current, i)
				if childErr != nil {
					return childErr
				}
				rewritten, found := cache[child]
				if !found {
					todo = append(todo, child)
					continue
				}
				if ast.ASTSourceDebug {
					left, sortErr := r.manager.GetSort(child)
					if sortErr != nil {
						return sortErr
					}
					right, sortErr := r.manager.GetSort(rewritten)
					if sortErr != nil {
						return sortErr
					}
					if left != right {
						return ast.ErrASTDomain
					}
				}
				args = append(args, rewritten)
				changed = changed || child != rewritten
			}
			if uint32(len(args)) != n {
				continue
			}
			rewritten := current
			if changed {
				decl, e := r.manager.GetDecl(current)
				if e != nil {
					return e
				}
				rewritten, e = r.manager.MkApp(decl, args)
				if e != nil {
					return e
				}
				if e = r.retainTemporary(rewritten); e != nil {
					return e
				}
			}
			cache[current] = rewritten
			todo = todo[:last]
		case ast.ASTQuantifier:
			rewritten, e := r.rewriteQuantifier(current)
			if e != nil {
				return e
			}
			if e = r.retainTemporary(rewritten); e != nil {
				return e
			}
			cache[current] = rewritten
			todo = todo[:last]
		default:
			return ast.ErrASTDomain
		}
	}
	rewritten, found := cache[root]
	if !found {
		return ast.ErrASTDomain
	}
	return r.manager.SetRef(out, rewritten)
}

func (r *ExprSafeReplace) ApplyRef(value ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	h, err := r.manager.RefGet(value)
	if err != nil {
		return err
	}
	return r.Apply(h, value)
}

func (r *ExprSafeReplace) ApplyRefSlice(values []ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	for _, value := range values {
		if err := r.ApplyRef(value); err != nil {
			return err
		}
	}
	return nil
}

func (r *ExprSafeReplace) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.clearTemporary(); err != nil {
		return err
	}
	for len(r.sourceRefs) != 0 {
		last := len(r.sourceRefs) - 1
		if err := r.manager.DelRef(&r.sourceRefs[last]); err != nil {
			return err
		}
		r.sourceRefs = r.sourceRefs[:last]
	}
	for len(r.destRefs) != 0 {
		last := len(r.destRefs) - 1
		if err := r.manager.DelRef(&r.destRefs[last]); err != nil {
			return err
		}
		r.destRefs = r.destRefs[:last]
	}
	r.replacements = nil
	r.substitution = make(map[ast.ASTHandle]ast.ASTHandle)
	return nil
}

func (r *ExprSafeReplace) ApplySubstitution(source, definition ast.ASTHandle, target ast.ASTRef) (result error) {
	if err := r.Reset(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.Reset()) }()
	if err := r.Insert(source, definition); err != nil {
		return err
	}
	return r.ApplyRef(target)
}

func (r *ExprSafeReplace) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if err := r.Reset(); err != nil {
		return err
	}
	r.closed = true
	return nil
}
