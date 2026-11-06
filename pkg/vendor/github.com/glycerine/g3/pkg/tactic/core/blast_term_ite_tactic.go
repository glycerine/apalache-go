package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type BlastTermITETactic struct {
	self        *BlastTermITETactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	services    model.ModelEvaluatorConfig
	params      uparams.ParamsRef
	paramsReady bool
	config      blastTermITEConfiguration
	traversal   *rewriter.Traversal
	maxMemory   uint64
	numFresh    uint32
	ownerBytes  uint64
	refs        uint32
	closed      bool
	running     bool
}

var _ tactic.Tactic = (*BlastTermITETactic)(nil)

func NewBlastTermITETactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *BlastTermITETactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BlastTermITETactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &BlastTermITETactic{manager: m, context: c, globals: g, services: services, ownerBytes: n}
	t.self = t
	t.config.owner = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err == nil {
		err = t.newTraversal()
	}
	return t, err
}

func MakeBlastTermITETactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewBlastTermITETactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}
func (t *BlastTermITETactic) newTraversal() (err error) {
	proofs, err := t.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		t.traversal, err = rewriter.NewProofTraversal(t.manager, &t.config)
	} else {
		t.traversal, err = rewriter.NewTraversal(t.manager, &t.config)
	}
	return err
}
func (t *BlastTermITETactic) readPolicy(p *uparams.ParamsRef) error {
	name := "max_memory"
	mb, err := t.context.GetUIntName(p, &name, ^uint32(0))
	t.maxMemory = umemory.MegabytesToBytes(mb)
	return err
}
func (t *BlastTermITETactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.traversal == nil {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *BlastTermITETactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *BlastTermITETactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *BlastTermITETactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.traversal != nil {
		if err = t.traversal.Close(); err != nil {
			return err
		}
		t.traversal = nil
	}
	if t.paramsReady {
		if err = t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *BlastTermITETactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.traversal.Close(); err != nil {
		return err
	}
	t.traversal = nil
	t.numFresh = 0
	return t.newTraversal()
}
func (t *BlastTermITETactic) Reset() error { return t.Cleanup() }
func (t *BlastTermITETactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewBlastTermITETactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *BlastTermITETactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(p)
}
func (t *BlastTermITETactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	if err := uparams.InsertMaxSteps(out); err != nil {
		return err
	}
	description := "(default: 128) maximum number of arguments (per application) that will be considered by the greedy (quadratic) heuristic."
	return out.InsertName("max_args", uparams.ParamUInt, &description, nil, nil)
}
func (t *BlastTermITETactic) CollectStatistics() map[string]float64 {
	return nil
}
func (t *BlastTermITETactic) ResetStatistics() error {
	return t.open()
}
func (t *BlastTermITETactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *BlastTermITETactic) SetProgressCallback(func(uint32)) error { return t.open() }
