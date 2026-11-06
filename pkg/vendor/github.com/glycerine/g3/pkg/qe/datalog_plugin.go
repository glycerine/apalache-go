package qe

// Source: src/qe/qe_dl_plugin.cpp, plugin ownership, caching and branch count.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type datalogQECache struct {
	atoms datalogQEAtoms
	refs  []ast.ASTRef
}

// DatalogPlugin owns cached atom and formula references and borrows its context.
type DatalogPlugin struct {
	self        *DatalogPlugin
	context     *SolverContext
	manager     *ast.ASTManager
	family      ast.FamilyID
	entries     map[arithmeticPluginKey]*datalogQECache
	constraints []ArithmeticConstraint
	closed      bool
}

func NewDatalogPlugin(context *SolverContext) (*DatalogPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, e := context.Manager()
	if e != nil {
		return nil, e
	}
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return nil, e
	}
	p := &DatalogPlugin{context: context, manager: m, family: family, entries: make(map[arithmeticPluginKey]*datalogQECache)}
	p.self = p
	return p, nil
}
func (p *DatalogPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.context.open()
}
func (p *DatalogPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}
func (p *DatalogPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if e := p.open(); e != nil {
		return ast.ASTHandle{}, false, e
	}
	return ast.ASTHandle{}, false, nil
}
func (p *DatalogPlugin) IsUninterpreted(h ast.ASTHandle) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	k, e := p.manager.GetKind(h)
	if e != nil {
		return false, e
	}
	if k != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return true, nil
}
func (p *DatalogPlugin) key(variable *ast.ContainsApp, formula ast.ASTHandle) (arithmeticPluginKey, error) {
	if e := p.open(); e != nil {
		return arithmeticPluginKey{}, e
	}
	if variable == nil {
		return arithmeticPluginKey{}, ast.ErrASTHandle
	}
	h, e := variable.Target()
	if e != nil {
		return arithmeticPluginKey{}, e
	}
	k, e := p.manager.GetKind(h)
	if e != nil {
		return arithmeticPluginKey{}, e
	}
	if k != ast.ASTApp {
		return arithmeticPluginKey{}, ast.ErrASTDomain
	}
	if _, e = p.manager.GetKind(formula); e != nil {
		return arithmeticPluginKey{}, e
	}
	return arithmeticPluginKey{variable: h, formula: formula}, nil
}
func (p *DatalogPlugin) release(entry *datalogQECache) error {
	for len(entry.refs) > 0 {
		i := len(entry.refs) - 1
		if e := p.manager.DelRef(&entry.refs[i]); e != nil {
			return e
		}
		entry.refs = entry.refs[:i]
	}
	return nil
}

// Prepare scans source-ordered atoms and caches independent references.
func (p *DatalogPlugin) Prepare(variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle) (bool, error) {
	key, e := p.key(variable, formula)
	if e != nil {
		return false, e
	}
	if p.entries[key] != nil {
		return true, nil
	}
	atoms, ok, e := collectDatalogQEAtoms(p.manager, key.variable, positive, negative)
	if e != nil || !ok {
		return ok, e
	}
	entry := &datalogQECache{atoms: atoms}
	roots := []ast.ASTHandle{key.variable, key.formula}
	for _, a := range atoms.positive {
		roots = append(roots, a.atom, a.value)
	}
	for _, a := range atoms.negative {
		roots = append(roots, a.atom, a.value)
	}
	for _, h := range roots {
		ref, err := p.manager.NewRef(h)
		if err != nil {
			return false, errors.Join(err, p.release(entry))
		}
		entry.refs = append(entry.refs, ref)
	}
	p.entries[key] = entry
	return true, nil
}
func (p *DatalogPlugin) PrepareAtoms(variable *ast.ContainsApp, formula ast.ASTHandle, atoms *ArithmeticAtomSet) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	if atoms == nil || atoms.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	pos, e := atoms.Positive()
	if e != nil {
		return false, e
	}
	neg, e := atoms.Negative()
	if e != nil {
		return false, e
	}
	return p.Prepare(variable, formula, pos, neg)
}

// GetNumBranches requires preparation and reads the source sort cardinality.
func (p *DatalogPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, out *numeral.OwnedRational) (bool, error) {
	if out == nil {
		return false, ast.ErrASTHandle
	}
	key, e := p.key(variable, formula)
	if e != nil {
		return false, e
	}
	entry := p.entries[key]
	if entry == nil {
		return false, ast.ErrASTDomain
	}
	sort, e := p.manager.GetSort(key.variable)
	if e != nil {
		return false, e
	}
	size, e := p.manager.GetSortSize(sort)
	if e != nil {
		return false, e
	}
	if !size.IsFinite() {
		return false, ast.ErrASTDomain
	}
	domain, e := size.Size()
	if e != nil {
		return false, e
	}
	count, _ := datalogQEBranchCount(domain, entry.atoms)
	return true, p.manager.NumeralService().SetUint64(out, count)
}
func (p *DatalogPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for len(p.constraints) > 0 {
		i := len(p.constraints) - 1
		if e := p.manager.DelRef(&p.constraints[i].Formula); e != nil {
			return e
		}
		p.constraints = p.constraints[:i]
	}
	for key, entry := range p.entries {
		if e := p.release(entry); e != nil {
			return e
		}
		delete(p.entries, key)
	}
	p.closed = true
	return nil
}
