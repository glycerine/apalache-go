package fp

// Source: rel_context::set_predicate_representation and the zero-rule part of
// rel_context::query, specialized to the ordered external/interval product.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

var ErrExternalIntervalDatalogFragment = errors.New("muz/fp: unsupported external/interval product Datalog fragment")

func isExternalIntervalRepresentation(kinds []string) bool {
	return len(kinds) == 2 && kinds[0] != kinds[1] &&
		(kinds[0] == "external_relation" && kinds[1] == "interval_relation" ||
			kinds[0] == "interval_relation" && kinds[1] == "external_relation")
}

type externalIntervalDatalogPredicate struct {
	declaration ast.ASTHandle
	sorts       []ast.ASTHandle
	kinds       []string
}

type externalIntervalDatalogEntry struct {
	sorts   []ast.ASTHandle
	kinds   []string
	product *rel.ExternalIntervalProduct
	root    ast.ASTRef
}

// externalIntervalDatalogActivation owns the concrete product relations for
// the retained specialized program. The callback plugin is borrowed from the
// fixedpoint context and must outlive this owner.
type externalIntervalDatalogActivation struct {
	manager              *ast.ASTManager
	plugin               *rel.ExternalRelationPlugin
	entries              map[ast.ASTHandle]*externalIntervalDatalogEntry
	order                []ast.ASTHandle
	borrowedEntries      map[ast.ASTHandle]*externalIntervalDatalogEntry
	identity             *externalIntervalDatalogIdentityRule
	ground               *externalIntervalDatalogGroundRule
	cartesian            *externalIntervalDatalogCartesianRule
	program              *externalIntervalDatalogProgram
	pending              memory.ConstructionFailure
	pendingCompletesRule bool
	pendingIdentity      *externalIntervalDatalogIdentityRule
	pendingGround        *externalIntervalDatalogGroundRule
	pendingCartesian     *externalIntervalDatalogCartesianRule
	closed               bool
}

type externalIntervalDatalogConstructionFailure struct {
	original, cleanup, reported error
	text                        string
	owner                       *externalIntervalDatalogConstructionOwner
}

type externalIntervalDatalogConstructionOwner struct {
	nested     memory.ConstructionFailure
	activation *externalIntervalDatalogActivation
}

func (o *externalIntervalDatalogConstructionOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	return o.activation.close()
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogConstructionFailure)(nil)

func (f *externalIntervalDatalogConstructionFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogConstructionFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogConstructionFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.activation.manager.NumeralService().RuntimeOwner()
}
func (f *externalIntervalDatalogConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalIntervalDatalogConstruction(cause error, activation *externalIntervalDatalogActivation) error {
	if activation == nil {
		return cause
	}
	owner := &externalIntervalDatalogConstructionOwner{activation: activation}
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogConstructionFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func newExternalIntervalDatalogActivation(manager *ast.ASTManager, plugin *rel.ExternalRelationPlugin, predicates []externalIntervalDatalogPredicate) (_ *externalIntervalDatalogActivation, resultErr error) {
	if manager == nil || plugin == nil || len(predicates) == 0 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	a := &externalIntervalDatalogActivation{
		manager: manager,
		plugin:  plugin,
		entries: make(map[ast.ASTHandle]*externalIntervalDatalogEntry, len(predicates)),
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishExternalIntervalDatalogConstruction(resultErr, a)
		}
	}()
	for _, predicate := range predicates {
		if a.entries[predicate.declaration] != nil || !isExternalIntervalRepresentation(predicate.kinds) || !rel.CanHandleIntervalSorts(manager, predicate.sorts) {
			return nil, ErrExternalIntervalDatalogFragment
		}
		product, selected, err := rel.NewSelectedExternalIntervalProduct(plugin, predicate.sorts, predicate.kinds)
		if err != nil || !selected {
			return nil, errors.Join(err, ErrExternalIntervalDatalogFragment)
		}
		entry := &externalIntervalDatalogEntry{
			sorts:   append([]ast.ASTHandle(nil), predicate.sorts...),
			kinds:   append([]string(nil), predicate.kinds...),
			product: product,
		}
		a.entries[predicate.declaration] = entry
		a.order = append(a.order, predicate.declaration)
		entry.root, err = manager.NewRef(predicate.declaration)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (a *externalIntervalDatalogActivation) open() error {
	if a == nil || a.closed || a.manager == nil || a.plugin == nil || a.entries == nil {
		return ErrExternalIntervalDatalogFragment
	}
	return nil
}

func externalIntervalKindsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (a *externalIntervalDatalogActivation) allowsSelection(declaration ast.ASTHandle, kinds []string) bool {
	if a == nil || a.closed {
		return false
	}
	entry := a.entries[declaration]
	return entry == nil || externalIntervalKindsEqual(entry.kinds, kinds)
}

func (a *externalIntervalDatalogActivation) contains(declaration ast.ASTHandle) bool {
	return a != nil && !a.closed && a.entries[declaration] != nil
}

// syncPredicates adds newly registered product predicates without replacing
// any relation that may already contain direct facts.
func (a *externalIntervalDatalogActivation) syncPredicates(predicates []externalIntervalDatalogPredicate) (resultErr error) {
	if err := a.open(); err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]bool, len(predicates))
	additions := make([]externalIntervalDatalogPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		if seen[predicate.declaration] {
			return ErrExternalIntervalDatalogFragment
		}
		seen[predicate.declaration] = true
		entry := a.entries[predicate.declaration]
		if entry == nil {
			additions = append(additions, predicate)
			continue
		}
		if !externalIntervalKindsEqual(entry.kinds, predicate.kinds) {
			return ErrExternalIntervalDatalogFragment
		}
	}
	for declaration := range a.entries {
		if !seen[declaration] {
			return ErrExternalIntervalDatalogFragment
		}
	}
	if len(additions) == 0 {
		return nil
	}
	staged, err := newExternalIntervalDatalogActivation(a.manager, a.plugin, additions)
	if err != nil {
		return err
	}
	defer func() {
		if staged != nil {
			resultErr = errors.Join(resultErr, staged.close())
		}
	}()
	for _, declaration := range staged.order {
		a.entries[declaration] = staged.entries[declaration]
		a.order = append(a.order, declaration)
		delete(staged.entries, declaration)
	}
	staged.order = nil
	return nil
}

func (a *externalIntervalDatalogActivation) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	if _, err := a.resumeExternalIntervalIdentity(); err != nil {
		return err
	}
	if err := a.discardExternalIntervalProgramExecution(); err != nil {
		return err
	}
	entry := a.entries[declaration]
	if entry == nil || len(values) != len(entry.sorts) {
		return ErrExternalIntervalDatalogFragment
	}
	return entry.product.AddFact(values)
}

// query implements the specialized product nonemptiness contract. A nonempty
// product is imprecise because of its interval component, so it is unknown,
// never true. Formula export is deliberately not attempted.
func (a *externalIntervalDatalogActivation) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	nonempty := false
	for _, declaration := range declarations {
		entry := a.entries[declaration]
		if entry == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared product query predicate", ErrExternalIntervalDatalogFragment)
		}
		empty, err := entry.product.Empty()
		if err != nil {
			return lbool.Undef, err
		}
		nonempty = nonempty || !empty
	}
	if nonempty {
		return lbool.Undef, nil
	}
	return lbool.False, nil
}

func (a *externalIntervalDatalogActivation) close() error {
	if a == nil || a.closed {
		return nil
	}
	for declaration, entry := range a.borrowedEntries {
		if entry == nil || a.entries[declaration] != entry {
			return ErrExternalIntervalDatalogFragment
		}
	}
	if a.pending != nil {
		if err := a.pending.Cleanup(); err != nil {
			return err
		}
		a.pending = nil
		a.pendingCompletesRule = false
		a.pendingIdentity = nil
		a.pendingGround = nil
		a.pendingCartesian = nil
	}
	// Products own external-plugin users, so every product must retire before
	// the borrowed plugin can be closed by the fixedpoint engine.
	if err := a.discardExternalIntervalProgramExecution(); err != nil {
		return err
	}
	for i := len(a.order) - 1; i >= 0; i-- {
		if _, borrowed := a.borrowedEntries[a.order[i]]; borrowed {
			continue
		}
		entry := a.entries[a.order[i]]
		if entry != nil && entry.product != nil {
			if err := entry.product.Close(); err != nil {
				return err
			}
			entry.product = nil
		}
	}
	if err := a.closeExternalIntervalProgramPlans(); err != nil {
		return err
	}
	if a.program == nil {
		if err := a.identity.close(a.manager); err != nil {
			return err
		}
		if err := a.cartesian.close(a.manager); err != nil {
			return err
		}
	}
	if a.program == nil && a.ground != nil {
		if err := a.ground.close(a.manager); err != nil {
			return err
		}
	}
	for i := len(a.order) - 1; i >= 0; i-- {
		if _, borrowed := a.borrowedEntries[a.order[i]]; borrowed {
			continue
		}
		entry := a.entries[a.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&entry.root); err != nil {
				return err
			}
		}
	}
	a.closed = true
	a.plugin = nil
	a.entries = nil
	a.order = nil
	a.borrowedEntries = nil
	a.identity = nil
	a.ground = nil
	a.cartesian = nil
	a.program = nil
	return nil
}
