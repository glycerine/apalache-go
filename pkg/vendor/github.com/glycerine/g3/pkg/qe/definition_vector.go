package qe

// Source: src/qe/qe.h:228-278; src/qe/qe.cpp:728-804.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// DefinitionVector owns parallel function-declaration and definition refs.
type DefinitionVector struct {
	self        *DefinitionVector
	manager     *ast.ASTManager
	config      Config
	variables   []ast.ASTRef
	definitions []ast.ASTRef
	closed      bool
}

func NewDefinitionVector(manager *ast.ASTManager, config Config) (*DefinitionVector, error) {
	if manager == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	d := &DefinitionVector{manager: manager, config: config}
	d.self = d
	return d, nil
}

func (d *DefinitionVector) open() error {
	if d == nil || d.self != d {
		return ast.ErrASTHandle
	}
	if d.closed {
		return arena.ErrClosed
	}
	if len(d.variables) != len(d.definitions) {
		return ast.ErrASTDomain
	}
	return nil
}

func (d *DefinitionVector) Size() (uint32, error) {
	if err := d.open(); err != nil {
		return 0, err
	}
	return uint32(len(d.definitions)), nil
}

func (d *DefinitionVector) Empty() (bool, error) {
	n, err := d.Size()
	return n == 0, err
}

func (d *DefinitionVector) Variable(index uint32) (ast.ASTHandle, error) {
	if err := d.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(d.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return d.manager.RefGet(d.variables[index])
}

func (d *DefinitionVector) Definition(index uint32) (ast.ASTHandle, error) {
	if err := d.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(d.definitions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return d.manager.RefGet(d.definitions[index])
}

// PushBack publishes the declaration before acquiring its parallel definition,
// then rolls the declaration back if the second acquisition is refused.
func (d *DefinitionVector) PushBack(declaration, definition ast.ASTHandle) error {
	if err := d.open(); err != nil {
		return err
	}
	kind, err := d.manager.GetKind(declaration)
	if err != nil || kind != ast.ASTFuncDecl {
		return errors.Join(ast.ErrASTDomain, err)
	}
	variable, err := d.manager.NewRef(declaration)
	if err != nil {
		return err
	}
	d.variables = append(d.variables, variable)
	value, err := d.manager.NewRef(definition)
	if err != nil {
		last := len(d.variables) - 1
		cleanup := d.manager.DelRef(&d.variables[last])
		d.variables = d.variables[:last]
		return errors.Join(err, cleanup)
	}
	d.definitions = append(d.definitions, value)
	return nil
}

func (d *DefinitionVector) Clone() (result *DefinitionVector, err error) {
	if err = d.open(); err != nil {
		return nil, err
	}
	result, err = NewDefinitionVector(d.manager, d.config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for i := range d.variables {
		variable, cause := d.manager.RefGet(d.variables[i])
		if cause != nil {
			return nil, cause
		}
		definition, cause := d.manager.RefGet(d.definitions[i])
		if cause != nil {
			return nil, cause
		}
		if cause = result.PushBack(variable, definition); cause != nil {
			return nil, cause
		}
	}
	return result, nil
}

func (d *DefinitionVector) Append(other *DefinitionVector) error {
	if err := d.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if d.manager != other.manager {
		return ast.ErrASTHandle
	}
	start := len(d.variables)
	for i := range other.variables {
		variable, err := other.manager.RefGet(other.variables[i])
		if err != nil {
			_ = d.Shrink(uint32(start))
			return err
		}
		definition, err := other.manager.RefGet(other.definitions[i])
		if err != nil {
			_ = d.Shrink(uint32(start))
			return err
		}
		if err = d.PushBack(variable, definition); err != nil {
			cleanup := d.Shrink(uint32(start))
			return errors.Join(err, cleanup)
		}
	}
	return nil
}

func (d *DefinitionVector) Shrink(size uint32) error {
	if err := d.open(); err != nil {
		return err
	}
	if size > uint32(len(d.variables)) {
		return arena.ErrBounds
	}
	for len(d.definitions) > int(size) {
		i := len(d.definitions) - 1
		if err := d.manager.DelRef(&d.definitions[i]); err != nil {
			return err
		}
		d.definitions = d.definitions[:i]
		if err := d.manager.DelRef(&d.variables[i]); err != nil {
			return err
		}
		d.variables = d.variables[:i]
	}
	return nil
}

func (d *DefinitionVector) Reset() error { return d.Shrink(0) }

// Normalize applies later definitions to earlier ones in reverse source order.
func (d *DefinitionVector) Normalize() (result error) {
	if result = d.open(); result != nil {
		return result
	}
	if len(d.definitions) <= 1 {
		return nil
	}
	substitution, err := d.manager.NewExprSubstitution()
	if err != nil {
		return err
	}
	replacer, err := astrewriter.NewSimplifyingExprReplacer(d.manager, d.config.Context, d.config.Globals, nil)
	if err != nil {
		if replacer != nil {
			err = errors.Join(err, replacer.Close())
		}
		return errors.Join(err, d.manager.DelExprSubstitution(&substitution))
	}
	defer func() {
		result = errors.Join(result, replacer.SetSubstitution(nil), replacer.Close(), d.manager.DelExprSubstitution(&substitution))
	}()
	for i := len(d.definitions) - 1; i >= 0; i-- {
		if err = replacer.SetSubstitution(&substitution); err != nil {
			return err
		}
		definition, err := d.manager.RefGet(d.definitions[i])
		if err != nil {
			return err
		}
		rewritten, err := d.manager.NewRef(definition)
		if err != nil {
			return err
		}
		if err = replacer.Rewrite(definition, rewritten); err != nil {
			return errors.Join(err, d.manager.DelRef(&rewritten))
		}
		h, err := d.manager.RefGet(rewritten)
		if err != nil {
			return errors.Join(err, d.manager.DelRef(&rewritten))
		}
		if err = d.manager.SetRef(d.definitions[i], h); err != nil {
			return errors.Join(err, d.manager.DelRef(&rewritten))
		}
		if err = d.manager.DelRef(&rewritten); err != nil {
			return err
		}
		declaration, err := d.manager.RefGet(d.variables[i])
		if err != nil {
			return err
		}
		constant, err := d.manager.MkApp(declaration, nil)
		if err != nil {
			return err
		}
		if err = d.manager.ExprSubstitutionInsert(substitution, constant, h, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	return nil
}

// Project keeps the first definition for each requested declaration and drops
// later recycled names and unrelated declarations.
func (d *DefinitionVector) Project(variables []ast.ASTHandle) error {
	if err := d.open(); err != nil {
		return err
	}
	requested := make(map[ast.ASTHandle]bool, len(variables))
	for _, variable := range variables {
		declaration, err := d.manager.GetDecl(variable)
		if err != nil {
			return err
		}
		requested[declaration] = true
	}
	for i := 0; i < len(d.variables); {
		declaration, err := d.manager.RefGet(d.variables[i])
		if err != nil {
			return err
		}
		if requested[declaration] {
			delete(requested, declaration)
			i++
			continue
		}
		if err = d.manager.DelRef(&d.definitions[i]); err != nil {
			return err
		}
		if err = d.manager.DelRef(&d.variables[i]); err != nil {
			return err
		}
		copy(d.variables[i:], d.variables[i+1:])
		copy(d.definitions[i:], d.definitions[i+1:])
		d.variables = d.variables[:len(d.variables)-1]
		d.definitions = d.definitions[:len(d.definitions)-1]
	}
	return nil
}

func (d *DefinitionVector) Close() error {
	if d == nil || d.self != d {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if err := d.Reset(); err != nil {
		return err
	}
	d.closed = true
	return nil
}

// GuardedDefinitions owns guards and independent normalized definition vectors.
type GuardedDefinitions struct {
	self    *GuardedDefinitions
	manager *ast.ASTManager
	config  Config
	guards  []ast.ASTRef
	defs    []*DefinitionVector
	closed  bool
}

func NewGuardedDefinitions(manager *ast.ASTManager, config Config) (*GuardedDefinitions, error) {
	probe, err := NewDefinitionVector(manager, config)
	if err != nil {
		return nil, err
	}
	if err = probe.Close(); err != nil {
		return nil, err
	}
	g := &GuardedDefinitions{manager: manager, config: config}
	g.self = g
	return g, nil
}

func (g *GuardedDefinitions) open() error {
	if g == nil || g.self != g {
		return ast.ErrASTHandle
	}
	if g.closed {
		return arena.ErrClosed
	}
	if len(g.guards) != len(g.defs) {
		return ast.ErrASTDomain
	}
	return nil
}

func (g *GuardedDefinitions) Size() (uint32, error) {
	if err := g.open(); err != nil {
		return 0, err
	}
	return uint32(len(g.guards)), nil
}

func (g *GuardedDefinitions) Guard(index uint32) (ast.ASTHandle, error) {
	if err := g.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(g.guards)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return g.manager.RefGet(g.guards[index])
}

func (g *GuardedDefinitions) DefinitionSize(index uint32) (uint32, error) {
	if err := g.open(); err != nil {
		return 0, err
	}
	if index >= uint32(len(g.defs)) {
		return 0, arena.ErrBounds
	}
	return g.defs[index].Size()
}

func (g *GuardedDefinitions) Variable(branch, index uint32) (ast.ASTHandle, error) {
	if err := g.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if branch >= uint32(len(g.defs)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return g.defs[branch].Variable(index)
}

func (g *GuardedDefinitions) Definition(branch, index uint32) (ast.ASTHandle, error) {
	if err := g.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if branch >= uint32(len(g.defs)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return g.defs[branch].Definition(index)
}

func (g *GuardedDefinitions) Add(guard ast.ASTHandle, definitions *DefinitionVector) (err error) {
	if err = g.open(); err != nil {
		return err
	}
	if err = definitions.open(); err != nil {
		return err
	}
	if definitions.manager != g.manager {
		return ast.ErrASTHandle
	}
	copyDefinitions, err := definitions.Clone()
	if err != nil {
		return err
	}
	guardRef, err := g.manager.NewRef(guard)
	if err != nil {
		return errors.Join(err, copyDefinitions.Close())
	}
	g.defs = append(g.defs, copyDefinitions)
	g.guards = append(g.guards, guardRef)
	return copyDefinitions.Normalize()
}

func (g *GuardedDefinitions) Project(variables []ast.ASTHandle) error {
	if err := g.open(); err != nil {
		return err
	}
	for _, definitions := range g.defs {
		if err := definitions.Project(variables); err != nil {
			return err
		}
	}
	return nil
}

func (g *GuardedDefinitions) Close() error {
	if g == nil || g.self != g {
		return ast.ErrASTHandle
	}
	if g.closed {
		return nil
	}
	for len(g.defs) > 0 {
		i := len(g.defs) - 1
		if err := g.defs[i].Close(); err != nil {
			return err
		}
		g.defs = g.defs[:i]
	}
	for len(g.guards) > 0 {
		i := len(g.guards) - 1
		if err := g.manager.DelRef(&g.guards[i]); err != nil {
			return err
		}
		g.guards = g.guards[:i]
	}
	g.closed = true
	return nil
}
