package rel

// Source: src/muz/rel/dl_external_relation.cpp/.h and the fixedpoint callback
// fallback in src/api/api_datalog.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ExternalRelationCallbacks are the concrete fixedpoint reduction callbacks.
// The plugin keeps callback declarations, arguments, and reduction results
// alive until it closes. Returning a null handle requests the source fallback.
type ExternalRelationCallbacks struct {
	Reduce       func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error)
	ReduceAssign func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error
}

// ExternalRelationPlugin owns the shared callback configuration for relations
// created by one source external relation plugin.
type ExternalRelationPlugin struct {
	manager    *RelationManager
	ast        *ast.ASTManager
	callbacks  ExternalRelationCallbacks
	trail      []ast.ASTRef
	predicates map[ast.ASTHandle]*ExternalRelation
	users      int
	autoClose  bool
	closed     bool
}

func NewExternalRelationPlugin(manager *RelationManager, callbacks ExternalRelationCallbacks) (*ExternalRelationPlugin, error) {
	if manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	return &ExternalRelationPlugin{manager: manager, ast: manager.manager, callbacks: callbacks, predicates: make(map[ast.ASTHandle]*ExternalRelation)}, nil
}

// SetCallbacks replaces the context-level reduction callbacks used by every
// relation created through this plugin, matching fixedpoint_context setters.
func (plugin *ExternalRelationPlugin) SetCallbacks(callbacks ExternalRelationCallbacks) error {
	if plugin == nil || plugin.closed || plugin.manager == nil || plugin.manager.open() != nil {
		return ErrTableRelation
	}
	plugin.callbacks = callbacks
	return nil
}

// ExternalRelation owns its relation expression and cached accessor declarations.
// The relation manager and callback functions are borrowed.
type ExternalRelation struct {
	plugin                           *ExternalRelationPlugin
	manager                          *RelationManager
	ast                              *ast.ASTManager
	sorts                            []ast.ASTHandle
	relation                         ast.ASTRef
	selectDecl, storeDecl, emptyDecl ast.ASTRef
	closed                           bool
}

// NewExternalRelation creates a fresh external relation and invokes the empty
// relation assignment callback, if present.
func NewExternalRelation(manager *RelationManager, sorts []ast.ASTHandle, callbacks ExternalRelationCallbacks) (_ *ExternalRelation, resultErr error) {
	plugin, err := NewExternalRelationPlugin(manager, callbacks)
	if err != nil {
		return nil, err
	}
	plugin.autoClose = true
	return plugin.NewEmpty(sorts)
}

// NewEmpty creates a relation belonging to this callback plugin.
func (plugin *ExternalRelationPlugin) NewEmpty(sorts []ast.ASTHandle) (result *ExternalRelation, resultErr error) {
	if plugin == nil || plugin.closed || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, ErrTableRelation
	}
	manager := plugin.manager
	m := manager.manager
	relationSort, err := m.MkDatalogRelationSort(sorts)
	if err != nil {
		return nil, err
	}
	sortRef, err := m.NewRef(relationSort)
	if err != nil {
		return nil, err
	}
	owner := &externalRelationConstructionOwner{manager: m, temps: []ast.ASTRef{sortRef}}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	expression, err := m.MkFreshConst("T", relationSort)
	if err != nil {
		return nil, err
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return nil, err
	}
	r := &ExternalRelation{plugin: plugin, manager: manager, ast: m, sorts: append([]ast.ASTHandle(nil), sorts...), relation: ref}
	plugin.users++
	owner.result = r
	empty, err := m.MkDatalogEmpty(relationSort)
	if err != nil {
		return nil, err
	}
	emptyRef, err := m.NewRef(empty)
	if err != nil {
		return nil, err
	}
	owner.temps = append(owner.temps, emptyRef)
	declaration, err := m.GetDecl(empty)
	if err != nil {
		return nil, err
	}
	if err = plugin.reduceAssign(declaration, nil, []ast.ASTHandle{expression}); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *ExternalRelation) open() error {
	if r == nil || r.closed || r.plugin == nil || r.plugin.closed || r.ast == nil || r.manager == nil || r.manager.open() != nil || r.relation == (ast.ASTRef{}) {
		return ErrTableRelation
	}
	return nil
}

// Relation returns the borrowed external relation expression.
func (r *ExternalRelation) Relation() (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return r.ast.RefGet(r.relation)
}

// ToFormula returns an owning reference to the external relation expression.
func (r *ExternalRelation) ToFormula() (ast.ASTRef, error) {
	expression, err := r.Relation()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return r.ast.NewRef(expression)
}

func (r *ExternalRelation) validateFact(values []ast.ASTHandle) error {
	if len(values) != len(r.sorts) {
		return ast.ErrASTDomain
	}
	for i, value := range values {
		sort, err := r.ast.GetSort(value)
		if err != nil {
			return err
		}
		if sort != r.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// accessor creates and retains the same per-relation declaration on first use.
func (r *ExternalRelation) accessor(cache *ast.ASTRef, op int32, args []ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if *cache != (ast.ASTRef{}) {
		return r.ast.RefGet(*cache)
	}
	var app ast.ASTHandle
	var err error
	switch op {
	case ast.DatalogOpStore:
		app, err = r.ast.MkDatalogStore(args[0], args[1:]...)
	case ast.DatalogOpSelect:
		app, err = r.ast.MkDatalogSelect(args[0], args[1:]...)
	case ast.DatalogOpIsEmpty:
		app, err = r.ast.MkDatalogIsEmpty(args[0])
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&appRef)) }()
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*cache, err = r.ast.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return declaration, nil
}

func (r *ExternalRelation) reduce(declaration ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	return r.plugin.reduce(declaration, args)
}

func (r *ExternalRelation) AddFact(values []ast.ASTHandle) error {
	expression, err := r.Relation()
	if err != nil {
		return err
	}
	if err = r.validateFact(values); err != nil {
		return err
	}
	args := append([]ast.ASTHandle{expression}, values...)
	declaration, err := r.accessor(&r.storeDecl, ast.DatalogOpStore, args)
	if err != nil {
		return err
	}
	return r.plugin.reduceAssign(declaration, args, args[:1])
}

func (r *ExternalRelation) ContainsFact(values []ast.ASTHandle) (found bool, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return false, err
	}
	if err = r.validateFact(values); err != nil {
		return false, err
	}
	args := append([]ast.ASTHandle{expression}, values...)
	declaration, err := r.accessor(&r.selectDecl, ast.DatalogOpSelect, args)
	if err != nil {
		return false, err
	}
	result, err := r.reduce(declaration, args)
	if err != nil {
		return false, err
	}
	ref, err := r.ast.NewRef(result)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	return !r.ast.IsFalse(result), nil
}

func (r *ExternalRelation) Empty() (empty bool, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return false, err
	}
	args := []ast.ASTHandle{expression}
	declaration, err := r.accessor(&r.emptyDecl, ast.DatalogOpIsEmpty, args)
	if err != nil {
		return false, err
	}
	result, err := r.reduce(declaration, args)
	if err != nil {
		return false, err
	}
	ref, err := r.ast.NewRef(result)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	return r.ast.IsTrue(result), nil
}

// Clone invokes the source clone assignment with a fresh relation expression.
func (r *ExternalRelation) Clone() (result *ExternalRelation, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	sort, err := r.ast.GetSort(expression)
	if err != nil {
		return nil, err
	}
	fresh, err := r.ast.MkFreshConst("T", sort)
	if err != nil {
		return nil, err
	}
	freshRef, err := r.ast.NewRef(fresh)
	if err != nil {
		return nil, err
	}
	result = &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), relation: freshRef}
	r.plugin.users++
	owner := &externalRelationConstructionOwner{manager: r.ast, result: result}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	app, err := r.ast.MkDatalogClone(expression)
	if err != nil {
		return nil, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	owner.temps = append(owner.temps, appRef)
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	if err = r.plugin.reduceAssign(declaration, []ast.ASTHandle{expression}, []ast.ASTHandle{fresh}); err != nil {
		return nil, err
	}
	return result, nil
}

// Complement invokes the source non-destructive complement reduction.
func (r *ExternalRelation) Complement() (result *ExternalRelation, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	app, err := r.ast.MkDatalogComplement(expression)
	if err != nil {
		return nil, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	owner := &externalRelationConstructionOwner{manager: r.ast, temps: []ast.ASTRef{appRef}}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	value, err := r.reduce(declaration, []ast.ASTHandle{expression})
	if err != nil {
		return nil, err
	}
	ref, err := r.ast.NewRef(value)
	if err != nil {
		return nil, err
	}
	result = &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), relation: ref}
	r.plugin.users++
	owner.result = result
	return result, nil
}

func (r *ExternalRelation) Close() (result error) {
	if r == nil || r.closed && r.plugin == nil {
		return nil
	}
	if !r.closed {
		for _, ref := range []*ast.ASTRef{&r.emptyDecl, &r.storeDecl, &r.selectDecl, &r.relation} {
			if *ref != (ast.ASTRef{}) {
				result = errors.Join(result, r.ast.DelRef(ref))
			}
		}
		if r.emptyDecl != (ast.ASTRef{}) || r.storeDecl != (ast.ASTRef{}) || r.selectDecl != (ast.ASTRef{}) || r.relation != (ast.ASTRef{}) {
			return result
		}
		r.closed = true
		r.plugin.users--
	}
	if r.plugin.autoClose && r.plugin.users == 0 {
		if err := r.plugin.Close(); err != nil {
			return errors.Join(result, err)
		}
	}
	r.plugin, r.manager, r.ast, r.sorts = nil, nil, nil, nil
	return result
}
