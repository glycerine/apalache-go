package core

// Source: src/tactic/core/reduce_args_tactic.cpp candidate discovery and cfg.

import (
	"encoding/binary"
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (f *reduceArgsFrame) mayBeUnique(term ast.ASTHandle) (bool, ast.ASTHandle, error) {
	m := f.owner.manager
	unique, err := m.IsUniqueValue(term)
	if err != nil || unique {
		return unique, ast.ASTHandle{}, err
	}
	base := term
	kind, err := m.GetKind(term)
	if err != nil {
		return false, base, err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(term)
		if err != nil {
			return false, base, err
		}
		if family == f.owner.family {
			d, err := m.GetDecl(term)
			if err != nil {
				return false, base, err
			}
			op, err := m.GetDeclKind(d)
			if err != nil {
				return false, base, err
			}
			n, err := m.GetNumArgs(term)
			if err != nil {
				return false, base, err
			}
			if op == 4 && n == 2 {
				lhs, err := m.GetArg(term, 0)
				if err != nil {
					return false, base, err
				}
				_, numeral, err := m.BitVectorNumeralText(lhs)
				if err != nil {
					return false, base, err
				}
				if numeral {
					base, err = m.GetArg(term, 1)
					if err != nil {
						return false, base, err
					}
				}
			}
		}
	}
	free, err := m.HasFreeVars(base)
	return !free, base, err
}
func (f *reduceArgsFrame) children(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := f.owner.manager
	kind, err := m.GetKind(term)
	if err != nil {
		return nil, err
	}
	var children []ast.ASTHandle
	switch kind {
	case ast.ASTVar:
	case ast.ASTApp:
		n, err := m.GetNumArgs(term)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < n; i++ {
			h, err := m.GetArg(term, i)
			if err != nil {
				return nil, err
			}
			children = append(children, h)
		}
	case ast.ASTQuantifier:
		h, err := m.GetQuantifierBody(term)
		if err != nil {
			return nil, err
		}
		children = append(children, h)
		n, err := m.GetNumPatterns(term)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < n; i++ {
			h, err := m.GetPattern(term, i)
			if err != nil {
				return nil, err
			}
			children = append(children, h)
		}
		n, err = m.GetNumNoPatterns(term)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < n; i++ {
			h, err := m.GetNoPattern(term, i)
			if err != nil {
				return nil, err
			}
			children = append(children, h)
		}
	default:
		return nil, ast.ErrASTDomain
	}
	return children, nil
}
func (f *reduceArgsFrame) visit(term ast.ASTHandle, first bool, non map[ast.ASTHandle]bool) error {
	m := f.owner.manager
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return err
	}
	n, err := m.GetNumArgs(term)
	if err != nil || n == 0 {
		return err
	}
	d, err := m.GetDecl(term)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(d)
	if err != nil || family != ast.NullFamilyID || non[d] {
		return err
	}
	if first {
		for i := n; i > 0; i-- {
			arg, err := m.GetArg(term, i-1)
			if err != nil {
				return err
			}
			possible, _, err := f.mayBeUnique(arg)
			if err != nil {
				return err
			}
			if possible {
				return nil
			}
		}
		non[d] = true
		return nil
	}
	c := f.candidates[d]
	if c == nil {
		c = &reduceArgsCandidate{mask: make([]bool, n), bases: make([]ast.ASTHandle, n)}
		f.candidates[d] = c
		for i := n; i > 0; i-- {
			arg, err := m.GetArg(term, i-1)
			if err != nil {
				return err
			}
			c.mask[i-1], c.bases[i-1], err = f.mayBeUnique(arg)
			if err != nil {
				return err
			}
		}
	} else {
		for i := n; i > 0; i-- {
			if !c.mask[i-1] {
				continue
			}
			arg, err := m.GetArg(term, i-1)
			if err != nil {
				return err
			}
			possible, base, err := f.mayBeUnique(arg)
			if err != nil {
				return err
			}
			c.mask[i-1] = possible && c.bases[i-1] == base
		}
	}
	return nil
}
func (f *reduceArgsFrame) walk(g *tactic.Goal, first bool, non map[ast.ASTHandle]bool) error {
	type frame struct {
		term     ast.ASTHandle
		children []ast.ASTHandle
		next     int
	}
	seen := make(map[ast.ASTHandle]bool)
	sz, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < sz; i++ {
		if err = f.owner.checkpoint(); err != nil {
			return err
		}
		root, err := g.Form(i)
		if err != nil {
			return err
		}
		if seen[root] {
			continue
		}
		children, err := f.children(root)
		if err != nil {
			return err
		}
		seen[root] = true
		stack := []frame{{term: root, children: children}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next < len(top.children) {
				child := top.children[top.next]
				top.next++
				if seen[child] {
					continue
				}
				children, err := f.children(child)
				if err != nil {
					return err
				}
				seen[child] = true
				stack = append(stack, frame{term: child, children: children})
				continue
			}
			term := top.term
			stack = stack[:len(stack)-1]
			if err = f.visit(term, first, non); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *reduceArgsFrame) analyze(g *tactic.Goal) error {
	non := make(map[ast.ASTHandle]bool)
	if err := f.walk(g, true, non); err != nil {
		return err
	}
	if err := f.owner.emitCandidates(non, nil, 169); err != nil {
		return err
	}
	if err := f.walk(g, false, non); err != nil {
		return err
	}
	for d, c := range f.candidates {
		any := false
		for _, v := range c.mask {
			any = any || v
		}
		if !any {
			delete(f.candidates, d)
		}
	}
	return f.owner.emitCandidates(nil, f.candidates, 255)
}

type reduceArgsConfiguration struct {
	rewriter.DefaultConfiguration
	frame *reduceArgsFrame
}

func (c *reduceArgsConfiguration) MaxStepsExceeded(uint32) (bool, error) {
	return false, c.frame.owner.checkpoint()
}
func (c *reduceArgsConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	f := c.frame
	m := f.owner.manager
	if err = m.ResetRef(proof); err != nil {
		return rewriter.RewriteFailed, err
	}
	candidate := f.candidates[d]
	if candidate == nil || args.Count() == 0 {
		return rewriter.RewriteFailed, nil
	}
	var all, remaining []ast.ASTHandle
	var key []byte
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		all = append(all, arg)
		if candidate.mask[i] {
			id, e := m.GetASTID(arg)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			key = binary.LittleEndian.AppendUint32(key, id)
		} else {
			remaining = append(remaining, arg)
		}
	}
	origin, err := m.MkApp(d, all)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	scratch := len(f.scratch)
	if err = f.hold(origin, true); err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { err = errors.Join(err, f.release(&f.scratch, scratch)) }()
	group := f.groups[d]
	if group == nil {
		group = make(map[string]*reduceArgsEntry)
		f.groups[d] = group
	}
	entry := group[string(key)]
	if entry == nil {
		entry = &reduceArgsEntry{application: origin}
		group[string(key)] = entry
		var domain []ast.ASTHandle
		for i, v := range candidate.mask {
			if !v {
				sort, e := m.GetDomain(d, uint32(i))
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				domain = append(domain, sort)
			}
		}
		name, e := m.GetASTName(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		rangeSort, e := m.GetRange(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		entry.declaration, e = m.MkFreshFuncDecl(name, usymbol.Symbol{}, domain, rangeSort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if err = f.hold(origin, false); err != nil {
			return rewriter.RewriteFailed, err
		}
		if err = f.hold(entry.declaration, false); err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	result, err := m.MkApp(entry.declaration, remaining)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, m.SetRef(out, result)
}
