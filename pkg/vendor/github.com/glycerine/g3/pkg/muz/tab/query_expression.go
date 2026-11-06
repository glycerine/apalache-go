package tab

// Source: src/muz/tab/tab_context.cpp imp::query and
// src/muz/base/dl_rule.cpp rule_manager::mk_query.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// QueryExpression runs tabulation for an arbitrary Boolean Horn expression.
// The synthetic query rule is query-owned; the supplied program is unchanged.
func QueryExpression(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, resultErr error) {
	return queryExpression(rules, query, rw, lite, querySolver, subsumptionSolver, strategy, nil, nil, evaluator...)
}

// QueryExpressionWithSession preserves the source TAB session state while
// keeping the synthetic query rule scoped to this call.
func QueryExpressionWithSession(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, resultErr error) {
	return QueryExpressionWithSessionStatistics(rules, query, rw, lite, querySolver, subsumptionSolver, session, nil, evaluator...)
}

// QueryExpressionWithSessionStatistics also accumulates this query's source
// search counters, without indexing an additional synthetic query rule.
func QueryExpressionWithSessionStatistics(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, resultErr error) {
	return queryExpression(rules, query, rw, lite, querySolver, subsumptionSolver, "", session, stats, evaluator...)
}

func queryExpression(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, resultErr error) {
	if rules == nil || rw == nil || rw.Manager() == nil {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	m := rw.Manager()
	attempt := &tabExpressionAttempt{manager: m}
	defer func() { answer, resultErr = attempt.finish(answer, resultErr) }()
	boolean, err := m.MkBoolSort()
	if err != nil {
		return lbool.Undef, answer, err
	}
	sort, err := m.GetSort(query)
	if err != nil {
		return lbool.Undef, answer, err
	}
	if sort != boolean {
		return lbool.Undef, answer, fmt.Errorf("muz/tab: query expression is not Boolean")
	}
	ruleManager, err := rules.RuleManager()
	if err != nil {
		return lbool.Undef, answer, err
	}
	compacted, err := compactExpressionVariables(m, ruleManager, query)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.compacted = compacted
	query, err = m.RefGet(compacted)
	if err != nil {
		return lbool.Undef, answer, err
	}
	variables, err := ruleManager.ExpressionVariables(query)
	if err != nil {
		return lbool.Undef, answer, err
	}
	count := uint32(0)
	for index := range variables {
		if index >= count {
			count = index + 1
		}
	}
	domain := make([]ast.ASTHandle, count)
	arguments := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		variableSort, found := variables[index]
		if !found {
			return lbool.Undef, answer, ast.ErrASTDomain
		}
		domain[count-index-1] = variableSort
		arguments[index], err = m.MkVar(index, variableSort)
		if err != nil {
			return lbool.Undef, answer, err
		}
	}
	prefix, err := m.Symbols.Intern("query")
	if err != nil {
		return lbool.Undef, answer, err
	}
	declaration, err := m.MkFreshFuncDecl(prefix, symbol.Symbol{}, domain, boolean)
	if err != nil {
		return lbool.Undef, answer, err
	}
	if err = ruleManager.RegisterPredicate(declaration); err != nil {
		return lbool.Undef, answer, err
	}
	// Domain order is outermost-first; applications use De Bruijn order.
	headArguments := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		headArguments[index] = arguments[count-index-1]
	}
	head, err := m.MkApp(declaration, headArguments)
	if err != nil {
		return lbool.Undef, answer, err
	}
	implication, err := m.MkImplies(query, head)
	if err != nil {
		return lbool.Undef, answer, err
	}
	scoped, err := base.NewRuleSet(ruleManager)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.scoped = scoped
	if err = ruleManager.AddFormula(scoped, implication, ""); err != nil {
		return lbool.Undef, answer, err
	}
	// Preserve the query's fragment checks even though its rule no longer
	// belongs to the indexed program collected by queryPredicate.
	attempt.properties, err = base.NewRuleProperties(ruleManager)
	if err != nil {
		return lbool.Undef, answer, err
	}
	if err = attempt.properties.Collect(scoped); err != nil {
		return lbool.Undef, answer, err
	}
	if err = attempt.properties.CheckExistentialTail(); err != nil {
		return lbool.Undef, answer, err
	}
	if err = attempt.properties.CheckNegativePredicates(); err != nil {
		return lbool.Undef, answer, err
	}
	countRules, err := scoped.Len()
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.queryRule, err = scoped.Rule(countRules - 1)
	if err != nil {
		return lbool.Undef, answer, err
	}
	return queryPredicate(rules, ast.ASTHandle{}, attempt.queryRule, rw, lite, querySolver, subsumptionSolver, strategy, session, stats, evaluator...)
}

func compactExpressionVariables(m *ast.ASTManager, ruleManager *base.RuleManager, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if m == nil {
		return result, ast.ErrASTHandle
	}
	variables, err := ruleManager.ExpressionVariables(expression)
	if err != nil {
		return result, err
	}
	count := uint32(0)
	for index := range variables {
		if index >= count {
			count = index + 1
		}
	}
	dense := uint32(len(variables)) == count
	if dense {
		return m.NewRef(expression)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, count)
	next := uint32(0)
	for index := uint32(0); index < count; index++ {
		variableSort, found := variables[index]
		if !found {
			bindings[index], err = m.MkVar(0, boolean)
		} else {
			bindings[index], err = m.MkVar(next, variableSort)
			next++
		}
		if err != nil {
			return result, err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, substitution.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(expression, bindings, result); err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}
