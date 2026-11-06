package fp

// Source: dl_compiler owns a rule-set view and obtains predicate signatures
// and requested relation kinds from the context. These are the concrete inputs
// needed by the external/interval instantiation before any relation callbacks.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// The rule set and AST handles are borrowed for compilation. Predicate
// metadata belongs to this compiler; generated filter/split declarations can
// therefore be described without registering them in the persistent engine.
type externalIntervalDatalogCompiler struct {
	manager        *ast.ASTManager
	set            *base.RuleSet
	predicates     map[ast.ASTHandle]*externalIntervalDatalogPredicate
	queryAuxiliary map[ast.ASTHandle]bool
}

func (e *PropositionalEngine) externalIntervalCompiler() *externalIntervalDatalogCompiler {
	c := &externalIntervalDatalogCompiler{
		manager:        e.manager,
		set:            e.set,
		predicates:     make(map[ast.ASTHandle]*externalIntervalDatalogPredicate, len(e.predicates)),
		queryAuxiliary: make(map[ast.ASTHandle]bool, len(e.auxiliary)),
	}
	for declaration, state := range e.predicates {
		if state != nil {
			c.predicates[declaration] = &externalIntervalDatalogPredicate{
				declaration: declaration,
				sorts:       append([]ast.ASTHandle(nil), state.sorts...),
				kinds:       append([]string(nil), state.requestedRepresentations...),
			}
		}
	}
	for declaration, auxiliary := range e.auxiliary {
		c.queryAuxiliary[declaration] = auxiliary
	}
	return c
}
