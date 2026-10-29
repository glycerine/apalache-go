package smtcore

// Source: src/cmd_context/tactic_cmds.h:30-63;
// src/cmd_context/tactic_cmds.cpp:35-50; src/tactic/tactic.h:140-144.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var errTacticRegistryOwner = errors.New("tactic: invalid, copied or retired registration owner")

// The cold reference contract of the source abstract tactic. Concrete tactics
// must additionally supply their original execution/converter algorithms; this
// registration layer neither applies tactics nor manufactures solver results.
type tacticReference interface {
	IncRef() error
	DecRef() error
	Close() error
}
type tacticFactory interface {
	Make(*ast.ASTManager, *uparams.ParamsRef) (tacticReference, error)
	Close() error
}

// The source command owns its factory and borrows its symbol/description.
// ParamsContext provides the source temporary empty params_ref at Make.
type tacticCmd struct {
	self               *tacticCmd
	params             *uparams.ParamsContext
	name               usymbol.Symbol
	description        string
	descriptionPresent bool
	factory            tacticFactory
	registered         *tacticManager
	ownerBytes         uint64
	closed             bool
}

func newTacticCmd(c *uparams.ParamsContext, name usymbol.Symbol, description *string, factory tacticFactory) (*tacticCmd, error) {
	if c == nil || factory == nil {
		return nil, errTacticRegistryOwner
	}
	if e := c.CheckOpen(); e != nil {
		return nil, e
	}
	if _, e := c.Symbols().Hash(name); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(tacticCmd{}).Size())
	if e := c.Budget().Reserve(n); e != nil {
		return nil, e
	}
	cmd := &tacticCmd{params: c, name: name, factory: factory, ownerBytes: n}
	if description != nil {
		cmd.description, cmd.descriptionPresent = *description, true
	}
	cmd.self = cmd
	return cmd, nil
}
func (cmd *tacticCmd) open() error {
	if cmd == nil || cmd.self != cmd || cmd.closed {
		return errTacticRegistryOwner
	}
	return cmd.params.CheckOpen()
}
func (cmd *tacticCmd) Make(m *ast.ASTManager) (value tacticReference, err error) {
	if err = cmd.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	p, err := cmd.params.NewValue() // actual null-backed params_ref temporary
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, cmd.params.Del(&p)) }()
	return cmd.factory.Make(m, &p)
}
func (cmd *tacticCmd) Close() error {
	if cmd == nil || cmd.self != cmd {
		return errTacticRegistryOwner
	}
	if cmd.closed {
		return nil
	}
	if e := cmd.factory.Close(); e != nil {
		return e
	}
	cmd.factory = nil
	cmd.closed = true
	cmd.params.Budget().Release(cmd.ownerBytes)
	cmd.ownerBytes = 0
	return nil
}

type tacticProbeInfo struct {
	self               *tacticProbeInfo
	symbols            *usymbol.Manager
	budget             *arena.Budget
	name               usymbol.Symbol
	description        string
	descriptionPresent bool
	probe              tacticReference
	registered         *tacticManager
	ownerBytes         uint64
	closed             bool
}

func newTacticProbeInfo(s *usymbol.Manager, b *arena.Budget, name usymbol.Symbol, description *string, probe tacticReference) (*tacticProbeInfo, error) {
	if s == nil {
		return nil, usymbol.ErrOwner
	}
	if _, e := s.Hash(name); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(tacticProbeInfo{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	if probe != nil {
		if e := probe.IncRef(); e != nil {
			b.Release(n)
			return nil, e
		}
	}
	p := &tacticProbeInfo{symbols: s, budget: b, name: name, probe: probe, ownerBytes: n}
	if description != nil {
		p.description, p.descriptionPresent = *description, true
	}
	p.self = p
	return p, nil
}
func (p *tacticProbeInfo) open() error {
	if p == nil || p.self != p || p.closed {
		return errTacticRegistryOwner
	}
	_, e := p.symbols.Hash(p.name)
	return e
}
func (p *tacticProbeInfo) Close() error {
	if p == nil || p.self != p {
		return errTacticRegistryOwner
	}
	if p.closed {
		return nil
	}
	if p.probe != nil {
		if e := p.probe.DecRef(); e != nil {
			return e
		}
		p.probe = nil
	}
	p.budget.Release(p.ownerBytes)
	p.ownerBytes = 0
	p.closed = true
	return nil
}
