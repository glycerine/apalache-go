package base

// Source: src/muz/base/hnf.cpp and hnf.h. This file ports the no-proof
// Horn-normal-form worklist, disjunctive and quantified-body definitions,
// fresh predicate construction, and universal-prefix reconstruction.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrHornNormalizationOwner = errors.New("muz/base: invalid or closed Horn normalization")

// HornNormalization owns normalized formulas, optional matching proofs, and
// fresh predicate declarations produced by a HornNormalizer.
type HornNormalization struct {
	manager  *ast.ASTManager
	Formulas []ast.ASTRef
	// Proofs is parallel to Formulas when NormalizeWithProof is used.
	Proofs          []ast.ASTRef
	FreshPredicates []ast.ASTRef
	closed          bool
}

func (r *HornNormalization) Close() error {
	if r == nil || r.manager == nil {
		return ErrHornNormalizationOwner
	}
	if r.closed {
		return nil
	}
	// Source ref_vector destruction decrements each vector in forward order.
	// A decrement may consume its reference before reporting a later error;
	// keep only the unfinished suffix and stop before touching later owners.
	for _, refs := range []*[]ast.ASTRef{&r.Proofs, &r.Formulas, &r.FreshPredicates} {
		for len(*refs) != 0 {
			err := r.manager.DelRef(&(*refs)[0])
			if (*refs)[0] == (ast.ASTRef{}) {
				*refs = (*refs)[1:]
			}
			if err != nil {
				return err
			}
		}
		*refs = nil
	}
	r.closed = true
	return nil
}

type hornPrefix struct {
	sorts []ast.ASTHandle
	names []usymbol.Symbol
}

type hornWork struct {
	formula ast.ASTHandle
}

// HornNormalizer converts formulas into a conjunction of Horn formulas. It
// owns its hoister and source cache/body roots, and borrows the AST manager.
// Close it before closing the manager; returned results own independent refs.
type HornNormalizer struct {
	manager         *ast.ASTManager
	prefix          usymbol.Symbol
	hoister         *rewriter.QuantifierHoister
	workRefs        []ast.ASTRef
	todoRefs        []ast.ASTRef
	bodyRefs        []ast.ASTRef
	freshPredicates []ast.ASTRef
	closed          bool
	cleanupPending  hornCleanup
}

func NewHornNormalizer(manager *ast.ASTManager, name string) (*HornNormalizer, error) {
	if manager == nil {
		return nil, ErrHornNormalizationOwner
	}
	if name == "" {
		name = "P"
	}
	prefix, err := manager.Symbols.Intern(name)
	if err != nil {
		return nil, err
	}
	hoister, err := rewriter.NewDefaultQuantifierHoister(manager)
	if err != nil {
		return nil, err
	}
	return &HornNormalizer{manager: manager, prefix: prefix, hoister: hoister}, nil
}

func (n *HornNormalizer) pin(locals *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := n.manager.NewRef(handle)
	if err != nil {
		return err
	}
	*locals = append(*locals, reference)
	return nil
}

func (n *HornNormalizer) predicate(expression ast.ASTHandle) (bool, error) {
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	declaration, err := n.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	family, err := n.manager.GetFamilyID(declaration)
	if err != nil {
		return false, err
	}
	rangeSort, err := n.manager.GetRange(declaration)
	if err != nil {
		return false, err
	}
	boolean, err := n.manager.MkBoolSort()
	return family == ast.NullFamilyID && rangeSort == boolean, err
}

// isHorn is the source invocation fast path, before resetting worklists or
// checking cancellation. It preserves an already-Horn formula and its proof.
func (n *HornNormalizer) isHorn(formula ast.ASTHandle) (bool, error) {
	m := n.manager
	for {
		kind, err := m.GetKind(formula)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTQuantifier {
			break
		}
		forall, err := m.IsForall(formula)
		if err != nil {
			return false, err
		}
		if !forall {
			break
		}
		formula, err = m.GetQuantifierBody(formula)
		if err != nil {
			return false, err
		}
	}
	op, app, err := n.operator(formula)
	if err != nil || !app || op != ast.ASTOpImplies {
		return false, err
	}
	head, err := m.GetArg(formula, 1)
	if err != nil {
		return false, err
	}
	predicate, err := n.predicate(head)
	if err != nil || !predicate {
		return false, err
	}
	body, err := m.GetArg(formula, 0)
	if err != nil {
		return false, err
	}
	kind, err := m.GetKind(body)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		return true, nil
	}
	if kind == ast.ASTQuantifier {
		return false, nil
	}
	op, app, err = n.operator(body)
	if err != nil {
		return false, err
	}
	parts := []ast.ASTHandle{body}
	if app && op == ast.ASTOpAnd {
		count, err := m.GetNumArgs(body)
		if err != nil {
			return false, err
		}
		parts = nil
		for i := uint32(0); i < count; i++ {
			part, err := m.GetArg(body, i)
			if err != nil {
				return false, err
			}
			parts = append(parts, part)
		}
	}
	for _, part := range parts {
		predicate, err := n.predicate(part)
		if err != nil {
			return false, err
		}
		if !predicate {
			contains, err := n.containsPredicate(part)
			if err != nil || contains {
				return false, err
			}
		}
	}
	return true, nil
}

func (n *HornNormalizer) containsPredicate(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := n.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			predicate, e := n.predicate(current)
			if e != nil {
				return false, e
			}
			if predicate {
				return true, nil
			}
			count, e := n.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for i := count; i > 0; i-- {
				argument, x := n.manager.GetArg(current, i-1)
				if x != nil {
					return false, x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			// quick_for_each_expr visits body, patterns and no-patterns.
			body, e := n.manager.GetQuantifierBody(current)
			if e != nil {
				return false, e
			}
			children := []ast.ASTHandle{body}
			patterns, e := n.manager.GetNumPatterns(current)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < patterns; i++ {
				child, e := n.manager.GetPattern(current, i)
				if e != nil {
					return false, e
				}
				children = append(children, child)
			}
			noPatterns, e := n.manager.GetNumNoPatterns(current)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < noPatterns; i++ {
				child, e := n.manager.GetNoPattern(current, i)
				if e != nil {
					return false, e
				}
				children = append(children, child)
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		}
	}
	return false, nil
}

func (n *HornNormalizer) operator(expression ast.ASTHandle) (int32, bool, error) {
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return -1, false, err
	}
	family, err := n.manager.GetFamilyID(expression)
	if err != nil || family != n.manager.BasicFamilyID() {
		return -1, false, err
	}
	operator, err := n.manager.GetDeclKind(expression)
	return operator, err == nil, err
}

func (n *HornNormalizer) pullForall(hoister *rewriter.QuantifierHoister, formula ast.ASTHandle, locals *[]ast.ASTRef) (core ast.ASTHandle, prefix hornPrefix, resultErr error) {
	pulled, err := hoister.PullQuantifierIndexed(true, formula, true, true)
	if err != nil {
		return ast.ASTHandle{}, hornPrefix{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, pulled.Close()) }()
	core, err = n.manager.RefGet(pulled.Formula)
	if err != nil {
		return ast.ASTHandle{}, hornPrefix{}, err
	}
	if err = n.pin(locals, core); err != nil {
		return ast.ASTHandle{}, hornPrefix{}, err
	}
	return core, hornPrefix{sorts: append([]ast.ASTHandle(nil), pulled.Sorts...), names: append([]usymbol.Symbol(nil), pulled.Names...)}, nil
}

func (n *HornNormalizer) bind(prefix hornPrefix, body ast.ASTHandle, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	if len(prefix.sorts) == 0 {
		return body, nil
	}
	owner, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*locals = append(*locals, owner)
	result, err := n.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: prefix.sorts, Names: prefix.names, Body: body})
	if err = errors.Join(err, n.manager.SetRef(owner, result)); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (n *HornNormalizer) freshHead(expression ast.ASTHandle, result *HornNormalization, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	used, err := rewriter.NewExprFreeVars(n.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = used.Collect(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0, used.Size())
	arguments := make([]ast.ASTHandle, 0, used.Size())
	for i := uint32(0); i < used.Size(); i++ {
		sort, err := used.Get(i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if sort.IsNull() {
			continue
		}
		domain = append(domain, sort)
		variable, e := n.manager.MkVar(i, sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments = append(arguments, variable)
	}
	boolean, err := n.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source converts the stored label through symbol::str before the
	// char-pointer overload interns the textual prefix. The label itself stays
	// unchanged, including numerical identity and present-empty spelling.
	text, err := n.manager.Symbols.String(n.prefix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	prefix, err := n.manager.Symbols.Intern(text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// The empty, non-null suffix contributes a second separator to the name.
	suffix, err := n.manager.Symbols.Intern("")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declarationRef, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result.FreshPredicates = append(result.FreshPredicates, declarationRef)
	declaration, err := n.manager.MkFreshFuncDecl(prefix, suffix, domain, boolean)
	if err = errors.Join(err, n.manager.SetRef(declarationRef, declaration)); err != nil {
		return ast.ASTHandle{}, err
	}
	cached, err := n.manager.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n.freshPredicates = append(n.freshPredicates, cached)
	headRef, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*locals = append(*locals, headRef)
	head, err := n.manager.MkApp(declaration, arguments)
	if err = errors.Join(err, n.manager.SetRef(headRef, head)); err != nil {
		return ast.ASTHandle{}, err
	}
	return head, nil
}

func (n *HornNormalizer) implication(body []ast.ASTHandle, head ast.ASTHandle, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	if len(body) == 0 {
		return head, nil
	}
	antecedent := body[0]
	var err error
	if len(body) > 1 {
		owner, e := n.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*locals = append(*locals, owner)
		antecedent, err = n.manager.MkAndArgs(body)
		if err = errors.Join(err, n.manager.SetRef(owner, antecedent)); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	owner, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*locals = append(*locals, owner)
	formula, err := n.manager.MkImplies(antecedent, head)
	if err = errors.Join(err, n.manager.SetRef(owner, formula)); err != nil {
		return ast.ASTHandle{}, err
	}
	return formula, nil
}

// Normalize returns the conjunction of Horn formulas and its fresh predicate
// declarations.  Compatible nested universal quantifiers are first pulled
// using the same polarity rules as the source quantifier_hoister.
func (n *HornNormalizer) Normalize(formula ast.ASTHandle) (result *HornNormalization, resultErr error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	result = &HornNormalization{manager: n.manager}
	failed := true
	defer func() {
		if failed || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	horn, err := n.isHorn(formula)
	if err != nil {
		return result, err
	}
	if horn {
		ref, err := n.manager.NewRef(formula)
		if err != nil {
			return result, err
		}
		result.Formulas = append(result.Formulas, ref)
		if err := n.traceHornResult(formula, result, true); err != nil {
			return result, err
		}
		failed = false
		return result, nil
	}
	if err = n.Reset(); err != nil {
		return result, err
	}
	var locals []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(n.manager, locals)) }()
	if err := n.pin(&locals, formula); err != nil {
		return result, err
	}
	hoister := n.hoister
	work := []hornWork{{formula: formula}}
	// Transfer pending work before lexical roots close, matching m_todo's
	// lifetime across a canceled invocation and the already-Horn shortcut.
	defer func() {
		for _, item := range work {
			resultErr = errors.Join(resultErr, n.pin(&n.todoRefs, item.formula))
		}
	}()
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	for len(work) != 0 {
		canceled, cancelErr := n.manager.Canceled()
		if cancelErr != nil {
			return result, cancelErr
		}
		if canceled {
			break
		}
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if err := n.beginHornStep(); err != nil {
			return result, err
		}
		core, prefix, err := n.pullForall(hoister, item.formula, &locals)
		if err != nil {
			return result, err
		}
		head := core
		var body []ast.ASTHandle
		for {
			operator, application, err := n.operator(head)
			if err != nil {
				return result, err
			}
			if !application || operator != ast.ASTOpImplies {
				break
			}
			antecedent, err := n.manager.GetArg(head, 0)
			if err != nil {
				return result, err
			}
			body = append(body, antecedent)
			head, err = n.manager.GetArg(head, 1)
			if err != nil {
				return result, err
			}
		}
		body, err = n.manager.FlattenAnd(body)
		if err != nil {
			return result, err
		}
		if err = n.storeHornBody(body); err != nil {
			return result, err
		}
		if len(body) == 1 {
			operator, application, e := n.operator(body[0])
			if e != nil {
				return result, e
			}
			contains, e := n.containsPredicate(body[0])
			if e != nil {
				return result, e
			}
			if application && operator == ast.ASTOpOr && contains {
				count, e := n.manager.GetNumArgs(body[0])
				if e != nil {
					return result, e
				}
				for i := uint32(0); i < count; i++ {
					disjunct, x := n.manager.GetArg(body[0], i)
					if x != nil {
						return result, x
					}
					definition, x := n.implication([]ast.ASTHandle{disjunct}, head, &locals)
					if x != nil {
						return result, x
					}
					definition, x = n.bind(prefix, definition, &locals)
					if x != nil {
						return result, x
					}
					work = append(work, hornWork{formula: definition})
				}
				continue
			}
		}
		for i, expression := range body {
			contains, e := n.containsPredicate(expression)
			if e != nil {
				return result, e
			}
			if !contains {
				continue
			}
			if e := n.traceHornBody(expression); e != nil {
				return result, e
			}
			operator, application, e := n.operator(expression)
			if e != nil {
				return result, e
			}
			if !application {
				continue
			}
			var disjuncts []ast.ASTHandle
			negate := false
			switch operator {
			case ast.ASTOpOr:
				count, x := n.manager.GetNumArgs(expression)
				if x != nil {
					return result, x
				}
				for j := uint32(0); j < count; j++ {
					argument, x := n.manager.GetArg(expression, j)
					if x != nil {
						return result, x
					}
					disjuncts = append(disjuncts, argument)
				}
			case ast.ASTOpNot:
				argument, x := n.manager.GetArg(expression, 0)
				if x != nil {
					return result, x
				}
				if op, app, x := n.operator(argument); x == nil && app && op == ast.ASTOpAnd {
					// Source uses the immediate conjunction arguments. Nested
					// conjunctions receive their own definition in a later step.
					count, x := n.manager.GetNumArgs(argument)
					if x != nil {
						return result, x
					}
					for j := uint32(0); j < count; j++ {
						part, x := n.manager.GetArg(argument, j)
						if x != nil {
							return result, x
						}
						disjuncts = append(disjuncts, part)
					}
					negate = true
				} else if x != nil {
					return result, x
				}
			case ast.ASTOpImplies:
				left, x := n.manager.GetArg(expression, 0)
				if x != nil {
					return result, x
				}
				right, x := n.manager.GetArg(expression, 1)
				if x != nil {
					return result, x
				}
				owner, x := n.manager.NewRef(ast.ASTHandle{})
				if x != nil {
					return result, x
				}
				locals = append(locals, owner)
				notLeft, x := n.manager.MkNotSimplified(left)
				if x = errors.Join(x, n.manager.SetRef(owner, notLeft)); x != nil {
					return result, x
				}
				disjuncts = []ast.ASTHandle{notLeft, right}
			}
			if len(disjuncts) == 0 {
				continue
			}
			fresh, found := memo[expression]
			if !found {
				fresh, e = n.freshHead(expression, result, &locals)
				if e != nil {
					return result, e
				}
				memo[expression] = fresh
				if err := n.rememberHornRoot(expression); err != nil {
					return result, err
				}
				if err := n.rememberHornRoot(fresh); err != nil {
					return result, err
				}
				for _, disjunct := range disjuncts {
					if negate {
						owner, x := n.manager.NewRef(ast.ASTHandle{})
						if x != nil {
							return result, x
						}
						locals = append(locals, owner)
						disjunct, e = n.manager.MkNot(disjunct)
						if e = errors.Join(e, n.manager.SetRef(owner, disjunct)); e != nil {
							return result, e
						}
					}
					definition, x := n.implication([]ast.ASTHandle{disjunct}, fresh, &locals)
					if x != nil {
						return result, x
					}
					definition, x = n.bind(prefix, definition, &locals)
					if x != nil {
						return result, x
					}
					work = append(work, hornWork{formula: definition})
				}
			}
			body[i] = fresh
			if err := n.replaceHornBody(i, fresh); err != nil {
				return result, err
			}
		}
		// Source eliminate_quantifier_body names compound predicate bodies
		// under universal antecedents after disjunction elimination. Each
		// occurrence gets its own definition; only disjunctions are memoized.
		for i, expression := range body {
			kind, err := n.manager.GetKind(expression)
			if err != nil {
				return result, err
			}
			if kind != ast.ASTQuantifier {
				continue
			}
			forall, err := n.manager.IsForall(expression)
			if err != nil {
				return result, err
			}
			if !forall {
				continue
			}
			contains, err := n.containsPredicate(expression)
			if err != nil {
				return result, err
			}
			if !contains {
				continue
			}
			inner, err := n.manager.GetQuantifierBody(expression)
			if err != nil {
				return result, err
			}
			predicate, err := n.predicate(inner)
			if err != nil {
				return result, err
			}
			if predicate {
				continue
			}
			fresh, err := n.freshHead(inner, result, &locals)
			if err != nil {
				return result, err
			}
			definition, err := n.implication([]ast.ASTHandle{inner}, fresh, &locals)
			if err != nil {
				return result, err
			}
			definition, err = n.bind(prefix, definition, &locals)
			if err != nil {
				return result, err
			}
			work = append(work, hornWork{formula: definition})
			body[i], err = n.manager.UpdateQuantifierBody(expression, fresh)
			if err != nil {
				return result, err
			}
			if err = n.pin(&locals, body[i]); err != nil {
				return result, err
			}
			if err = n.replaceHornBody(i, body[i]); err != nil {
				return result, err
			}
		}
		normalized, err := n.implication(body, head, &locals)
		if err != nil {
			return result, err
		}
		normalized, err = n.bind(prefix, normalized, &locals)
		if err != nil {
			return result, err
		}
		reference, err := n.manager.NewRef(normalized)
		if err != nil {
			return result, err
		}
		result.Formulas = append(result.Formulas, reference)
	}
	if err := n.traceHornResult(formula, result, false); err != nil {
		return result, err
	}
	failed = false
	return result, nil
}

// AddFormula normalizes a formula, registers introduced predicates, converts
// every Horn conjunct to a Rule, and retains it in set.
func (m *RuleManager) AddFormula(set *RuleSet, formula ast.ASTHandle, name string) (resultErr error) {
	label, err := m.ruleName(name)
	if err != nil {
		return err
	}
	return m.AddFormulaWithSymbol(set, formula, label)
}

// AddFormulaWithSymbol preserves the source label through Horn normalization
// and uses its source spelling as the prefix for introduced predicates.
func (m *RuleManager) AddFormulaWithSymbol(set *RuleSet, formula ast.ASTHandle, name usymbol.Symbol) error {
	return m.addFormula(set, formula, name, nil, false)
}

// AddFormulaWithUnboundFix applies the source rule_manager QE-lite reduction
// to each normalized rule and optionally existentially binds residual tail
// variables that do not occur in predicate positions.
func (m *RuleManager) AddFormulaWithUnboundFix(set *RuleSet, formula ast.ASTHandle, name string, lite *qe.Lite, enabled bool) error {
	if lite == nil {
		return fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	label, err := m.ruleName(name)
	if err != nil {
		return err
	}
	return m.AddFormulaWithSymbolAndUnboundFix(set, formula, label, lite, enabled)
}

// AddFormulaWithSymbolAndUnboundFix carries the exact label through the
// configured unbound-variable rewrite of each normalized clause.
func (m *RuleManager) AddFormulaWithSymbolAndUnboundFix(set *RuleSet, formula ast.ASTHandle, name usymbol.Symbol, lite *qe.Lite, enabled bool) error {
	if lite == nil {
		return fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	return m.addFormula(set, formula, name, lite, enabled)
}

func (m *RuleManager) addFormula(set *RuleSet, formula ast.ASTHandle, name usymbol.Symbol, lite *qe.Lite, fixUnbound bool) (resultErr error) {
	if err := m.checkRuleName(name); err != nil {
		return err
	}
	if set == nil || set.manager != m {
		return ErrRuleSetOwner
	}
	labelRewriter, err := rewriter.NewLabelRewriter(m.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, labelRewriter.Close()) }()
	formulaReference, err := m.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formulaReference)) }()
	proofReference, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&proofReference)) }()
	if err = labelRewriter.RemoveLabels(formulaReference, proofReference); err != nil {
		return err
	}
	formula, err = m.manager.RefGet(formulaReference)
	if err != nil {
		return err
	}
	normalizer, err := m.prepareHornNormalizer(name)
	if err != nil {
		return err
	}
	normalized, err := normalizer.Normalize(formula)
	if normalized != nil {
		defer func() { resultErr = errors.Join(resultErr, normalized.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = m.registerHornPredicates(); err != nil {
		return err
	}
	for _, reference := range normalized.Formulas {
		horn, e := m.manager.RefGet(reference)
		if e != nil {
			return e
		}
		rule, e := m.newRuleFromFormulaWithSymbol(horn, name, true)
		if e != nil {
			return e
		}
		if lite != nil {
			fixed, _, fixErr := m.FixUnboundVariables(rule, lite, fixUnbound)
			if fixErr != nil {
				return errors.Join(fixErr, rule.Close())
			}
			if closeErr := rule.Close(); closeErr != nil {
				return errors.Join(closeErr, fixed.Close())
			}
			rule = fixed
		}
		if e = set.Add(rule); e != nil {
			return errors.Join(e, rule.Close())
		}
		if e = rule.Close(); e != nil {
			return e
		}
	}
	return nil
}
