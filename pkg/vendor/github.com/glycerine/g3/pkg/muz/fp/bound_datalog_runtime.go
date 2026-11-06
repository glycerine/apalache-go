package fp

// Source: dl_bound_relation.cpp bound_relation::add_fact and rel_context's
// imprecise relation query status. Positive Horn rules are lowered by the
// companion bound_datalog_positive implementation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

var ErrBoundDatalogFragment = errors.New("muz/fp: unsupported arithmetic bound Datalog fragment")

type boundDatalogEntry struct {
	sorts []ast.ASTHandle
	seed  *rel.BoundRelation
	root  ast.ASTRef
}

// boundDatalogRuntime owns declaration pins and seed facts, and publishes a
// separate saturated snapshot only after the complete supported program succeeds.
type boundDatalogRuntime struct {
	manager           *ast.ASTManager
	rules             *base.RuleSet
	entries           map[ast.ASTHandle]*boundDatalogEntry
	order             []ast.ASTHandle
	evaluated         map[ast.ASTHandle]*rel.BoundRelation
	similarity        *rel.BoundSimilarityProgram
	similarityRuntime *boundDatalogRuntime
	similarityCleanup memory.ConstructionFailure
	pending           []ast.ASTRef
	closed            bool
	formulaCheck      *smtcore.SMTSolverConfig
}

type boundDatalogDeclaration struct {
	declaration ast.ASTHandle
	sorts       []ast.ASTHandle
}

// boundDatalogCleanupReceipt preserves a partially constructed runtime when
// native AST cleanup refuses. Cleanup may be retried without recovering an
// otherwise unreachable owner from a joined error.
type boundDatalogCleanupReceipt struct {
	original error
	cleanup  error
	owner    *memory.Runtime
	runtime  *boundDatalogRuntime
}

func (e *boundDatalogCleanupReceipt) Error() string {
	return errors.Join(e.original, e.cleanup).Error()
}
func (e *boundDatalogCleanupReceipt) Unwrap() error                 { return errors.Join(e.original, e.cleanup) }
func (e *boundDatalogCleanupReceipt) RuntimeOwner() *memory.Runtime { return e.owner }
func (e *boundDatalogCleanupReceipt) OriginalCause() error          { return e.original }
func (e *boundDatalogCleanupReceipt) CleanupCause() error           { return e.cleanup }
func (e *boundDatalogCleanupReceipt) Cleanup() error {
	if e == nil || e.runtime == nil {
		return nil
	}
	e.cleanup = e.runtime.close()
	if e.cleanup == nil {
		e.runtime = nil
	}
	return e.cleanup
}

// boundDatalogDeclarations validates the complete declaration list without
// acquiring declaration roots or changing an existing runtime. This is the
// source plugin's signature gate, including its vacuously supported nullary
// relations.
func boundDatalogDeclarations(manager *ast.ASTManager, declarations []ast.ASTHandle) ([]boundDatalogDeclaration, bool, error) {
	if manager == nil || len(declarations) == 0 {
		return nil, false, nil
	}
	plugin, err := rel.NewBoundRelationPlugin(manager)
	if err != nil {
		return nil, false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, false, err
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	validated := make([]boundDatalogDeclaration, 0, len(declarations))
	for _, declaration := range declarations {
		kind, err := manager.GetKind(declaration)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTFuncDecl || seen[declaration] {
			return nil, false, ErrBoundDatalogFragment
		}
		seen[declaration] = true
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return nil, false, err
		}
		if rangeSort != boolean {
			return nil, false, nil
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, false, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, false, err
			}
		}
		capable, err := plugin.CanHandleSignature(sorts)
		if err != nil {
			return nil, false, err
		}
		if !capable {
			return nil, false, nil
		}
		validated = append(validated, boundDatalogDeclaration{declaration: declaration, sorts: sorts})
	}
	return validated, true, nil
}

// newBoundDatalogActivation distinguishes an unsupported signature from an
// error after the bound backend has been selected.
func newBoundDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (*boundDatalogRuntime, bool, error) {
	_, selected, err := boundDatalogDeclarations(manager, declarations)
	if err != nil || !selected {
		return nil, selected, err
	}
	runtime, err := newBoundDatalogRuntime(manager, rules, declarations)
	return runtime, true, err
}

func newBoundDatalogRuntime(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (_ *boundDatalogRuntime, resultErr error) {
	if rules == nil {
		return nil, ErrBoundDatalogFragment
	}
	validated, selected, err := boundDatalogDeclarations(manager, declarations)
	if err != nil {
		return nil, err
	}
	if !selected {
		return nil, ErrBoundDatalogFragment
	}
	r := &boundDatalogRuntime{manager: manager, rules: rules, entries: make(map[ast.ASTHandle]*boundDatalogEntry, len(declarations))}
	defer func() {
		if resultErr != nil {
			if cleanup := r.close(); cleanup != nil {
				receipt := &boundDatalogCleanupReceipt{original: resultErr, cleanup: cleanup, runtime: r}
				if service := r.manager.NumeralService(); service != nil {
					receipt.owner = service.RuntimeOwner()
				}
				var failure memory.ConstructionFailure
				if errors.As(resultErr, &failure) || errors.As(cleanup, &failure) {
					if owner := failure.RuntimeOwner(); owner != nil {
						receipt.owner = owner
					}
				}
				resultErr = receipt
			}
		}
	}()
	for _, item := range validated {
		declaration, sorts := item.declaration, item.sorts
		root, err := manager.NewRef(declaration)
		if err != nil {
			return nil, err
		}
		r.entries[declaration] = &boundDatalogEntry{sorts: sorts, seed: rel.NewBoundRelation(uint32(len(sorts)), true), root: root}
		r.order = append(r.order, declaration)
	}
	return r, nil
}

// syncPredicates monotonically registers late predicates and synthetic
// nullary query heads. The complete list is checked before roots, seeds, or a
// published result are changed.
func (r *boundDatalogRuntime) syncPredicates(declarations []ast.ASTHandle) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.retirePending(); err != nil {
		return err
	}
	validated, selected, err := boundDatalogDeclarations(r.manager, declarations)
	if err != nil {
		return err
	}
	if !selected {
		return ErrBoundDatalogFragment
	}
	current := make(map[ast.ASTHandle]bool, len(validated))
	for _, item := range validated {
		current[item.declaration] = true
	}
	for _, declaration := range r.order {
		if !current[declaration] {
			return ErrBoundDatalogFragment
		}
	}
	additions := make([]boundDatalogDeclaration, 0, len(validated)-len(r.entries))
	for _, item := range validated {
		if r.entries[item.declaration] == nil {
			additions = append(additions, item)
		}
	}
	roots := make([]ast.ASTRef, 0, len(additions))
	defer func() {
		if resultErr == nil {
			return
		}
		r.pending = append(r.pending, roots...)
		resultErr = errors.Join(resultErr, r.retirePending())
	}()
	for _, item := range additions {
		root, err := r.manager.NewRef(item.declaration)
		if err != nil {
			return err
		}
		roots = append(roots, root)
	}
	for i, item := range additions {
		seed, err := r.newRelation(item.sorts, false)
		if err != nil {
			return err
		}
		r.entries[item.declaration] = &boundDatalogEntry{
			sorts: item.sorts,
			seed:  seed,
			root:  roots[i],
		}
		r.order = append(r.order, item.declaration)
	}
	roots = nil
	if len(additions) != 0 {
		r.evaluated = nil
	}
	return nil
}

// clone copies seed abstractions while borrowing a destination clone's rule
// set. Published saturation state is deliberately not shared.
func (r *boundDatalogRuntime) clone(rules *base.RuleSet) (_ *boundDatalogRuntime, resultErr error) {
	if err := r.open(); err != nil || rules == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return nil, err
	}
	copyRuntime, err := newBoundDatalogRuntime(r.manager, rules, r.order)
	if err != nil {
		return nil, err
	}
	if r.formulaCheck != nil {
		config := *r.formulaCheck
		copyRuntime.formulaCheck = &config
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, copyRuntime.close())
		}
	}()
	for declaration, entry := range r.entries {
		copyRuntime.entries[declaration].seed = entry.seed.Clone()
		if copyRuntime.formulaCheck != nil {
			if err := rel.VerifyBoundRelationClone(r.manager, entry.sorts, *copyRuntime.formulaCheck, entry.seed, copyRuntime.entries[declaration].seed); err != nil {
				return nil, err
			}
		}
	}
	return copyRuntime, nil
}

func (r *boundDatalogRuntime) open() error {
	if r == nil || r.closed || r.manager == nil || r.rules == nil || r.entries == nil {
		return ErrBoundDatalogFragment
	}
	return nil
}

func (r *boundDatalogRuntime) newRelation(sorts []ast.ASTHandle, full bool) (*rel.BoundRelation, error) {
	relation := rel.NewBoundRelation(uint32(len(sorts)), !full)
	if r.formulaCheck != nil {
		if err := rel.VerifyBoundRelationConstant(r.manager, sorts, *r.formulaCheck, relation, full); err != nil {
			return nil, err
		}
	}
	return relation, nil
}

func (r *boundDatalogRuntime) verifySeedRelations() error {
	if r.formulaCheck == nil {
		return nil
	}
	for _, declaration := range r.order {
		entry := r.entries[declaration]
		if err := rel.VerifyBoundRelationConstant(r.manager, entry.sorts, *r.formulaCheck, entry.seed, false); err != nil {
			return err
		}
	}
	return nil
}

// retirePending keeps every staged declaration owner attached to the runtime
// until native deletion succeeds, so a refused rollback remains retryable.
func (r *boundDatalogRuntime) retirePending() (result error) {
	if r == nil || r.manager == nil {
		return ErrBoundDatalogFragment
	}
	for i := len(r.pending) - 1; i >= 0; i-- {
		if r.pending[i] != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&r.pending[i]))
		}
	}
	kept := r.pending[:0]
	for _, root := range r.pending {
		if root != (ast.ASTRef{}) {
			kept = append(kept, root)
		}
	}
	r.pending = kept
	return result
}

func (r *boundDatalogRuntime) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	entry := r.entries[declaration]
	if entry == nil || len(values) != len(entry.sorts) {
		return ErrBoundDatalogFragment
	}
	for i, value := range values {
		sort, err := r.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != entry.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	if err := r.addFactTo(entry.seed, entry.sorts, values); err != nil {
		return err
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) addFactTo(target *rel.BoundRelation, sorts, values []ast.ASTHandle) (resultErr error) {
	if r.formulaCheck == nil {
		return target.AddFact(values)
	}
	beforeFormula, err := target.ToFormula(r.manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&beforeFormula)) }()
	before, err := r.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	if err = target.AddFact(values); err != nil {
		return err
	}
	return rel.VerifyBoundRelationFactAdd(r.manager, sorts, *r.formulaCheck, before, values, target)
}

func (r *boundDatalogRuntime) filterIdentical(target *rel.BoundRelation, sorts []ast.ASTHandle, columns []uint32) (resultErr error) {
	if r.formulaCheck == nil {
		return target.FilterIdentical(columns)
	}
	beforeFormula, err := target.ToFormula(r.manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&beforeFormula)) }()
	before, err := r.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	equalities := make([]ast.ASTHandle, 0, len(columns)-1)
	for _, column := range columns[1:] {
		left, err := r.manager.MkVar(columns[0], sorts[columns[0]])
		if err != nil {
			return err
		}
		leftRef, err := r.manager.NewRef(left)
		if err != nil {
			return err
		}
		refs = append(refs, leftRef)
		right, err := r.manager.MkVar(column, sorts[column])
		if err != nil {
			return err
		}
		rightRef, err := r.manager.NewRef(right)
		if err != nil {
			return err
		}
		refs = append(refs, rightRef)
		equality, err := r.manager.MkEq(left, right)
		if err != nil {
			return err
		}
		equalityRef, err := r.manager.NewRef(equality)
		if err != nil {
			return err
		}
		refs = append(refs, equalityRef)
		equalities = append(equalities, equality)
	}
	condition, err := r.manager.MkAndArgs(equalities)
	if err != nil {
		return err
	}
	conditionRef, err := r.manager.NewRef(condition)
	if err != nil {
		return err
	}
	refs = append(refs, conditionRef)
	if err = target.FilterIdentical(columns); err != nil {
		return err
	}
	return rel.VerifyBoundRelationFilter(r.manager, sorts, *r.formulaCheck, before, condition, target, true)
}

func (r *boundDatalogRuntime) filterEqual(target *rel.BoundRelation, sorts []ast.ASTHandle, column uint32, value ast.ASTHandle) (resultErr error) {
	if r.formulaCheck == nil {
		target.FilterEqual(rel.Element(0), column)
		return nil
	}
	beforeFormula, err := target.ToFormula(r.manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&beforeFormula)) }()
	before, err := r.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	variable, err := r.manager.MkVar(column, sorts[column])
	if err != nil {
		return err
	}
	variableRef, err := r.manager.NewRef(variable)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&variableRef)) }()
	condition, err := r.manager.MkEq(variable, value)
	if err != nil {
		return err
	}
	conditionRef, err := r.manager.NewRef(condition)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&conditionRef)) }()
	target.FilterEqual(rel.Element(0), column)
	return rel.VerifyBoundRelationFilter(r.manager, sorts, *r.formulaCheck, before, condition, target, false)
}

func (r *boundDatalogRuntime) filterInterpreted(target *rel.BoundRelation, sorts []ast.ASTHandle, condition ast.ASTHandle) (resultErr error) {
	if r.formulaCheck == nil {
		_, err := target.FilterInterpreted(r.manager, condition)
		return err
	}
	beforeFormula, err := target.ToFormula(r.manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&beforeFormula)) }()
	before, err := r.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	if _, err = target.FilterInterpreted(r.manager, condition); err != nil {
		return err
	}
	return rel.VerifyBoundRelationFilter(r.manager, sorts, *r.formulaCheck, before, condition, target, true)
}

func (r *boundDatalogRuntime) join(left *rel.BoundRelation, leftSorts []ast.ASTHandle, right *rel.BoundRelation, rightSorts []ast.ASTHandle, leftColumns, rightColumns []uint32) (*rel.BoundRelation, error) {
	joined, err := rel.JoinBoundRelations(left, right, leftColumns, rightColumns)
	if err != nil || r.formulaCheck == nil {
		return joined, err
	}
	if err = rel.VerifyBoundRelationJoin(r.manager, leftSorts, rightSorts, *r.formulaCheck, left, right, leftColumns, rightColumns, joined); err != nil {
		return nil, err
	}
	return joined, nil
}

func (r *boundDatalogRuntime) project(source *rel.BoundRelation, sorts []ast.ASTHandle, removed []uint32) (*rel.BoundRelation, []ast.ASTHandle, error) {
	projected, err := source.Project(removed)
	if err != nil {
		return nil, nil, err
	}
	resultSorts := make([]ast.ASTHandle, 0, len(sorts)-len(removed))
	removedIndex := 0
	for i, sort := range sorts {
		if removedIndex < len(removed) && removed[removedIndex] == uint32(i) {
			removedIndex++
			continue
		}
		resultSorts = append(resultSorts, sort)
	}
	if r.formulaCheck == nil {
		return projected, resultSorts, nil
	}
	formula, err := source.ToFormula(r.manager, sorts)
	if err != nil {
		return nil, nil, err
	}
	defer r.manager.DelRef(&formula)
	handle, err := r.manager.RefGet(formula)
	if err != nil {
		return nil, nil, err
	}
	removedInts := make([]int, len(removed))
	for i, column := range removed {
		removedInts[i] = int(column)
	}
	if err = rel.VerifyBoundRelationProject(r.manager, sorts, resultSorts, *r.formulaCheck, handle, removedInts, projected); err != nil {
		return nil, nil, err
	}
	return projected, resultSorts, nil
}

func (r *boundDatalogRuntime) rename(source *rel.BoundRelation, sorts []ast.ASTHandle, cycle []uint32) (*rel.BoundRelation, []ast.ASTHandle, error) {
	renamed, err := source.Rename(cycle)
	if err != nil {
		return nil, nil, err
	}
	resultSorts := append([]ast.ASTHandle(nil), sorts...)
	for i, column := range cycle {
		destination := cycle[(i+1)%len(cycle)]
		resultSorts[destination] = sorts[column]
	}
	if r.formulaCheck == nil {
		return renamed, resultSorts, nil
	}
	formula, err := source.ToFormula(r.manager, sorts)
	if err != nil {
		return nil, nil, err
	}
	defer r.manager.DelRef(&formula)
	handle, err := r.manager.RefGet(formula)
	if err != nil {
		return nil, nil, err
	}
	if err = rel.VerifyBoundRelationRename(r.manager, sorts, resultSorts, *r.formulaCheck, handle, cycle, renamed); err != nil {
		return nil, nil, err
	}
	return renamed, resultSorts, nil
}

func (r *boundDatalogRuntime) union(target, source *rel.BoundRelation, sorts []ast.ASTHandle, widen bool) (changed bool, resultErr error) {
	if r.formulaCheck == nil {
		if widen {
			return target.Widen(source)
		}
		return target.Union(source)
	}
	beforeFormula, err := target.ToFormula(r.manager, sorts)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&beforeFormula)) }()
	before, err := r.manager.RefGet(beforeFormula)
	if err != nil {
		return false, err
	}
	if widen {
		changed, err = target.Widen(source)
	} else {
		changed, err = target.Union(source)
	}
	if err != nil {
		return changed, err
	}
	return changed, errors.Join(err, rel.VerifyBoundRelationUnion(r.manager, sorts, *r.formulaCheck, before, source, target))
}

func (r *boundDatalogRuntime) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	r.evaluated = nil
	if err := r.discardSimilarity(); err != nil {
		return lbool.Undef, err
	}
	if err := r.saturatePositive(true); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, declaration := range declarations {
		if r.evaluated[declaration] == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrBoundDatalogFragment)
		}
	}
	for _, declaration := range declarations {
		if !r.evaluated[declaration].Empty() {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}

func (r *boundDatalogRuntime) relation(declaration ast.ASTHandle) (*rel.BoundRelation, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return nil, err
	}
	return r.evaluated[declaration].Clone(), nil
}

func (r *boundDatalogRuntime) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return r.evaluated[declaration].ToFormula(r.manager, r.entries[declaration].sorts)
}

func (r *boundDatalogRuntime) reset() error {
	if err := r.open(); err != nil {
		return err
	}
	r.evaluated = nil
	if err := r.discardSimilarity(); err != nil {
		return err
	}
	for _, entry := range r.entries {
		seed, err := r.newRelation(entry.sorts, false)
		if err != nil {
			return err
		}
		entry.seed = seed
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	if err := r.discardSimilarity(); err != nil {
		return err
	}
	result = errors.Join(result, r.retirePending())
	for i := len(r.order) - 1; i >= 0; i-- {
		entry := r.entries[r.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&entry.root))
		}
	}
	if result != nil {
		return result
	}
	r.closed = true
	r.manager, r.rules, r.entries, r.order, r.evaluated, r.pending = nil, nil, nil, nil, nil, nil
	return nil
}
