package pdr

// Source: model_search::get_trace in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	astsubst "github.com/glycerine/g3/pkg/ast/substitution"
)

type traceObligation struct {
	node      *ModelNode
	predicate ast.ASTHandle
}

func (c *Context) traceMaximumVariable(expressions ...[]ast.ASTHandle) (uint32, error) {
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return 0, err
	}
	for _, group := range expressions {
		for _, expression := range group {
			if err = used.Process(expression); err != nil {
				return 0, err
			}
		}
	}
	return used.MaxFoundVarIndexPlusOne(), nil
}

func (c *Context) unifiedTraceConstraints(root *ModelNode, complete bool) (result []ast.ASTRef, resultErr error) {
	phase := "update models"
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("pdr: trace %s: %w", phase, resultErr)
		}
	}()
	if err := updateTraceModels(root); err != nil {
		return nil, fmt.Errorf("pdr: update trace models: %w", err)
	}
	owned := make([]ast.ASTRef, 0)
	defer func() {
		if x := closeASTRefs(c.manager, owned); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace temporaries: %w", x))
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(c.manager, result))
			result = nil
		}
	}()
	hold := func(reference ast.ASTRef, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		handle, err := c.manager.RefGet(reference)
		if err != nil {
			return ast.ASTHandle{}, errors.Join(err, c.manager.DelRef(&reference))
		}
		owned = append(owned, reference)
		return handle, nil
	}
	rootRule, err := c.selectedTraceRule(root)
	if err != nil {
		return nil, fmt.Errorf("pdr: select root trace rule: %w", err)
	}
	head, err := rootRule.rule.Head()
	if err != nil {
		return nil, err
	}
	work := []traceObligation{{node: root, predicate: head}}
	constraints := make([]ast.ASTHandle, 0)
	var anchor *PredicateTransformer
	if !complete {
		anchor, err = traceRecursiveAnchor(root)
		if err != nil {
			return nil, err
		}
	}
	theory, err := rewriter.NewTheoryRewriter(c.manager, c.owner.contexts.config.Params, c.owner.contexts.config.Globals, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := theory.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace rewriter: %w", x))
		}
	}()
	unifier, err := astsubst.NewUnifier(c.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := unifier.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace unifier: %w", x))
		}
	}()
	iteration := 0
	for len(work) != 0 {
		phase = fmt.Sprintf("select rule %d", iteration)
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		rule, x := c.selectedTraceRule(current.node)
		if x != nil {
			return nil, fmt.Errorf("pdr: select trace rule: %w", x)
		}
		rawHead, x := rule.rule.Head()
		if x != nil {
			return nil, x
		}
		phase = fmt.Sprintf("instantiate rule %d", iteration)
		instantiatedHead, x := hold(c.instantiateTraceExpression(current.node, rule, rawHead))
		if x != nil {
			return nil, fmt.Errorf("pdr: instantiate trace head: %w", x)
		}
		tailSize, x := rule.rule.TailSize()
		if x != nil {
			return nil, x
		}
		uninterpreted, x := rule.rule.UninterpretedTailSize()
		if x != nil {
			return nil, x
		}
		tails := make([]ast.ASTHandle, tailSize)
		for index := 0; index < tailSize; index++ {
			tail, _, y := rule.rule.Tail(index)
			if y != nil {
				return nil, y
			}
			tails[index], y = hold(c.instantiateTraceExpression(current.node, rule, tail))
			if y != nil {
				return nil, fmt.Errorf("pdr: instantiate trace tail %d: %w", index, y)
			}
		}
		pending := make([]ast.ASTHandle, len(work))
		for i := range work {
			pending[i] = work[i].predicate
		}
		phase = fmt.Sprintf("measure variables %d", iteration)
		maximum, x := c.traceMaximumVariable(constraints, pending, []ast.ASTHandle{current.predicate, instantiatedHead}, tails)
		if x != nil {
			return nil, x
		}
		substitution, x := astsubst.New(c.manager)
		if x != nil {
			return nil, x
		}
		if x = substitution.Reserve(2, maximum); x != nil {
			_ = substitution.Close()
			return nil, x
		}
		phase = fmt.Sprintf("unify predicates %d", iteration)
		unified, x := unifier.UnifyPair(current.predicate, instantiatedHead, substitution, true)
		if x != nil || !unified {
			_ = substitution.Close()
			if x != nil {
				return nil, x
			}
			return nil, fmt.Errorf("pdr: trace predicate does not unify: %w", ast.ErrASTDomain)
		}
		deltas := []uint32{0, maximum}
		apply := func(expression ast.ASTHandle, offset uint32, rewrite bool) (ast.ASTHandle, error) {
			applied, y := c.manager.NewRef(ast.ASTHandle{})
			if y != nil {
				return ast.ASTHandle{}, y
			}
			if y = substitution.Apply(2, deltas, astsubst.NewExprOffset(expression, offset), applied); y != nil {
				return ast.ASTHandle{}, errors.Join(y, c.manager.DelRef(&applied))
			}
			handle, y := hold(applied, nil)
			if y != nil || !rewrite {
				return handle, y
			}
			rewritten, y := c.manager.NewRef(ast.ASTHandle{})
			if y != nil {
				return ast.ASTHandle{}, y
			}
			if y = theory.Rewrite(handle, rewritten); y != nil {
				return ast.ASTHandle{}, errors.Join(y, c.manager.DelRef(&rewritten))
			}
			return hold(rewritten, nil)
		}
		phase = "apply unifier"
		for i := range constraints {
			constraints[i], x = apply(constraints[i], 0, true)
			if x != nil {
				_ = substitution.Close()
				return nil, x
			}
		}
		for i := range work {
			work[i].predicate, x = apply(work[i].predicate, 0, false)
			if x != nil {
				_ = substitution.Close()
				return nil, x
			}
		}
		predicate, x := apply(current.predicate, 0, false)
		if x != nil {
			_ = substitution.Close()
			return nil, x
		}
		if complete || iteration != 0 && (anchor == nil || current.node.transformer == anchor) {
			constraints = append(constraints, predicate)
		}
		appliedTails := make([]ast.ASTHandle, len(tails))
		for i := range tails {
			appliedTails[i], x = apply(tails[i], 1, false)
			if x != nil {
				_ = substitution.Close()
				return nil, x
			}
			if i >= uninterpreted {
				constraints = append(constraints, appliedTails[i])
			}
		}
		if x = substitution.Close(); x != nil {
			return nil, x
		}
		children, x := current.node.Children()
		if x != nil {
			return nil, x
		}
		if len(children) == 0 {
			continue
		}
		if len(children) != uninterpreted {
			return nil, fmt.Errorf("pdr: trace has %d children for %d uninterpreted tails: %w", len(children), uninterpreted, ast.ErrASTDomain)
		}
		for i := len(children) - 1; i >= 0; i-- {
			work = append(work, traceObligation{node: children[i], predicate: appliedTails[i]})
		}
		iteration++
	}

	phase = "build ground replacement"
	replacement, err := c.manager.NewExprSubstitutionPolicy(false, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := c.manager.DelExprSubstitution(&replacement); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: delete trace replacement: %w", x))
		}
	}()
	for _, constraint := range constraints {
		if kind, x := c.manager.GetKind(constraint); x != nil {
			return nil, x
		} else if kind != ast.ASTApp {
			continue
		}
		family, x := c.manager.GetFamilyID(constraint)
		if x != nil {
			return nil, x
		}
		operation, x := c.manager.GetDeclKind(constraint)
		if x != nil {
			return nil, x
		}
		if family != c.manager.BasicFamilyID() || operation != ast.ASTOpEq {
			continue
		}
		left, x := c.manager.GetArg(constraint, 0)
		if x != nil {
			return nil, x
		}
		right, x := c.manager.GetArg(constraint, 1)
		if x != nil {
			return nil, x
		}
		leftKind, x := c.manager.GetKind(left)
		if x != nil {
			return nil, x
		}
		rightGround, x := c.manager.IsGround(right)
		if x != nil {
			return nil, x
		}
		if leftKind == ast.ASTVar && rightGround {
			if x = c.manager.ExprSubstitutionInsert(replacement, left, right, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
				return nil, x
			}
			continue
		}
		rightKind, x := c.manager.GetKind(right)
		if x != nil {
			return nil, x
		}
		leftGround, x := c.manager.IsGround(left)
		if x != nil {
			return nil, x
		}
		if rightKind == ast.ASTVar && leftGround {
			if x = c.manager.ExprSubstitutionInsert(replacement, right, left, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
				return nil, x
			}
		}
	}
	phase = "create trace replacer"
	replacer, err := rewriter.NewDefaultExprReplacer(c.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := replacer.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace replacer: %w", x))
		}
	}()
	if err = replacer.SetSubstitution(&replacement); err != nil {
		return nil, err
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	phase = "finalize constraints"
	for _, constraint := range constraints {
		replaced, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return nil, x
		}
		if x = replacer.Rewrite(constraint, replaced); x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&replaced))
		}
		replacedHandle, x := hold(replaced, nil)
		if x != nil {
			return nil, x
		}
		rewritten, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return nil, x
		}
		if x = theory.Rewrite(replacedHandle, rewritten); x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&rewritten))
		}
		handle, x := c.manager.RefGet(rewritten)
		if x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&rewritten))
		}
		if handle == truth {
			if x = c.manager.DelRef(&rewritten); x != nil {
				return nil, x
			}
			continue
		}
		result = append(result, rewritten)
	}
	phase = "cleanup"
	return result, nil
}
