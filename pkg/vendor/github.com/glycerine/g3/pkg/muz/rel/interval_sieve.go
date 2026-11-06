package rel

// Source: src/muz/rel/dl_sieve_relation.cpp/.h, instantiated with
// interval_relation_plugin for mixed finite/arithmetic product signatures.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// IntervalSieve constrains exactly the Int/Real columns of a mixed signature.
// Other columns are full and deliberately ignored, as in sieve_relation.
type IntervalSieve struct {
	manager    *ast.ASTManager
	sorts      []ast.ASTHandle
	innerCols  []bool
	sigToInner []int
	innerToSig []int
	inner      *IntervalRelation
}

func newIntervalSieve(manager *ast.ASTManager, sorts []ast.ASTHandle, mask []bool, inner *IntervalRelation) (*IntervalSieve, error) {
	if manager == nil || inner == nil || len(sorts) != len(mask) {
		return nil, ast.ErrASTDomain
	}
	result := &IntervalSieve{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), innerCols: append([]bool(nil), mask...), sigToInner: make([]int, len(sorts)), inner: inner}
	for i := range result.sigToInner {
		result.sigToInner[i] = -1
	}
	for outer, selected := range mask {
		if !selected {
			continue
		}
		if !CanHandleIntervalSorts(manager, []ast.ASTHandle{sorts[outer]}) {
			return nil, ast.ErrASTDomain
		}
		result.sigToInner[outer] = len(result.innerToSig)
		result.innerToSig = append(result.innerToSig, outer)
	}
	if arity, err := inner.Arity(); err != nil || int(arity) != len(result.innerToSig) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	return result, nil
}

func intervalSieveMask(manager *ast.ASTManager, sorts []ast.ASTHandle) []bool {
	mask := make([]bool, len(sorts))
	for i, sort := range sorts {
		mask[i] = CanHandleIntervalSorts(manager, []ast.ASTHandle{sort})
	}
	return mask
}

func NewIntervalSieveEmpty(manager *ast.ASTManager, sorts []ast.ASTHandle) (*IntervalSieve, error) {
	mask := intervalSieveMask(manager, sorts)
	count := 0
	for _, selected := range mask {
		if selected {
			count++
		}
	}
	return newIntervalSieve(manager, sorts, mask, NewIntervalEmpty(uint32(count)))
}

func NewIntervalSieveFull(manager *ast.ASTManager, sorts []ast.ASTHandle) (*IntervalSieve, error) {
	mask := intervalSieveMask(manager, sorts)
	count := 0
	for _, selected := range mask {
		if selected {
			count++
		}
	}
	return newIntervalSieve(manager, sorts, mask, NewIntervalFull(uint32(count)))
}

func (r *IntervalSieve) open() error {
	if r == nil || r.manager == nil || r.inner == nil || len(r.sorts) != len(r.innerCols) {
		return ast.ErrASTDomain
	}
	return nil
}

func (r *IntervalSieve) InnerColumns() []bool {
	if r.open() != nil {
		return nil
	}
	return append([]bool(nil), r.innerCols...)
}

func (r *IntervalSieve) Empty() bool { return r == nil || r.inner == nil || r.inner.Empty() }

func (r *IntervalSieve) Clone() (*IntervalSieve, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	inner, err := r.inner.Clone()
	if err != nil {
		return nil, err
	}
	return newIntervalSieve(r.manager, r.sorts, r.innerCols, inner)
}

func (r *IntervalSieve) selected(values []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil || len(values) != len(r.sorts) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	result := make([]ast.ASTHandle, len(r.innerToSig))
	for inner, outer := range r.innerToSig {
		result[inner] = values[outer]
	}
	return result, nil
}

func (r *IntervalSieve) AddFact(values []ast.ASTHandle) error {
	selected, err := r.selected(values)
	if err != nil {
		return err
	}
	sorts, _ := r.selected(r.sorts)
	return r.inner.AddFactExpressions(r.manager, sorts, selected)
}

func (r *IntervalSieve) ContainsFact(values []ast.ASTHandle) (bool, error) {
	selected, err := r.selected(values)
	if err != nil {
		return false, err
	}
	return r.inner.ContainsFactExpressions(r.manager, selected)
}

func (r *IntervalSieve) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := r.open(); err != nil || uint64(column) >= uint64(len(r.sorts)) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	inner := r.sigToInner[column]
	if inner < 0 { // ignored source columns are a deliberate no-op
		return nil
	}
	return r.inner.FilterEqualExpression(r.manager, uint32(inner), value)
}

func (r *IntervalSieve) FilterIdentical(columns []uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	inner := make([]uint32, 0, len(columns))
	for _, column := range columns {
		if uint64(column) >= uint64(len(r.sorts)) {
			return ast.ErrASTDomain
		}
		if mapped := r.sigToInner[column]; mapped >= 0 {
			inner = append(inner, uint32(mapped))
		}
	}
	return r.inner.FilterIdenticalColumns(inner)
}

func (r *IntervalSieve) FilterInterpreted(condition ast.ASTHandle) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	used, err := ast.NewASTUsedVars(r.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil || uint64(used.MaxFoundVarIndexPlusOne()) > uint64(len(r.sorts)) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	bindings := make([]ast.ASTHandle, len(r.sorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	for outer := range r.sorts {
		if used.Contains(uint32(outer)) == (ast.ASTHandle{}) {
			continue
		}
		inner := r.sigToInner[outer]
		if inner < 0 { // source returns the identity operation
			return nil
		}
		variable, e := r.manager.MkVar(uint32(inner), r.sorts[outer])
		if e != nil {
			return e
		}
		ref, e := r.manager.NewRef(variable)
		if e != nil {
			return e
		}
		refs = append(refs, ref)
		bindings[outer] = variable
	}
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, out)
	if err = subst.Substitute(condition, bindings, out); err != nil {
		return err
	}
	rewritten, err := r.manager.RefGet(out)
	if err != nil {
		return err
	}
	return r.inner.FilterInterpreted(r.manager, rewritten)
}
