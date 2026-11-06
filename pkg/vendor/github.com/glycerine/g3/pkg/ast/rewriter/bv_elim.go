package rewriter

// Source: src/ast/rewriter/bv_elim.cpp:13-114;
// src/ast/rewriter/bv_elim.h:24-47.

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// BVElimConfiguration replaces bit-vector quantifier declarations with one
// Boolean declaration per bit and substitutes mkbv terms into the body and
// patterns.
type BVElimConfiguration struct {
	DefaultConfiguration
	self    *BVElimConfiguration
	manager *ast.ASTManager
	closed  bool
}

func NewBVElimConfiguration(m *ast.ASTManager) (*BVElimConfiguration, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	c := &BVElimConfiguration{manager: m}
	c.self = c
	c.defaults.manager = m
	return c, nil
}

func (c *BVElimConfiguration) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ast.ErrASTHandle
	}
	_, e := c.manager.ProofsEnabled()
	return e
}

func (c *BVElimConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (result bool, err error) {
	if e := c.open(); e != nil {
		return false, e
	}
	m := c.manager
	count, e := m.GetNumDeclarations(q)
	if e != nil {
		return false, e
	}
	boolSort, e := m.MkBoolSort()
	if e != nil {
		return false, e
	}
	var reverseNames []usymbol.Symbol
	var reverseSorts, reverseSubstitutions []ast.ASTHandle
	var variableIndex uint32
	found := false
	for i := count; i > 0; {
		i--
		sort, x := m.GetDeclarationSort(q, i)
		if x != nil {
			return false, x
		}
		name, x := m.GetDeclarationName(q, i)
		if x != nil {
			return false, x
		}
		isBV, x := m.IsBitVectorSort(sort)
		if x != nil {
			return false, x
		}
		if isBV {
			width, x := m.BitVectorSize(sort)
			if x != nil {
				return false, x
			}
			text, x := m.Symbols.Display(name)
			if x != nil {
				return false, x
			}
			bits := make([]ast.ASTHandle, width)
			found = true
			for j := uint32(0); j < width; j++ {
				bits[j], x = m.MkVar(variableIndex, boolSort)
				if x != nil {
					return false, x
				}
				variableIndex++
				reverseSorts = append(reverseSorts, boolSort)
				bitName, x := m.Symbols.Intern(text + "_" + strconv.FormatUint(uint64(j), 10))
				if x != nil {
					return false, x
				}
				reverseNames = append(reverseNames, bitName)
			}
			bv, x := m.MkBitVectorFromBools(bits)
			if x != nil {
				return false, x
			}
			reverseSubstitutions = append(reverseSubstitutions, bv)
		} else {
			v, x := m.MkVar(variableIndex, sort)
			if x != nil {
				return false, x
			}
			variableIndex++
			reverseSubstitutions = append(reverseSubstitutions, v)
			reverseSorts = append(reverseSorts, sort)
			reverseNames = append(reverseNames, name)
		}
	}
	if !found {
		return false, nil
	}
	reverseHandles := func(values []ast.ASTHandle) []ast.ASTHandle {
		out := make([]ast.ASTHandle, len(values))
		for i := range values {
			out[len(values)-i-1] = values[i]
		}
		return out
	}
	sorts := reverseHandles(reverseSorts)
	substitutions := reverseHandles(reverseSubstitutions)
	names := make([]usymbol.Symbol, len(reverseNames))
	for i := range reverseNames {
		names[len(reverseNames)-i-1] = reverseNames[i]
	}
	subst, e := NewVarSubstitution(m, true)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, subst.Close()) }()
	tmp, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	var retained []ast.ASTRef
	defer func() {
		for i := len(retained) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&retained[i]))
		}
	}()
	retain := func(h ast.ASTHandle) error {
		r, x := m.NewRef(h)
		if x == nil {
			retained = append(retained, r)
		}
		return x
	}
	if e = subst.Substitute(body, substitutions, tmp); e != nil {
		return false, e
	}
	newBody, e := m.RefGet(tmp)
	if e != nil {
		return false, e
	}
	if e = retain(newBody); e != nil {
		return false, e
	}
	rewriteArgs := func(input ast.AppArguments) ([]ast.ASTHandle, error) {
		values := make([]ast.ASTHandle, input.Count())
		for i := range values {
			h, x := input.At(uint32(i))
			if x != nil {
				return nil, x
			}
			if x = subst.Substitute(h, substitutions, tmp); x != nil {
				return nil, x
			}
			values[i], x = m.RefGet(tmp)
			if x != nil {
				return nil, x
			}
			if x = retain(values[i]); x != nil {
				return nil, x
			}
		}
		return values, nil
	}
	newPatterns, e := rewriteArgs(patterns)
	if e != nil {
		return false, e
	}
	newNoPatterns, e := rewriteArgs(noPatterns)
	if e != nil {
		return false, e
	}
	weight, e := m.GetQuantifierWeight(q)
	if e != nil {
		return false, e
	}
	qid, skid, e := m.GetQuantifierIDs(q)
	if e != nil {
		return false, e
	}
	value, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: newBody, Weight: weight, QID: qid, SkolemID: skid, Patterns: newPatterns, NoPatterns: newNoPatterns})
	if e != nil {
		return false, e
	}
	if e = m.SetRef(out, value); e != nil {
		return false, e
	}
	p, e := m.MkProofRewrite(q, value)
	if e != nil {
		return false, e
	}
	if e = m.SetRef(proof, p); e != nil {
		return false, e
	}
	return true, nil
}

func (c *BVElimConfiguration) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// BVElim owns the source bv_elim_rw traversal wrapper.
type BVElim struct {
	self      *BVElim
	manager   *ast.ASTManager
	config    *BVElimConfiguration
	traversal *Traversal
	header    uint64
	closed    bool
}

func NewBVElim(m *ast.ASTManager) (*BVElim, error) {
	c, e := NewBVElimConfiguration(m)
	if e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BVElim{}).Size() + reflect.TypeOf(BVElimConfiguration{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	r := &BVElim{manager: m, config: c, header: bytes}
	r.self = r
	r.traversal, e = NewTraversal(m, c)
	return r, e
}

func (r *BVElim) open() error {
	if r == nil || r.self != r || r.closed || r.config == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return r.config.open()
}

func (r *BVElim) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.traversal.Rewrite(root, out); e != nil {
		return e
	}
	proofs, e := r.manager.ProofsEnabled()
	if e != nil || !proofs {
		return e
	}
	value, e := r.manager.RefGet(out)
	if e != nil {
		return e
	}
	var p ast.ASTHandle
	if value == root {
		p, e = r.manager.MkProofReflexivity(root)
	} else {
		p, e = r.manager.MkProofRewrite(root, value)
	}
	if e != nil {
		return e
	}
	return r.manager.SetRef(proof, p)
}

func (r *BVElim) Reset() error {
	if e := r.open(); e != nil {
		return e
	}
	return r.traversal.Reset()
}

func (r *BVElim) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if e := r.traversal.Close(); e != nil {
			return e
		}
	}
	if r.config != nil {
		if e := r.config.Close(); e != nil {
			return e
		}
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
