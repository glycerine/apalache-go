package spacer

// Source: spacer_context.cpp context::check_invariant and expand_node's
// post-lemma CASSERT at line 2978.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) CheckPredicateInvariant(level uint32, declaration ast.ASTHandle) (valid bool, resultErr error) {
	if err := c.open(); err != nil {
		return false, err
	}
	p := c.transformers[declaration]
	if p == nil {
		return false, ast.ErrASTDomain
	}
	next := level
	if next != InfinityLevel {
		next++
	}
	invariant, err := p.Formulas(next, false)
	if err != nil {
		return false, err
	}
	attempt := &invariantCheckAttempt{manager: c.manager, references: []ast.ASTRef{invariant}}
	defer func() {
		resultErr = attempt.finish(resultErr)
		if resultErr != nil {
			valid = false
		}
	}()
	body, err := c.manager.RefGet(invariant)
	if err != nil {
		return false, err
	}
	if c.manager.IsTrue(body) {
		return true, nil
	}
	premises, err := p.Premises(level)
	if err != nil {
		return false, err
	}
	attempt.references = append(attempt.references, premises...)
	formulas := make([]ast.ASTHandle, 0, len(premises)+1)
	for _, ref := range premises {
		h, err := c.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
		formulas = append(formulas, h)
	}
	negated, err := c.manager.MkNot(body)
	if err != nil {
		return false, err
	}
	formulas = append(formulas, negated)
	formula, err := c.manager.MkAndArray(formulas)
	if err != nil {
		return false, err
	}
	root, err := c.manager.NewRef(formula)
	if err != nil {
		return false, err
	}
	attempt.references = append(attempt.references, root)
	contexts := c.owner.contexts[1]
	solver, err := smtcore.NewSMTSolver(c.manager, contexts.params, "", contexts.config)
	if err != nil {
		return false, err
	}
	attempt.solver = solver
	if err = solver.Assert(formula); err != nil {
		return false, err
	}
	status, err := solver.CheckSat(nil)
	return status == lbool.False, err
}

func (c *Context) CheckInvariants(level uint32) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	for _, declaration := range c.order {
		if err := spacerCheckpoint(c.manager); err != nil {
			return false, err
		}
		valid, err := c.CheckPredicateInvariant(level, declaration)
		if err != nil || !valid {
			return valid, err
		}
	}
	return true, nil
}

func (p *PredicateTransformer) assertExpansionInvariants(obligation *ProofObligation) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := debug.ConditionalAssertionEnabled("spacer")
	if err != nil || !enabled {
		return err
	}
	valid, err := p.expansionInvariantsHold(obligation)
	if err != nil {
		return err
	}
	if valid {
		return nil
	}
	return diagnostic.DebugAssertionFailure(debug, "src/muz/spacer/spacer_context.cpp", 2978, "n.level() == 0 || check_invariant(n.level()-1)")
}

// Kept separate so the source's level-zero short circuit can be verified
// without enabling debug builds or executing the process assertion handler.
func (p *PredicateTransformer) expansionInvariantsHold(obligation *ProofObligation) (bool, error) {
	level, err := obligation.Level()
	if err != nil {
		return false, err
	}
	if level == 0 {
		return true, nil
	}
	if p.context == nil {
		return true, nil
	} // A standalone transformer has no context-wide assertion scope.
	return p.context.CheckInvariants(level - 1)
}
