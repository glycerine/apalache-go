package qe

// Source: src/qe/qe_mbp.cpp/.h mbp::impl and public forwarding methods.

import (
	"errors"
	"reflect"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// ModelBasedProjection owns the concrete retained arithmetic, datatype and
// array projectors. Dispatch follows source family-id order.
type ModelBasedProjection struct {
	self         *ModelBasedProjection
	manager      *ast.ASTManager
	config       Config
	preprocessor *projectionPreprocessor
	arithmetic   *ArithmeticProjection
	datatype     *DatatypeProjection
	array        *ArrayProjection
	families     []ast.FamilyID
	pending      []*modelProjectionCall
	bytes        uint64
	closed       bool
}
type modelProjectionCall struct {
	scratch    *projectionPreprocessCall
	vectors    []*[]ast.ASTRef
	maximizers []*ArithmeticProjection
	failures   []memory.ConstructionFailure
}
type ModelProjectionFailure struct {
	owner          *ModelBasedProjection
	cause, cleanup error
	constructor    bool
}

func (e *ModelProjectionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ModelProjectionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ModelProjectionFailure) OriginalCause() error { return e.cause }
func (e *ModelProjectionFailure) CleanupCause() error  { return e.cleanup }
func (e *ModelProjectionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *ModelProjectionFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func NewModelBasedProjection(m *ast.ASTManager, config Config) (result *ModelBasedProjection, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ModelBasedProjection{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	p := &ModelBasedProjection{manager: m, config: config, bytes: n}
	p.self = p
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				p.pending = append(p.pending, &modelProjectionCall{failures: []memory.ConstructionFailure{failure}})
			}
			if cleanup := p.Close(); cleanup != nil {
				result = p
				err = &ModelProjectionFailure{owner: p, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	p.preprocessor, err = newProjectionPreprocessor(m, config)
	if err != nil {
		return nil, err
	}
	p.arithmetic, err = NewArithmeticProjection(m, config)
	if err != nil {
		return nil, err
	}
	p.datatype, err = NewDatatypeProjection(m, config)
	if err != nil {
		return nil, err
	}
	p.array, err = NewArrayProjection(m, config)
	if err != nil {
		return nil, err
	}
	p.families = []ast.FamilyID{p.arithmetic.family, p.datatype.family, p.array.family}
	sort.Slice(p.families, func(i, j int) bool { return p.families[i] < p.families[j] })
	return p, nil
}
func (p *ModelBasedProjection) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *ModelBasedProjection) begin(md *model.ModelCore) (*modelProjectionCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &modelProjectionCall{scratch: &projectionPreprocessCall{owner: p.preprocessor}}
	p.pending = append(p.pending, c)
	return c, nil
}
func (p *ModelBasedProjection) drain() error {
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		c := p.pending[i]
		// Child failure receipts must be drained before their borrowed inputs.
		for len(c.failures) > 0 {
			j := len(c.failures) - 1
			if err := c.failures[j].Cleanup(); err != nil {
				return err
			}
			c.failures = c.failures[:j]
		}
		if c.scratch != nil {
			if err := c.scratch.close(); err != nil {
				return err
			}
		}
		for len(c.maximizers) > 0 {
			j := len(c.maximizers) - 1
			if err := c.maximizers[j].Close(); err != nil {
				c.scratch.retain(err)
				return err
			}
			c.maximizers = c.maximizers[:j]
		}
		for len(c.vectors) > 0 {
			j := len(c.vectors) - 1
			if err := p.preprocessor.shrink(c.vectors[j], 0); err != nil {
				c.scratch.retain(err)
				return err
			}
			c.vectors = c.vectors[:j]
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *ModelBasedProjection) finish(c *modelProjectionCall, err *error) {
	c.scratch.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &ModelProjectionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *ModelBasedProjection) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	// Source destroys plugins in family-id order; incomplete construction has
	// no published dispatch vector and is cleaned in construction order.
	if len(p.families) > 0 {
		for _, fid := range p.families {
			if err := p.closeFamily(fid); err != nil {
				return err
			}
		}
	} else {
		if p.arithmetic != nil {
			if err := p.arithmetic.Close(); err != nil {
				return err
			}
			p.arithmetic = nil
		}
		if p.datatype != nil {
			if err := p.datatype.Close(); err != nil {
				return err
			}
			p.datatype = nil
		}
		if p.array != nil {
			if err := p.array.Close(); err != nil {
				return err
			}
			p.array = nil
		}
	}
	if p.preprocessor != nil {
		if err := p.preprocessor.Close(); err != nil {
			return err
		}
		p.preprocessor = nil
	}
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (p *ModelBasedProjection) closeFamily(fid ast.FamilyID) error {
	if p.arithmetic != nil {
		f, err := p.arithmetic.FamilyID()
		if err != nil {
			return err
		}
		if f == fid {
			if err = p.arithmetic.Close(); err != nil {
				return err
			}
			p.arithmetic = nil
			return nil
		}
	}
	if p.datatype != nil {
		f, err := p.datatype.FamilyID()
		if err != nil {
			return err
		}
		if f == fid {
			if err = p.datatype.Close(); err != nil {
				return err
			}
			p.datatype = nil
			return nil
		}
	}
	if p.array != nil {
		f, err := p.array.FamilyID()
		if err != nil {
			return err
		}
		if f == fid {
			if err = p.array.Close(); err != nil {
				return err
			}
			p.array = nil
		}
	}
	return nil
}

func (p *ModelBasedProjection) preprocess(md *model.ModelCore, vars, forms *[]ast.ASTRef) error {
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	if err := p.preprocessor.extractLiterals(md, forms); err != nil {
		return err
	}
	change := true
	for change && len(*vars) > 0 {
		var err error
		change, err = p.preprocessor.solveEqualities(vars, forms)
		if err != nil {
			return err
		}
		for _, fid := range p.families {
			var changed bool
			if f, e := p.arithmetic.FamilyID(); e != nil {
				return e
			} else if f == fid {
				changed, err = p.arithmetic.Solve(md, vars, forms)
			}
			if f, e := p.datatype.FamilyID(); e != nil {
				return e
			} else if f == fid {
				changed, err = p.datatype.Solve(md, vars, forms)
			}
			if f, e := p.array.FamilyID(); e != nil {
				return e
			} else if f == fid {
				changed, err = p.array.Solve(md, vars, forms)
			}
			if err != nil {
				return err
			}
			change = change || changed
		}
	}
	return nil
}

// Solve runs source cheap solving to a fixed point, including datatype lifting.
func (p *ModelBasedProjection) Solve(md *model.ModelCore, vars, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin(md)
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	return p.preprocess(md, vars, forms)
}
func (p *ModelBasedProjection) ExtractLiterals(md *model.ModelCore, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin(md)
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if forms == nil {
		return ast.ErrASTHandle
	}
	return p.preprocessor.extractLiterals(md, forms)
}

// Maximize uses a fresh arithmetic projector, as the source forwarding method
// does, rather than changing the persistent arithmetic dispatch member.
func (p *ModelBasedProjection) Maximize(md *model.ModelCore, forms []ast.ASTRef, objective ast.ASTHandle, ge, gt ast.ASTRef) (value simplex.Optimum, err error) {
	c, err := p.begin(md)
	if err != nil {
		return value, err
	}
	defer p.finish(c, &err)
	a, err := NewArithmeticProjection(p.manager, p.config)
	if a != nil {
		c.maximizers = append(c.maximizers, a)
	}
	if err != nil {
		return value, err
	}
	return a.Maximize(md, forms, objective, ge, gt)
}
