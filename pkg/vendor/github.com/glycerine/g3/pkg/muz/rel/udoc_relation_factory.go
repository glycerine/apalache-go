package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_plugin manager cache,
// can_handle_signature, mk_empty, and mk_full.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// UDocRelationFactory owns the source plugin's DocManager cache and borrows
// the AST manager. The fixedpoint context closes its relations before Close.
type UDocRelationFactory struct {
	manager  *ast.ASTManager
	config   arena.VectorConfig
	managers map[uint32]*DocManager
	closed   bool
	checker  *smtcore.SMTSolverConfig
}

type TypedUDocRelation struct {
	factory  *UDocRelationFactory
	sorts    []ast.ASTHandle
	relation *UDocRelation
	closed   bool
	checker  *udocFormulaChecker
}

func NewUDocRelationFactory(manager *ast.ASTManager, config arena.VectorConfig) (*UDocRelationFactory, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	return &UDocRelationFactory{manager: manager, config: config, managers: make(map[uint32]*DocManager)}, nil
}

// NewFormulaCheckedUDocFactory selects the source check_relation(doc)
// specialization. Every relation and derived operation still uses UDoc storage.
func NewFormulaCheckedUDocFactory(manager *ast.ASTManager, config arena.VectorConfig, checker smtcore.SMTSolverConfig) (*UDocRelationFactory, error) {
	if manager == nil || checker.Params == nil {
		return nil, ast.ErrASTDomain
	}
	if err := checker.Params.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	factory, err := NewUDocRelationFactory(manager, config)
	if err == nil {
		factory.checker = &checker
	}
	return factory, err
}

func (f *UDocRelationFactory) FormulaChecked() bool { return f != nil && f.checker != nil }

func (*UDocRelationFactory) Name() string { return "doc" }

func (f *UDocRelationFactory) open() error {
	if f == nil || f.closed || f.manager == nil || f.managers == nil {
		return arena.ErrClosed
	}
	return nil
}

func (f *UDocRelationFactory) sortWidth(sort ast.ASTHandle) (uint32, bool, error) {
	if err := f.open(); err != nil {
		return 0, false, err
	}
	bitVector, err := f.manager.IsBitVectorSort(sort)
	if err != nil {
		return 0, false, err
	}
	if bitVector {
		width, err := f.manager.BitVectorSize(sort)
		return width, err == nil, err
	}
	size, finite, err := f.manager.DatalogFiniteSortSize(sort)
	if err != nil || !finite {
		return 0, false, err
	}
	width := uint32(0)
	for size != 0 {
		width++
		size /= 2
	}
	return width, true, nil
}

func (f *UDocRelationFactory) SignatureWidths(sorts []ast.ASTHandle) ([]uint32, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	widths := make([]uint32, len(sorts))
	var total uint64
	for i, sort := range sorts {
		width, accepted, err := f.sortWidth(sort)
		if err != nil {
			return nil, err
		}
		if !accepted || width == 0 {
			return nil, ast.ErrASTDomain
		}
		widths[i] = width
		total += uint64(width)
		if total > math.MaxUint32 {
			return nil, arena.ErrBounds
		}
	}
	return widths, nil
}

func (f *UDocRelationFactory) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	_, err := f.SignatureWidths(sorts)
	if errors.Is(err, ast.ErrASTDomain) {
		return false, nil
	}
	return err == nil, err
}

func (f *UDocRelationFactory) managerFor(widths []uint32) (*DocManager, error) {
	var total uint32
	for _, width := range widths {
		if width == 0 || width > math.MaxUint32-total {
			return nil, arena.ErrBounds
		}
		total += width
	}
	if manager := f.managers[total]; manager != nil {
		return manager, nil
	}
	manager, err := NewDocManager(total, f.config)
	if err != nil {
		return nil, err
	}
	f.managers[total] = manager
	return manager, nil
}

func (f *UDocRelationFactory) NewEmpty(sorts []ast.ASTHandle) (*TypedUDocRelation, error) {
	widths, err := f.SignatureWidths(sorts)
	if err != nil {
		return nil, err
	}
	manager, err := f.managerFor(widths)
	if err != nil {
		return nil, err
	}
	relation, err := NewUDocRelation(manager, widths)
	if err != nil {
		return nil, err
	}
	result := &TypedUDocRelation{factory: f, sorts: append([]ast.ASTHandle(nil), sorts...), relation: relation}
	if f.checker != nil {
		falsity, err := f.manager.MkFalse()
		if err == nil {
			err = result.installChecker(falsity)
		}
		if err != nil {
			return nil, finishCheckedUDocConstruction(err, result, ast.ASTRef{})
		}
	}
	return result, nil
}

func (f *UDocRelationFactory) NewFull(sorts []ast.ASTHandle) (result *TypedUDocRelation, resultErr error) {
	result, err := f.NewEmpty(sorts)
	if err != nil {
		return nil, err
	}
	full, err := result.relation.manager.AllocateX()
	if err == nil {
		err = result.relation.elems.PushOwned(full)
	}
	if err != nil {
		return nil, finishCheckedUDocConstruction(err, result, ast.ASTRef{})
	}
	if result.checker != nil {
		truth, err := f.manager.MkTrue()
		if err == nil {
			err = result.checkFormula(truth)
		}
		if err != nil {
			return nil, finishCheckedUDocConstruction(err, result, ast.ASTRef{})
		}
	}
	return result, nil
}

func (r *TypedUDocRelation) open() error {
	if r == nil || r.closed || r.factory == nil || r.factory.open() != nil || r.relation == nil {
		return arena.ErrClosed
	}
	return r.relation.check()
}

func (r *TypedUDocRelation) Sorts() []ast.ASTHandle {
	if r.open() != nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.sorts...)
}

func (r *TypedUDocRelation) Core() *UDocRelation {
	if r.open() != nil {
		return nil
	}
	return r.relation
}

func (r *TypedUDocRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	if r.checker != nil {
		if err := r.checker.manager.DelRef(&r.checker.formula); err != nil {
			return err
		}
		r.checker = nil
	}
	err := r.relation.Close()
	r.closed = true
	r.factory = nil
	r.sorts = nil
	r.relation = nil
	return err
}

func (f *UDocRelationFactory) Close() {
	if f == nil || f.closed {
		return
	}
	for _, manager := range f.managers {
		manager.Close()
	}
	f.closed = true
	f.manager = nil
	f.managers = nil
}
