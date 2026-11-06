package rel

// Source: src/muz/rel/dl_bound_relation.cpp
// bound_relation_plugin::can_handle_signature/mk_empty/mk_full.

import "github.com/glycerine/g3/pkg/ast"

// BoundRelationPlugin is the concrete bound backend registered by rel_context.
// It borrows its AST manager; constructed relations contain only Go-owned
// column state and do not borrow either the plugin or the manager.
type BoundRelationPlugin struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
}

func NewBoundRelationPlugin(manager *ast.ASTManager) (*BoundRelationPlugin, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	return &BoundRelationPlugin{manager: manager, arithmetic: family}, nil
}

func (*BoundRelationPlugin) Name() string { return "bound_relation" }

// CanHandleSignature accepts exactly Int and Real columns, including the
// source's vacuously supported nullary signature.
func (p *BoundRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	if p == nil || p.manager == nil {
		return false, ast.ErrASTDomain
	}
	for _, sort := range sorts {
		kind, err := p.manager.GetKind(sort)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTSort {
			return false, nil
		}
		family, err := p.manager.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		if family != p.arithmetic {
			return false, nil
		}
		declaration, err := p.manager.GetDeclKind(sort)
		if err != nil {
			return false, err
		}
		// arith_decl_plugin.h declares REAL_SORT then INT_SORT.
		if declaration != 0 && declaration != 1 {
			return false, nil
		}
	}
	return true, nil
}

func (p *BoundRelationPlugin) NewEmpty(sorts []ast.ASTHandle) (*BoundRelation, error) {
	accepted, err := p.CanHandleSignature(sorts)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, ast.ErrASTDomain
	}
	return NewBoundRelation(uint32(len(sorts)), true), nil
}

func (p *BoundRelationPlugin) NewFull(sorts []ast.ASTHandle) (*BoundRelation, error) {
	accepted, err := p.CanHandleSignature(sorts)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, ast.ErrASTDomain
	}
	return NewBoundRelation(uint32(len(sorts)), false), nil
}

// Join is the concrete typed form of bound_relation_plugin::mk_join_fn and
// join_fn::operator(). Static Go types replace the source's dynamic kind
// checks; the result owns its column state independently of both operands.
func (p *BoundRelationPlugin) Join(left, right *BoundRelation, leftColumns, rightColumns []uint32) (*BoundRelation, error) {
	if p == nil || p.manager == nil {
		return nil, ast.ErrASTDomain
	}
	return JoinBoundRelations(left, right, leftColumns, rightColumns)
}

// Project is the concrete typed form of
// bound_relation_plugin::mk_project_fn/project_fn::operator().
func (p *BoundRelationPlugin) Project(source *BoundRelation, removedColumns []uint32) (*BoundRelation, error) {
	if p == nil || p.manager == nil {
		return nil, ast.ErrASTDomain
	}
	return source.Project(removedColumns)
}

// Rename is the concrete typed form of
// bound_relation_plugin::mk_rename_fn/rename_fn::operator().
func (p *BoundRelationPlugin) Rename(source *BoundRelation, cycle []uint32) (*BoundRelation, error) {
	if p == nil || p.manager == nil {
		return nil, ast.ErrASTDomain
	}
	return source.Rename(cycle)
}

// Union is the concrete bound-to-bound form of
// bound_relation_plugin::mk_union_fn/mk_widen_fn and union_fn::operator().
// A non-nil delta receives the resulting target exactly when the lattice
// value changes, following vector_relation::mk_union.
func (p *BoundRelationPlugin) Union(target, source, delta *BoundRelation, widen bool) (bool, error) {
	if p == nil || p.manager == nil {
		return false, ast.ErrASTDomain
	}
	if delta != nil {
		return target.UnionWithDelta(source, delta, widen)
	}
	if widen {
		return target.Widen(source)
	}
	return target.Union(source)
}

// UnionInterval is the concrete interval-source branch selected by
// bound_relation_plugin::mk_union_fn/mk_widen_fn. Source mk_union_i uses the
// same bound pruning for union and widening and deliberately does not publish
// its optional delta.
func (p *BoundRelationPlugin) UnionInterval(target *BoundRelation, source *IntervalRelation, _ *BoundRelation, _ bool) error {
	if p == nil || p.manager == nil {
		return ast.ErrASTDomain
	}
	return target.UnionInterval(source)
}
