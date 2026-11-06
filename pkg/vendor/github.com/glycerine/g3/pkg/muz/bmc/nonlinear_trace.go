package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::get_proof.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// NonlinearTrace follows the model-selected BMC rules in source preorder.
// The caller owns each returned rule reference.
func (c *Compiler) NonlinearTrace(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []*base.Rule, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil {
			resultErr = disposeRuleOwners(c.manager, result, resultErr)
			result = nil
		}
	}()
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	queryRef, err := c.CompileQuery(query, level)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = disposeASTRefs(c.manager, []ast.ASTRef{queryRef}, resultErr) }()
	application, err := c.manager.RefGet(queryRef)
	if err != nil {
		return nil, err
	}
	if err = c.traceNonlinearRule(rules, evaluator, query, application, level, &result, nil); err != nil {
		return result, err
	}
	return result, nil
}

// finishNonlinearSelector keeps the selector declaration, application and
// evaluated value alive until nested evaluator cleanup has completed. A match
// is not published while any of these temporary owners still needs cleanup.
func finishNonlinearSelector(m *ast.ASTManager, refs []ast.ASTRef, matched bool, cause error) (bool, error) {
	if err := disposeASTRefs(m, refs, cause); err != nil {
		return false, err
	}
	return matched, nil
}

func (c *Compiler) nonlinearRuleSelected(evaluator *model.ModelEvaluator, predicate ast.ASTHandle, args []ast.ASTHandle, index, level uint32) (matched bool, resultErr error) {
	m := c.manager
	var refs []ast.ASTRef
	defer func() { matched, resultErr = finishNonlinearSelector(m, refs, matched, resultErr) }()
	declaration, err := c.levelDeclaration(predicate, level, &index)
	if err != nil {
		return false, err
	}
	refs = append(refs, declaration)
	decl, err := m.RefGet(declaration)
	if err != nil {
		return false, err
	}
	probe, err := m.MkApp(decl, args)
	if err != nil {
		return false, err
	}
	probeRef, err := m.NewRef(probe)
	if err != nil {
		return false, err
	}
	refs = append(refs, probeRef)
	valueRef, err := evaluator.EvaluateValue(probe)
	if err != nil {
		return false, err
	}
	refs = append(refs, valueRef)
	value, err := m.RefGet(valueRef)
	return err == nil && m.IsTrue(value), err
}

func (c *Compiler) traceNonlinearRule(rules *base.RuleSet, evaluator *model.ModelEvaluator, predicate, application ast.ASTHandle, level uint32, trace *[]*base.Rule, proofResult *ast.ASTRef) (resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil && proofResult != nil && *proofResult != (ast.ASTRef{}) {
			// Transfer the output token into the retry receipt before clearing
			// the caller-visible slot. A native refusal must not discard the
			// recursive frame's sole proof owner.
			resultErr = disposeASTRefs(m, []ast.ASTRef{*proofResult}, resultErr)
			*proofResult = ast.ASTRef{}
		}
	}()
	canceled, err := m.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
	}
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = disposeRuleOwners(m, group, resultErr)
	}()
	count, err := m.GetNumArgs(application)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, count)
	argSorts := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = m.GetArg(application, uint32(i))
		if err != nil {
			return err
		}
		argSorts[i], err = m.GetSort(args[i])
		if err != nil {
			return err
		}
	}
	var selected *base.Rule
	for i, rule := range group {
		matched, err := c.nonlinearRuleSelected(evaluator, predicate, args, uint32(i), level)
		if err != nil {
			return err
		}
		if matched {
			selected = rule
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("bmc: model has no selected rule at level %d", level)
	}
	owned, err := selected.Retain()
	if err != nil {
		return err
	}
	*trace = append(*trace, owned)
	tails, err := selected.UninterpretedTailSize()
	if err != nil {
		return err
	}
	if tails == 0 && proofResult == nil {
		return nil
	}
	if tails > 0 && level == 0 {
		return ast.ErrASTDomain
	}
	variableSorts, err := selected.VariableSorts()
	if err != nil {
		return err
	}
	var premises []ast.ASTHandle
	if proofResult != nil {
		proof, found, err := selected.Proof()
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("bmc: nonlinear rule has no proof")
		}
		if len(variableSorts) == 0 && tails == 0 {
			*proofResult, err = m.NewRef(proof)
			return err
		}
		premises = append(premises, proof)
	}
	name, err := m.GetASTName(predicate)
	if err != nil {
		return err
	}
	text, err := m.Symbols.Display(name)
	if err != nil {
		return err
	}
	var refs []ast.ASTRef
	defer func() {
		// nonlinear::get_proof owns model-evaluated bindings, substituted
		// children, and its conclusion until this recursive frame retires.
		// Preserve the exact reverse frontier when native cleanup refuses.
		resultErr = disposeASTRefs(m, refs, resultErr)
	}()
	bindings := make([]ast.ASTHandle, len(variableSorts))
	// The source proof path normally sees transformed rules. When compiling a
	// raw rule, variables occurring directly in its head are not Skolemized:
	// recover them from the corresponding model-evaluated head arguments.
	head, err := selected.Head()
	if err != nil {
		return err
	}
	for i := range args {
		headArg, err := m.GetArg(head, uint32(i))
		if err != nil {
			return err
		}
		kind, err := m.GetKind(headArg)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			continue
		}
		index, err := m.GetVarIndex(headArg)
		if err != nil || int(index) >= len(bindings) {
			return errors.Join(ast.ErrASTDomain, err)
		}
		if !bindings[index].IsNull() {
			continue
		}
		valueRef, err := evaluator.EvaluateValue(args[i])
		if err != nil {
			return err
		}
		refs = append(refs, valueRef)
		bindings[index], err = m.RefGet(valueRef)
		if err != nil {
			return err
		}
	}
	for i, sort := range variableSorts {
		if sort.IsNull() || !bindings[i].IsNull() {
			continue
		}
		sym, err := m.Symbols.Intern(fmt.Sprintf("%s@%d", text, i))
		if err != nil {
			return err
		}
		decl, err := m.MkFuncDecl(sym, argSorts, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return err
		}
		term, err := m.MkApp(decl, args)
		if err != nil {
			return err
		}
		termRef, err := m.NewRef(term)
		if err != nil {
			return err
		}
		refs = append(refs, termRef)
		valueRef, err := evaluator.EvaluateValue(term)
		if err != nil {
			return err
		}
		refs = append(refs, valueRef)
		bindings[i], err = m.RefGet(valueRef)
		if err != nil {
			return err
		}
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	positions := make([]ast.ASTProofPosition, 0, tails)
	substitutions := make([][]ast.ASTHandle, 1, tails+1)
	substitutions[0] = bindings
	for i := 0; i < tails; i++ {
		tail, _, err := selected.Tail(i)
		if err != nil {
			return err
		}
		decl, err := selected.TailDeclaration(i)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		if err = sub.Substitute(tail, bindings, ref); err != nil {
			return err
		}
		body, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		var child ast.ASTRef
		var childResult *ast.ASTRef
		if proofResult != nil {
			childResult = &child
		}
		if err = c.traceNonlinearRule(rules, evaluator, decl, body, level-1, trace, childResult); err != nil {
			return err
		}
		if proofResult != nil {
			refs = append(refs, child)
			childProof, err := m.RefGet(child)
			if err != nil {
				return err
			}
			premises = append(premises, childProof)
			positions = append(positions, ast.ASTProofPosition{First: uint32(i + 1)})
			substitutions = append(substitutions, nil)
		}
	}
	if proofResult != nil {
		// The search uses a level-indexed predicate at the root. Proofs must
		// conclude the original predicate at the model-evaluated arguments.
		values := make([]ast.ASTHandle, len(args))
		for i, argument := range args {
			value, e := evaluator.EvaluateValue(argument)
			if e != nil {
				return e
			}
			refs = append(refs, value)
			values[i], e = m.RefGet(value)
			if e != nil {
				return e
			}
		}
		conclusion, err := m.MkApp(predicate, values)
		if err != nil {
			return err
		}
		conclusionRef, err := m.NewRef(conclusion)
		if err != nil {
			return err
		}
		refs = append(refs, conclusionRef)
		proof, err := m.MkProofHyperResolve(premises, conclusion, positions, substitutions)
		if err != nil {
			return err
		}
		*proofResult, err = m.NewRef(proof)
		return err
	}
	return nil
}
