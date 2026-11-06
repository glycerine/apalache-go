package smtcore

// Source: src/smt/cached_var_subst.h and src/smt/cached_var_subst.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type smtCachedSubstitutionEntry struct {
	quantifier ast.ASTHandle
	bindings   []ast.ASTHandle
	owned      []ast.ASTRef
	result     ast.ASTRef
}

type SMTCachedVarSubstitution struct {
	self         *SMTCachedVarSubstitution
	manager      *ast.ASTManager
	substitution *rewriter.VarSubstitution
	entries      map[ast.ASTHandle]map[int][]*smtCachedSubstitutionEntry
	ordered      []*smtCachedSubstitutionEntry
	closed       bool
}

func NewSMTCachedVarSubstitution(m *ast.ASTManager) (*SMTCachedVarSubstitution, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	c := &SMTCachedVarSubstitution{manager: m, entries: make(map[ast.ASTHandle]map[int][]*smtCachedSubstitutionEntry)}
	c.self = c
	var err error
	c.substitution, err = rewriter.NewVarSubstitution(m, true)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *SMTCachedVarSubstitution) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil || c.substitution == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *SMTCachedVarSubstitution) find(quantifier ast.ASTHandle, bindings []ast.ASTHandle) *smtCachedSubstitutionEntry {
	byArity := c.entries[quantifier]
	if byArity == nil {
		return nil
	}
	for _, entry := range byArity[len(bindings)] {
		if len(entry.bindings) != len(bindings) {
			continue
		}
		equal := true
		for i := range bindings {
			if entry.bindings[i] != bindings[i] {
				equal = false
				break
			}
		}
		if equal {
			return entry
		}
	}
	return nil
}

func (c *SMTCachedVarSubstitution) Instantiate(quantifier ast.ASTHandle, bindings []*SMTEnode, out ast.ASTRef) (cached bool, result error) {
	if err := c.open(); err != nil {
		return false, err
	}
	kind, err := c.manager.GetKind(quantifier)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTQuantifier {
		return false, ast.ErrASTDomain
	}
	owners := make([]ast.ASTHandle, len(bindings))
	for i, binding := range bindings {
		if binding == nil || binding.owner.IsNull() {
			return false, ast.ErrASTDomain
		}
		owners[i] = binding.owner
	}
	if entry := c.find(quantifier, owners); entry != nil {
		value, err := c.manager.RefGet(entry.result)
		if err != nil {
			return false, err
		}
		return true, c.manager.SetRef(out, value)
	}
	body, err := c.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return false, err
	}
	if err = c.substitution.Substitute(body, owners, out); err != nil {
		return false, err
	}
	value, err := c.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	entry := &smtCachedSubstitutionEntry{quantifier: quantifier, bindings: append([]ast.ASTHandle(nil), owners...)}
	handles := make([]ast.ASTHandle, 0, len(owners)+2)
	handles = append(handles, quantifier)
	handles = append(handles, owners...)
	handles = append(handles, value)
	for _, handle := range handles {
		ref, err := c.manager.NewRef(handle)
		if err != nil {
			for i := len(entry.owned) - 1; i >= 0; i-- {
				err = errors.Join(err, c.manager.DelRef(&entry.owned[i]))
			}
			return false, err
		}
		entry.owned = append(entry.owned, ref)
	}
	entry.result = entry.owned[len(entry.owned)-1]
	byArity := c.entries[quantifier]
	if byArity == nil {
		byArity = make(map[int][]*smtCachedSubstitutionEntry)
		c.entries[quantifier] = byArity
	}
	byArity[len(owners)] = append(byArity[len(owners)], entry)
	c.ordered = append(c.ordered, entry)
	return false, nil
}

func (c *SMTCachedVarSubstitution) Size() uint32 {
	if c == nil {
		return 0
	}
	return uint32(len(c.ordered))
}

func (c *SMTCachedVarSubstitution) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.clearEntries(); err != nil {
		return err
	}
	return c.substitution.Reset()
}

func (c *SMTCachedVarSubstitution) clearEntries() error {
	for len(c.ordered) != 0 {
		last := len(c.ordered) - 1
		entry := c.ordered[last]
		for len(entry.owned) != 0 {
			i := len(entry.owned) - 1
			if err := c.manager.DelRef(&entry.owned[i]); err != nil {
				return err
			}
			entry.owned = entry.owned[:i]
		}
		c.ordered = c.ordered[:last]
	}
	c.entries = make(map[ast.ASTHandle]map[int][]*smtCachedSubstitutionEntry)
	return nil
}

func (c *SMTCachedVarSubstitution) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.clearEntries(); err != nil {
		return err
	}
	if c.substitution != nil {
		if err := c.substitution.Close(); err != nil {
			return err
		}
		c.substitution = nil
	}
	c.closed = true
	return nil
}
