package fp

// Source: rel_context::scoped_query restores original predicates after the
// compiler's LOAD/STORE program. Helper products are local; original entries
// remain the STORE destinations owned by the relational context.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// newExternalIntervalDatalogScope borrows the original entries and owns only
// the generated helper entries and compiled program. The caller must keep the
// source and normalized rule owners alive and unchanged until scope.close
// succeeds. In particular, cleanup may finish a committed STORE operation.
func newExternalIntervalDatalogScope(source *externalIntervalDatalogActivation, helpers []externalIntervalDatalogPredicate, program *externalIntervalDatalogProgram) (*externalIntervalDatalogActivation, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	if len(helpers) == 0 || program == nil || program.registers != nil || program.next != 0 || program.storing || source.pending != nil || source.program != nil && source.program.registers != nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	known := make(map[ast.ASTHandle]bool, len(source.entries)+len(helpers))
	for _, declaration := range source.order {
		entry := source.entries[declaration]
		if entry == nil || entry.product == nil || entry.root == (ast.ASTRef{}) || known[declaration] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		root, err := source.manager.RefGet(entry.root)
		if err != nil {
			return nil, err
		}
		if root != declaration {
			return nil, ErrExternalIntervalDatalogFragment
		}
		if _, err = entry.product.Sorts(); err != nil {
			return nil, err
		}
		known[declaration] = true
	}
	for _, helper := range helpers {
		if known[helper.declaration] || !isExternalIntervalRepresentation(helper.kinds) || !rel.CanHandleIntervalSorts(source.manager, helper.sorts) {
			return nil, ErrExternalIntervalDatalogFragment
		}
		known[helper.declaration] = true
	}
	for _, declaration := range program.declarations {
		if !known[declaration] {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	// Construct only helper empties. The ordinary register LOAD supplies the
	// source-required clones of nonempty originals; no activation clone occurs.
	scope, err := newExternalIntervalDatalogActivation(source.manager, source.plugin, helpers)
	if err != nil {
		return nil, err
	}
	scope.borrowedEntries = make(map[ast.ASTHandle]*externalIntervalDatalogEntry, len(source.order))
	for _, declaration := range source.order {
		entry := source.entries[declaration]
		scope.borrowedEntries[declaration] = entry
		scope.entries[declaration] = entry
	}
	scope.order = append(append([]ast.ASTHandle(nil), source.order...), scope.order...)
	if err = scope.installExternalIntervalProgram(program); err != nil {
		return nil, finishExternalIntervalDatalogConstruction(err, scope)
	}
	return scope, nil
}
