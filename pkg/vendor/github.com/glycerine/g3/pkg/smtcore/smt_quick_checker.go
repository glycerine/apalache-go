package smtcore

// Source: src/smt/smt_quick_checker.h and src/smt/smt_quick_checker.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtQuickCollectorEntry struct {
	expression ast.ASTHandle
	parent     ast.ASTHandle
	position   uint32
}

type smtQuickCheckKey struct {
	expression ast.ASTHandle
	value      bool
}

// SMTQuickChecker finds quantifier bindings falsified, or not proven true, by
// the current congruence classes and assignments.
type SMTQuickChecker struct {
	context    *SMTCheckerContext
	state      *SMTInternalizerState
	rewriter   *rewriter.TheoryRewriter
	registry   *SMTQuantifierRegistry
	values     map[*SMTEnode]ast.ASTHandle
	candidates [][]*SMTEnode
	bindings   []*SMTEnode
	checkCache map[smtQuickCheckKey]bool
	canonCache map[ast.ASTHandle]ast.ASTHandle
	newExprs   []ast.ASTRef
}

func NewSMTQuickChecker(context *SMTCheckerContext, rw *rewriter.TheoryRewriter, registry *SMTQuantifierRegistry) (*SMTQuickChecker, error) {
	if context == nil || context.manager == nil || rw == nil || rw.Manager() != context.manager || registry == nil || registry.manager != context.manager {
		return nil, ast.ErrASTHandle
	}
	return &SMTQuickChecker{context: context, rewriter: rw, registry: registry, values: make(map[*SMTEnode]ast.ASTHandle)}, nil
}

// SetValue records the concrete model value used when canonicalizing an
// internalized equality-class root.
func (q *SMTQuickChecker) SetValue(node *SMTEnode, value ast.ASTHandle) error {
	if q == nil || q.context == nil || node == nil || node.manager != q.context.manager || node.root == nil {
		return ast.ErrASTDomain
	}
	if _, err := q.context.manager.GetKind(value); err != nil {
		return err
	}
	q.values[node.root] = value
	return nil
}

func (q *SMTQuickChecker) collectorCheckArg(node *SMTEnode, parent ast.ASTHandle, position uint32, conservative bool) bool {
	if parent.IsNull() || !conservative {
		return true
	}
	for _, current := range q.context.byDeclaration[parent] {
		if !q.context.isRelevant(current.owner) || !current.IsCongruenceRoot() || position >= uint32(len(current.arguments)) {
			continue
		}
		if current.arguments[position].root == node.root {
			return true
		}
	}
	return false
}

func (q *SMTQuickChecker) collectCore(expression, parent ast.ASTHandle, parentPosition uint32, conservative bool, found []bool, sets []map[*SMTEnode]struct{}, temporary []map[*SMTEnode]struct{}, cache map[smtQuickCollectorEntry]struct{}) error {
	decl, err := q.context.manager.GetDecl(expression)
	if err != nil {
		return err
	}
	count, err := q.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for position := uint32(0); position < count; position++ {
		argument, err := q.context.manager.GetArg(expression, position)
		if err != nil {
			return err
		}
		kind, err := q.context.manager.GetKind(argument)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			index, err := q.context.manager.GetVarIndex(argument)
			if err != nil {
				return err
			}
			if index >= uint32(len(found)) {
				return nil
			}
			if found[index] && conservative {
				if len(sets[index]) == 0 {
					continue
				}
				clear(temporary[index])
				for _, current := range q.context.byDeclaration[decl] {
					if q.context.isRelevant(current.owner) && current.IsCongruenceRoot() && q.collectorCheckArg(current, parent, parentPosition, true) && position < uint32(len(current.arguments)) {
						root := current.arguments[position].root
						if _, present := sets[index][root]; present {
							temporary[index][root] = struct{}{}
						}
					}
				}
				sets[index], temporary[index] = temporary[index], sets[index]
				continue
			}
			found[index] = true
			for _, current := range q.context.byDeclaration[decl] {
				if q.context.isRelevant(current.owner) && current.IsCongruenceRoot() && q.collectorCheckArg(current, parent, parentPosition, conservative) && position < uint32(len(current.arguments)) {
					sets[index][current.arguments[position].root] = struct{}{}
				}
			}
			continue
		}
		family, err := q.context.manager.GetFamilyID(expression)
		if err != nil {
			return err
		}
		if family == q.context.manager.BasicFamilyID() {
			if err = q.collect(argument, ast.ASTHandle{}, 0, conservative, found, sets, temporary, cache); err != nil {
				return err
			}
		} else if err = q.collect(argument, decl, position, conservative, found, sets, temporary, cache); err != nil {
			return err
		}
	}
	return nil
}

func (q *SMTQuickChecker) collect(expression, parent ast.ASTHandle, position uint32, conservative bool, found []bool, sets []map[*SMTEnode]struct{}, temporary []map[*SMTEnode]struct{}, cache map[smtQuickCollectorEntry]struct{}) error {
	kind, err := q.context.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier || kind == ast.ASTVar {
		return nil
	}
	ground, err := q.context.manager.IsGround(expression)
	if err != nil || ground {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	entry := smtQuickCollectorEntry{expression: expression, parent: parent, position: position}
	if _, present := cache[entry]; present {
		return nil
	}
	cache[entry] = struct{}{}
	return q.collectCore(expression, parent, position, conservative, found, sets, temporary, cache)
}

// CollectCandidates implements the source collector and returns candidates by
// de-Bruijn variable index.
func (q *SMTQuickChecker) CollectCandidates(quantifier ast.ASTHandle, conservative bool) ([][]*SMTEnode, error) {
	if q == nil || q.context == nil {
		return nil, ast.ErrASTHandle
	}
	count, err := q.context.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return nil, err
	}
	body, err := q.context.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return nil, err
	}
	found := make([]bool, count)
	sets := make([]map[*SMTEnode]struct{}, count)
	temporary := make([]map[*SMTEnode]struct{}, count)
	for index := range sets {
		sets[index] = make(map[*SMTEnode]struct{})
		temporary[index] = make(map[*SMTEnode]struct{})
	}
	if err = q.collect(body, ast.ASTHandle{}, 0, conservative, found, sets, temporary, make(map[smtQuickCollectorEntry]struct{})); err != nil {
		return nil, err
	}
	result := make([][]*SMTEnode, count)
	for index := range sets {
		for candidate := range sets[index] {
			result[index] = append(result[index], candidate)
		}
	}
	return result, nil
}

func (q *SMTQuickChecker) clearQuery() error {
	var result error
	for index := len(q.newExprs); index > 0; index-- {
		result = errors.Join(result, q.context.manager.DelRef(&q.newExprs[index-1]))
	}
	q.newExprs = nil
	q.checkCache = nil
	q.canonCache = nil
	return result
}

func (q *SMTQuickChecker) retainApplication(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := q.context.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q.newExprs = append(q.newExprs, ref)
	if err = q.rewriter.MakeApp(declaration, ast.SliceArguments(arguments), ref); err != nil {
		return ast.ASTHandle{}, err
	}
	return q.context.manager.RefGet(ref)
}

func (q *SMTQuickChecker) congruent(declaration ast.ASTHandle, arguments []*SMTEnode) *SMTEnode {
	for _, candidate := range q.context.byDeclaration[declaration] {
		if !candidate.IsCongruenceRoot() || len(candidate.arguments) != len(arguments) {
			continue
		}
		matched := true
		for index := range arguments {
			if candidate.arguments[index].root != arguments[index].root {
				matched = false
				break
			}
		}
		if !matched && candidate.commutative && len(arguments) == 2 {
			matched = candidate.arguments[0].root == arguments[1].root && candidate.arguments[1].root == arguments[0].root
		}
		if matched {
			return candidate
		}
	}
	return nil
}

func (q *SMTQuickChecker) canonize(expression ast.ASTHandle) (ast.ASTHandle, error) {
	kind, err := q.context.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTVar {
		index, err := q.context.manager.GetVarIndex(expression)
		if err != nil || index >= uint32(len(q.bindings)) {
			return expression, err
		}
		return q.bindings[len(q.bindings)-int(index)-1].root.owner, nil
	}
	if node := q.context.enodes[expression]; node != nil {
		return node.root.owner, nil
	}
	if kind != ast.ASTApp {
		return expression, nil
	}
	count, err := q.context.manager.GetNumArgs(expression)
	if err != nil || count == 0 {
		return expression, err
	}
	if result, present := q.canonCache[expression]; present {
		return result, nil
	}
	arguments := make([]ast.ASTHandle, count)
	argumentNodes := make([]*SMTEnode, count)
	allInternalized := true
	for index := uint32(0); index < count; index++ {
		arguments[index], err = q.context.manager.GetArg(expression, index)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		arguments[index], err = q.canonize(arguments[index])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		argumentNodes[index] = q.context.enodes[arguments[index]]
		if argumentNodes[index] == nil {
			allInternalized = false
		}
	}
	declaration, err := q.context.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if allInternalized {
		if node := q.congruent(declaration, argumentNodes); node != nil {
			q.canonCache[expression] = node.root.owner
			return node.root.owner, nil
		}
	}
	for index, node := range argumentNodes {
		if node != nil {
			value, present := q.values[node.root]
			if q.state != nil {
				var err error
				value, present, err = q.state.GetTheoryValue(node)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			if present {
				ref, err := q.context.manager.NewRef(value)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				q.newExprs = append(q.newExprs, ref)
				arguments[index] = value
			}
		}
	}
	result, err := q.retainApplication(declaration, arguments)
	if err == nil {
		q.canonCache[expression] = result
	}
	return result, err
}

func (q *SMTQuickChecker) allArgs(expression ast.ASTHandle, value bool) (bool, error) {
	count, err := q.context.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := q.context.manager.GetArg(expression, index)
		if err != nil {
			return false, err
		}
		matched, err := q.check(argument, value)
		if err != nil || !matched {
			return matched, err
		}
	}
	return true, nil
}

func (q *SMTQuickChecker) anyArg(expression ast.ASTHandle, value bool) (bool, error) {
	count, err := q.context.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := q.context.manager.GetArg(expression, index)
		if err != nil {
			return false, err
		}
		matched, err := q.check(argument, value)
		if err != nil || matched {
			return matched, err
		}
	}
	return false, nil
}

func (q *SMTQuickChecker) checkCore(expression ast.ASTHandle, value bool) (bool, error) {
	sort, err := q.context.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	boolean, err := q.context.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	if sort != boolean {
		return false, ast.ErrASTDomain
	}
	if _, internalized := q.context.assignments[expression]; internalized && q.context.isRelevant(expression) {
		assignment := q.context.assignments[expression]
		return assignment != lbool.Undef && value == (assignment == lbool.True), nil
	}
	kind, err := q.context.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := q.context.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	if family == q.context.manager.BasicFamilyID() {
		op, err := q.context.manager.GetDeclKind(expression)
		if err != nil {
			return false, err
		}
		arg := func(index uint32) (ast.ASTHandle, error) { return q.context.manager.GetArg(expression, index) }
		switch op {
		case ast.ASTOpTrue:
			return value, nil
		case ast.ASTOpFalse:
			return !value, nil
		case ast.ASTOpNot:
			child, err := arg(0)
			if err != nil {
				return false, err
			}
			return q.check(child, !value)
		case ast.ASTOpOr:
			if value {
				return q.anyArg(expression, true)
			}
			return q.allArgs(expression, false)
		case ast.ASTOpAnd:
			if value {
				return q.allArgs(expression, true)
			}
			return q.anyArg(expression, false)
		case ast.ASTOpIff:
			left, e := arg(0)
			if e != nil {
				return false, e
			}
			right, e := arg(1)
			if e != nil {
				return false, e
			}
			lt, e := q.check(left, true)
			if e != nil {
				return false, e
			}
			lf, e := q.check(left, false)
			if e != nil {
				return false, e
			}
			rt, e := q.check(right, true)
			if e != nil {
				return false, e
			}
			rf, e := q.check(right, false)
			if e != nil {
				return false, e
			}
			if value {
				return lt && rt || lf && rf, nil
			}
			return lt && rf || lf && rt, nil
		case ast.ASTOpIte:
			condition, e := arg(0)
			if e != nil {
				return false, e
			}
			thenTerm, e := arg(1)
			if e != nil {
				return false, e
			}
			elseTerm, e := arg(2)
			if e != nil {
				return false, e
			}
			conditionTrue, e := q.check(condition, true)
			if e != nil {
				return false, e
			}
			if conditionTrue {
				return q.check(thenTerm, value)
			}
			conditionFalse, e := q.check(condition, false)
			if e != nil {
				return false, e
			}
			if conditionFalse {
				return q.check(elseTerm, value)
			}
			thenMatch, e := q.check(thenTerm, value)
			if e != nil || !thenMatch {
				return thenMatch, e
			}
			return q.check(elseTerm, value)
		case ast.ASTOpEq:
			left, e := arg(0)
			if e != nil {
				return false, e
			}
			right, e := arg(1)
			if e != nil {
				return false, e
			}
			left, e = q.canonize(left)
			if e != nil {
				return false, e
			}
			right, e = q.canonize(right)
			if e != nil {
				return false, e
			}
			if value {
				return left == right, nil
			}
			leftNode, rightNode := q.context.enodes[left], q.context.enodes[right]
			if leftNode != nil && rightNode != nil && q.context.isRelevant(left) && q.context.isRelevant(right) && leftNode.root != rightNode.root {
				return true, nil
			}
			return q.context.manager.AreDistinct(left, right)
		}
	}
	canonical, err := q.canonize(expression)
	if err != nil {
		return false, err
	}
	if assignment, internalized := q.context.assignments[canonical]; internalized && q.context.isRelevant(canonical) && assignment != lbool.Undef {
		return value == (assignment == lbool.True), nil
	}
	op, err := q.context.manager.GetDeclKind(canonical)
	if err == nil {
		if value && op == ast.ASTOpTrue || !value && op == ast.ASTOpFalse {
			return true, nil
		}
	}
	return false, nil
}

func (q *SMTQuickChecker) check(expression ast.ASTHandle, value bool) (bool, error) {
	key := smtQuickCheckKey{expression: expression, value: value}
	if result, present := q.checkCache[key]; present {
		return result, nil
	}
	result, err := q.checkCore(expression, value)
	if err == nil {
		q.checkCache[key] = result
	}
	return result, err
}

func (q *SMTQuickChecker) checkQuantifier(quantifier ast.ASTHandle, value bool) (result bool, err error) {
	body, err := q.context.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return false, err
	}
	q.checkCache = make(map[smtQuickCheckKey]bool)
	q.canonCache = make(map[ast.ASTHandle]ast.ASTHandle)
	defer func() { err = errors.Join(err, q.clearQuery()) }()
	return q.check(body, value)
}

func (q *SMTQuickChecker) processCandidates(quantifier ast.ASTHandle, unsat bool) (bool, error) {
	for _, candidates := range q.candidates {
		if len(candidates) == 0 {
			return false, nil
		}
	}
	q.bindings = make([]*SMTEnode, len(q.candidates))
	generated := false
	var visit func(int) error
	visit = func(index int) error {
		if index < len(q.candidates) {
			for _, candidate := range q.candidates[index] {
				q.bindings[len(q.bindings)-index-1] = candidate
				if err := visit(index + 1); err != nil {
					return err
				}
			}
			return nil
		}
		id, err := q.context.manager.GetASTID(quantifier)
		if err != nil {
			return err
		}
		contained, err := q.registry.fingerprints.Contains(quantifier, id, q.bindings)
		if err != nil || contained {
			return err
		}
		candidate := false
		if unsat {
			candidate, err = q.checkQuantifier(quantifier, false)
			if err != nil {
				return err
			}
		} else {
			matched, err := q.checkQuantifier(quantifier, true)
			if err != nil {
				return err
			}
			candidate = !matched
		}
		if !candidate {
			return nil
		}
		inserted, err := q.registry.AddInstance(quantifier, ast.ASTHandle{}, q.bindings, SMTEnodesMaxGeneration(q.bindings), 0, 0)
		if err != nil {
			return err
		}
		generated = generated || inserted
		return nil
	}
	if err := visit(0); err != nil {
		return false, err
	}
	return generated, nil
}

func (q *SMTQuickChecker) InstantiateUnsat(quantifier ast.ASTHandle) (bool, error) {
	candidates, err := q.CollectCandidates(quantifier, true)
	if err != nil {
		return false, err
	}
	q.candidates = candidates
	return q.processCandidates(quantifier, true)
}

func (q *SMTQuickChecker) InstantiateNotSat(quantifier ast.ASTHandle) (bool, error) {
	candidates, err := q.CollectCandidates(quantifier, false)
	if err != nil {
		return false, err
	}
	q.candidates = candidates
	return q.processCandidates(quantifier, false)
}

func (q *SMTQuickChecker) internalizeCandidate(expression ast.ASTHandle) (*SMTEnode, error) {
	if node := q.context.enodes[expression]; node != nil {
		return node, nil
	}
	kind, err := q.context.manager.GetKind(expression)
	if err != nil {
		return nil, err
	}
	ground, err := q.context.manager.IsGround(expression)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp || !ground {
		return nil, ast.ErrASTDomain
	}
	count, err := q.context.manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := q.context.manager.GetArg(expression, index)
		if err != nil {
			return nil, err
		}
		if _, err = q.internalizeCandidate(argument); err != nil {
			return nil, err
		}
	}
	node, err := NewSMTEnode(q.context.manager, expression, q.context.enodes, SMTEnodeOptions{CongruenceEnabled: true})
	if err != nil {
		return nil, err
	}
	if count != 0 {
		node.congruence = node
	}
	if err = q.context.RegisterEnode(node, false); err != nil {
		return nil, err
	}
	return node, nil
}

// InstantiateNotSatCandidates uses a caller-supplied pool, grouped by the
// declaration sort of each quantifier variable.
func (q *SMTQuickChecker) InstantiateNotSatCandidates(quantifier ast.ASTHandle, expressions []ast.ASTHandle) (bool, error) {
	count, err := q.context.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	q.candidates = make([][]*SMTEnode, count)
	for index := uint32(0); index < count; index++ {
		sort, err := q.context.manager.GetDeclarationSort(quantifier, index)
		if err != nil {
			return false, err
		}
		for _, expression := range expressions {
			expressionSort, err := q.context.manager.GetSort(expression)
			if err != nil {
				return false, err
			}
			if expressionSort != sort {
				continue
			}
			node, err := q.internalizeCandidate(expression)
			if err != nil {
				return false, err
			}
			q.candidates[index] = append(q.candidates[index], node)
		}
	}
	return q.processCandidates(quantifier, false)
}
