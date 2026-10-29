package smtcore

// Source: src/smt/smt_relevancy.h and src/smt/smt_relevancy.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtRelevancyHandlerKind uint8

const (
	smtRelevancySimple smtRelevancyHandlerKind = iota
	smtRelevancyPair
	smtRelevancyAnd
	smtRelevancyOr
	smtRelevancyITE
	smtRelevancyTermITE
)

type smtRelevancyHandler struct {
	kind                  smtRelevancyHandlerKind
	first, second, target ast.ASTHandle
	thenEquality          ast.ASTHandle
	elseEquality          ast.ASTHandle
}

type smtRelevancyTrailKind uint8

const (
	smtRelevancyHandlerTrail smtRelevancyTrailKind = iota
	smtRelevancyPositiveWatchTrail
	smtRelevancyNegativeWatchTrail
)

type smtRelevancyTrail struct {
	kind       smtRelevancyTrailKind
	expression ast.ASTHandle
}

type smtRelevancyScope struct {
	relevantLimit int
	trailLimit    int
}

type SMTRelevancyPropagator struct {
	context       *SMTCheckerContext
	internalizer  *SMTInternalizerState // borrowed logical-context callback owner
	enabled       bool
	relevant      []ast.ASTHandle
	qhead         int
	handlers      map[ast.ASTHandle][]smtRelevancyHandler
	positiveWatch map[ast.ASTHandle][]smtRelevancyHandler
	negativeWatch map[ast.ASTHandle][]smtRelevancyHandler
	trail         []smtRelevancyTrail
	scopes        []smtRelevancyScope
	newlyRelevant []ast.ASTHandle
	propagating   bool
}

func NewSMTRelevancyPropagator(context *SMTCheckerContext, enabled bool) (*SMTRelevancyPropagator, error) {
	if context == nil || context.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTRelevancyPropagator{
		context: context, enabled: enabled, handlers: make(map[ast.ASTHandle][]smtRelevancyHandler),
		positiveWatch: make(map[ast.ASTHandle][]smtRelevancyHandler), negativeWatch: make(map[ast.ASTHandle][]smtRelevancyHandler),
	}, nil
}

func (p *SMTRelevancyPropagator) IsRelevant(expression ast.ASTHandle) bool {
	return p != nil && (!p.enabled || p.context.relevant[expression])
}

func (p *SMTRelevancyPropagator) assignment(expression ast.ASTHandle) lbool.LBool {
	// Source find_assignment reads the atom beneath a NOT. A nongate NOT
	// can also own a Boolean variable; assignment events use its direct value.
	if p.context.manager.IsFalse(expression) {
		return lbool.False
	}
	if child, err := p.negatedArgument(expression); err == nil && !child.IsNull() {
		return p.context.assignments[child].Negated()
	}
	if value, found := p.context.assignments[expression]; found {
		return value
	}
	return lbool.Undef
}

func (p *SMTRelevancyPropagator) negatedArgument(expression ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.context.manager
	kind, err := m.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, err
	}
	family, err := m.GetFamilyID(expression)
	if err != nil || family != m.BasicFamilyID() {
		return ast.ASTHandle{}, err
	}
	op, err := m.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpNot {
		return ast.ASTHandle{}, err
	}
	return m.GetArg(expression, 0)
}

// Source context::add_rel_watch receives a literal, then watches its positive
// expression and signed truth value. Normalize before looking up watch keys.
func (p *SMTRelevancyPropagator) literalWatch(expression ast.ASTHandle, value bool) (ast.ASTHandle, bool, error) {
	for {
		child, err := p.negatedArgument(expression)
		if err != nil || child.IsNull() {
			return expression, value, err
		}
		expression, value = child, !value
	}
}

func (p *SMTRelevancyPropagator) addLiteralWatch(expression ast.ASTHandle, value bool, handler smtRelevancyHandler) error {
	expression, value, err := p.literalWatch(expression, value)
	if err != nil {
		return err
	}
	return p.addWatch(expression, value, handler)
}

func (p *SMTRelevancyPropagator) setRelevant(expression ast.ASTHandle) error {
	if p.context.relevant[expression] {
		return nil
	}
	p.context.relevant[expression] = true
	p.relevant = append(p.relevant, expression)
	p.newlyRelevant = append(p.newlyRelevant, expression)
	if p.internalizer != nil {
		return p.internalizer.notifyRelevant(expression)
	}
	return nil
}

func (p *SMTRelevancyPropagator) MarkRelevant(expression ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled || p.context.relevant[expression] {
		return nil
	}
	if _, err := p.context.manager.GetKind(expression); err != nil {
		return err
	}
	if node := p.context.enodes[expression]; node != nil {
		start := node
		for current := start; ; current = current.next {
			if current == nil || current.owner.IsNull() {
				return ast.ErrASTDomain
			}
			if err := p.setRelevant(current.owner); err != nil {
				return err
			}
			if current.next == start {
				break
			}
		}
	} else {
		if err := p.setRelevant(expression); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) DrainNewlyRelevant() []ast.ASTHandle {
	if p == nil {
		return nil
	}
	result := append([]ast.ASTHandle(nil), p.newlyRelevant...)
	p.newlyRelevant = nil
	return result
}

func (p *SMTRelevancyPropagator) invoke(handler smtRelevancyHandler) error {
	switch handler.kind {
	case smtRelevancySimple:
		return p.MarkRelevant(handler.target)
	case smtRelevancyPair:
		if p.IsRelevant(handler.first) && p.IsRelevant(handler.second) {
			return p.MarkRelevant(handler.target)
		}
		return nil
	case smtRelevancyAnd:
		if p.IsRelevant(handler.target) {
			return p.propagateAnd(handler.target)
		}
		return nil
	case smtRelevancyOr:
		if p.IsRelevant(handler.target) {
			return p.propagateOr(handler.target)
		}
		return nil
	case smtRelevancyITE:
		if p.IsRelevant(handler.target) {
			return p.propagateITE(handler.target)
		}
		return nil
	case smtRelevancyTermITE:
		if !p.IsRelevant(handler.target) {
			return nil
		}
		condition, err := p.context.manager.GetArg(handler.target, 0)
		if err != nil {
			return err
		}
		if err = p.MarkRelevant(condition); err != nil {
			return err
		}
		switch p.assignment(condition) {
		case lbool.False:
			return p.MarkRelevant(handler.elseEquality)
		case lbool.True:
			return p.MarkRelevant(handler.thenEquality)
		}
		return nil
	default:
		return ast.ErrASTDomain
	}
}

func (p *SMTRelevancyPropagator) addHandler(source ast.ASTHandle, handler smtRelevancyHandler) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	if p.context.relevant[source] {
		return p.invoke(handler)
	}
	p.handlers[source] = append(p.handlers[source], handler)
	p.trail = append(p.trail, smtRelevancyTrail{kind: smtRelevancyHandlerTrail, expression: source})
	return nil
}

func (p *SMTRelevancyPropagator) AddDependency(source, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	return p.addHandler(source, smtRelevancyHandler{kind: smtRelevancySimple, target: target})
}

func (p *SMTRelevancyPropagator) AddPairDependency(first, second, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	handler := smtRelevancyHandler{kind: smtRelevancyPair, first: first, second: second, target: target}
	if err := p.addHandler(first, handler); err != nil {
		return err
	}
	return p.addHandler(second, handler)
}

func (p *SMTRelevancyPropagator) AddAndHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, smtRelevancyHandler{kind: smtRelevancyAnd, target: parent})
}
func (p *SMTRelevancyPropagator) AddOrHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, smtRelevancyHandler{kind: smtRelevancyOr, target: parent})
}
func (p *SMTRelevancyPropagator) AddITEHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, smtRelevancyHandler{kind: smtRelevancyITE, target: parent})
}
func (p *SMTRelevancyPropagator) AddTermITEHandler(parent, thenEquality, elseEquality ast.ASTHandle) error {
	return p.addHandler(parent, smtRelevancyHandler{kind: smtRelevancyTermITE, target: parent, thenEquality: thenEquality, elseEquality: elseEquality})
}

func (p *SMTRelevancyPropagator) addWatch(expression ast.ASTHandle, value bool, handler smtRelevancyHandler) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	assignment := p.assignment(expression)
	if !value {
		assignment = assignment.Negated()
	}
	switch assignment {
	case lbool.False:
		return nil
	case lbool.True:
		return p.invoke(handler)
	case lbool.Undef:
		if value {
			p.positiveWatch[expression] = append(p.positiveWatch[expression], handler)
			p.trail = append(p.trail, smtRelevancyTrail{kind: smtRelevancyPositiveWatchTrail, expression: expression})
		} else {
			p.negativeWatch[expression] = append(p.negativeWatch[expression], handler)
			p.trail = append(p.trail, smtRelevancyTrail{kind: smtRelevancyNegativeWatchTrail, expression: expression})
		}
		return nil
	default:
		return ast.ErrASTDomain
	}
}

func (p *SMTRelevancyPropagator) AddWatchTarget(expression ast.ASTHandle, value bool, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	assignment := p.assignment(expression)
	if !value {
		assignment = assignment.Negated()
	}
	if assignment == lbool.True {
		if err := p.MarkRelevant(target); err != nil {
			return err
		}
		return p.Propagate()
	}
	return p.addWatch(expression, value, smtRelevancyHandler{kind: smtRelevancySimple, target: target})
}

// AddAndWatches installs the shared handler used by the internalizer for a
// Boolean conjunction. A false child can select itself once parent is relevant.
func (p *SMTRelevancyPropagator) AddAndWatches(parent ast.ASTHandle) error {
	handler := smtRelevancyHandler{kind: smtRelevancyAnd, target: parent}
	count, err := p.validateBasicApplication(parent, ast.ASTOpAnd)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, err := p.context.manager.GetArg(parent, index)
		if err != nil {
			return err
		}
		child, value, err := p.literalWatch(child, false)
		if err != nil {
			return err
		}
		if err = p.addWatch(child, value, handler); err != nil {
			return err
		}
	}
	return nil
}

// AddOrWatches installs the shared handler used by the internalizer for a
// Boolean disjunction. A true child can select itself once parent is relevant.
func (p *SMTRelevancyPropagator) AddOrWatches(parent ast.ASTHandle) error {
	handler := smtRelevancyHandler{kind: smtRelevancyOr, target: parent}
	count, err := p.validateBasicApplication(parent, ast.ASTOpOr)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, err := p.context.manager.GetArg(parent, index)
		if err != nil {
			return err
		}
		child, value, err := p.literalWatch(child, true)
		if err != nil {
			return err
		}
		if err = p.addWatch(child, value, handler); err != nil {
			return err
		}
	}
	return nil
}

// AddITEWatches installs both condition watches used for a Boolean ITE.
func (p *SMTRelevancyPropagator) AddITEWatches(parent ast.ASTHandle) error {
	if _, err := p.validateBasicApplication(parent, ast.ASTOpIte); err != nil {
		return err
	}
	condition, err := p.context.manager.GetArg(parent, 0)
	if err != nil {
		return err
	}
	handler := smtRelevancyHandler{kind: smtRelevancyITE, target: parent}
	if err = p.addLiteralWatch(condition, true, handler); err != nil {
		return err
	}
	return p.addLiteralWatch(condition, false, handler)
}

// Cached assertions internalized with filtering disabled have no watches.
// Restore the concrete source handlers when filtering is later enabled.
// Repeating this after a scope pop is safe: existing watches are retained,
// while missing scoped watches are recreated under the current scope.
func (p *SMTRelevancyPropagator) restoreExpressionWatches(root ast.ASTHandle) error {
	if !p.enabled {
		return nil
	}
	m := p.context.manager
	ensureWatch := func(expression ast.ASTHandle, value bool, handler smtRelevancyHandler) error {
		var err error
		expression, value, err = p.literalWatch(expression, value)
		if err != nil {
			return err
		}
		watches := p.negativeWatch[expression]
		if value {
			watches = p.positiveWatch[expression]
		}
		for _, existing := range watches {
			if existing == handler {
				return nil
			}
		}
		return p.addWatch(expression, value, handler)
	}
	ensureHandler := func(expression ast.ASTHandle, handler smtRelevancyHandler) error {
		for _, existing := range p.handlers[expression] {
			if existing == handler {
				return nil
			}
		}
		return p.addHandler(expression, handler)
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		expression := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[expression] {
			continue
		}
		seen[expression] = true
		kind, err := m.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue // Quantifier bodies are internalized through instances.
		}
		count, err := m.GetNumArgs(expression)
		if err != nil {
			return err
		}
		arguments := make([]ast.ASTHandle, count)
		for index := range arguments {
			arguments[index], err = m.GetArg(expression, uint32(index))
			if err != nil {
				return err
			}
			stack = append(stack, arguments[index])
		}
		family, err := m.GetFamilyID(expression)
		if err != nil {
			return err
		}
		if family != m.BasicFamilyID() {
			continue
		}
		op, err := m.GetDeclKind(expression)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpAnd, ast.ASTOpOr:
			handler := smtRelevancyHandler{kind: smtRelevancyAnd, target: expression}
			if op == ast.ASTOpOr {
				handler.kind = smtRelevancyOr
			}
			for _, argument := range arguments {
				if err := ensureWatch(argument, op == ast.ASTOpOr, handler); err != nil {
					return err
				}
			}
		case ast.ASTOpIte:
			if len(arguments) != 3 {
				return ast.ErrASTDomain
			}
			handler := smtRelevancyHandler{kind: smtRelevancyITE, target: expression}
			sort, err := m.GetSort(expression)
			if err != nil {
				return err
			}
			boolean, err := m.MkBoolSort()
			if err != nil {
				return err
			}
			if sort != boolean {
				if p.context.enodes[expression] == nil {
					continue
				}
				handler.kind = smtRelevancyTermITE
				handler.thenEquality, err = m.MkEq(expression, arguments[1])
				if err != nil {
					return err
				}
				handler.elseEquality, err = m.MkEq(expression, arguments[2])
				if err != nil {
					return err
				}
				if err = ensureHandler(expression, handler); err != nil {
					return err
				}
			}
			for _, value := range []bool{true, false} {
				if err := ensureWatch(arguments[0], value, handler); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// AddTermITEWatches installs the parent handler and both condition watches
// used by internalize_ite_term.
func (p *SMTRelevancyPropagator) AddTermITEWatches(parent, thenEquality, elseEquality ast.ASTHandle) error {
	if _, err := p.validateBasicApplication(parent, ast.ASTOpIte); err != nil {
		return err
	}
	condition, err := p.context.manager.GetArg(parent, 0)
	if err != nil {
		return err
	}
	handler := smtRelevancyHandler{kind: smtRelevancyTermITE, target: parent, thenEquality: thenEquality, elseEquality: elseEquality}
	if err = p.addLiteralWatch(condition, true, handler); err != nil {
		return err
	}
	if err = p.addLiteralWatch(condition, false, handler); err != nil {
		return err
	}
	return p.addHandler(parent, handler)
}

func (p *SMTRelevancyPropagator) validateBasicApplication(expression ast.ASTHandle, operation int32) (uint32, error) {
	if p == nil || p.context == nil {
		return 0, ast.ErrASTHandle
	}
	kind, err := p.context.manager.GetKind(expression)
	if err != nil {
		return 0, err
	}
	if kind != ast.ASTApp {
		return 0, ast.ErrASTDomain
	}
	family, err := p.context.manager.GetFamilyID(expression)
	if err != nil {
		return 0, err
	}
	op, err := p.context.manager.GetDeclKind(expression)
	if err != nil {
		return 0, err
	}
	if family != p.context.manager.BasicFamilyID() || op != operation {
		return 0, ast.ErrASTDomain
	}
	return p.context.manager.GetNumArgs(expression)
}

func (p *SMTRelevancyPropagator) propagateAll(expression ast.ASTHandle) error {
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := count; index > 0; index-- {
		argument, err := p.context.manager.GetArg(expression, index-1)
		if err != nil {
			return err
		}
		if err = p.MarkRelevant(argument); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) propagateOr(expression ast.ASTHandle) error {
	value := p.assignment(expression)
	if value == lbool.Undef {
		value = lbool.True
	}
	if value == lbool.False {
		return p.propagateAll(expression)
	}
	if value != lbool.True {
		return nil
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	var selected ast.ASTHandle
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.True {
			if p.context.relevant[argument] {
				return nil
			}
			if selected.IsNull() {
				selected = argument
			}
		}
	}
	if !selected.IsNull() {
		return p.MarkRelevant(selected)
	}
	return nil
}

func (p *SMTRelevancyPropagator) propagateAnd(expression ast.ASTHandle) error {
	value := p.assignment(expression)
	if value == lbool.True {
		return p.propagateAll(expression)
	}
	if value != lbool.False {
		return nil
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	var selected ast.ASTHandle
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.False {
			if p.context.relevant[argument] {
				return nil
			}
			if selected.IsNull() {
				selected = argument
			}
		}
	}
	if !selected.IsNull() {
		return p.MarkRelevant(selected)
	}
	return nil
}

func (p *SMTRelevancyPropagator) propagateITE(expression ast.ASTHandle) error {
	condition, err := p.context.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if err = p.MarkRelevant(condition); err != nil {
		return err
	}
	index := uint32(0)
	switch p.assignment(condition) {
	case lbool.False:
		index = 2
	case lbool.True:
		index = 1
	default:
		return nil
	}
	branch, err := p.context.manager.GetArg(expression, index)
	if err != nil {
		return err
	}
	return p.MarkRelevant(branch)
}

func (p *SMTRelevancyPropagator) Propagate() error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled || p.propagating {
		return nil
	}
	p.propagating = true
	defer func() { p.propagating = false }()
	for p.qhead < len(p.relevant) {
		expression := p.relevant[p.qhead]
		p.qhead++
		kind, err := p.context.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			family, err := p.context.manager.GetFamilyID(expression)
			if err != nil {
				return err
			}
			if family == p.context.manager.BasicFamilyID() {
				op, err := p.context.manager.GetDeclKind(expression)
				if err != nil {
					return err
				}
				switch op {
				case ast.ASTOpOr:
					err = p.propagateOr(expression)
				case ast.ASTOpAnd:
					err = p.propagateAnd(expression)
				case ast.ASTOpIte:
					err = p.propagateITE(expression)
				default:
					err = p.propagateAll(expression)
				}
			} else {
				err = p.propagateAll(expression)
			}
			if err != nil {
				return err
			}
		}
		handlers := p.handlers[expression]
		for index := len(handlers); index > 0; index-- {
			if err := p.invoke(handlers[index-1]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) CanPropagate() bool {
	return p != nil && p.qhead < len(p.relevant)
}

func (p *SMTRelevancyPropagator) RelevantExpressions() []ast.ASTHandle {
	if p == nil || !p.enabled {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.relevant...)
}

func (p *SMTRelevancyPropagator) checkAllArguments(expression ast.ASTHandle) error {
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if !p.IsRelevant(argument) {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) checkOr(expression ast.ASTHandle) error {
	if p.assignment(expression) == lbool.False {
		return p.checkAllArguments(expression)
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.True && p.IsRelevant(argument) {
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (p *SMTRelevancyPropagator) checkAnd(expression ast.ASTHandle) error {
	if p.assignment(expression) == lbool.True {
		return p.checkAllArguments(expression)
	}
	if p.assignment(expression) != lbool.False {
		return nil
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.False && p.IsRelevant(argument) {
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (p *SMTRelevancyPropagator) checkITE(expression ast.ASTHandle) error {
	condition, err := p.context.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if !p.IsRelevant(condition) {
		return ast.ErrASTDomain
	}
	var branchIndex uint32
	switch p.assignment(condition) {
	case lbool.False:
		branchIndex = 2
	case lbool.True:
		branchIndex = 1
	default:
		return nil
	}
	branch, err := p.context.manager.GetArg(expression, branchIndex)
	if err != nil {
		return err
	}
	sort, err := p.context.manager.GetSort(expression)
	if err != nil {
		return err
	}
	boolean, err := p.context.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		if !p.IsRelevant(branch) {
			return ast.ErrASTDomain
		}
		return nil
	}
	equality, err := p.context.manager.MkEq(expression, branch)
	if err != nil {
		return err
	}
	if !p.IsRelevant(equality) {
		return ast.ErrASTDomain
	}
	return nil
}

// Check verifies the source debug invariants for relevant expressions after
// propagation reaches a fixed point.
func (p *SMTRelevancyPropagator) Check(expressions []ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	if p.CanPropagate() {
		return ast.ErrASTDomain
	}
	for _, expression := range expressions {
		if !p.IsRelevant(expression) {
			continue
		}
		kind, err := p.context.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			if node := p.context.enodes[expression]; node != nil {
				for current := node.next; current != node; current = current.next {
					if current == nil || !p.IsRelevant(current.owner) {
						return ast.ErrASTDomain
					}
				}
			}
			continue
		}
		family, err := p.context.manager.GetFamilyID(expression)
		if err != nil {
			return err
		}
		if family != p.context.manager.BasicFamilyID() {
			if err = p.checkAllArguments(expression); err != nil {
				return err
			}
			continue
		}
		op, err := p.context.manager.GetDeclKind(expression)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpOr:
			err = p.checkOr(expression)
		case ast.ASTOpAnd:
			err = p.checkAnd(expression)
		case ast.ASTOpIte:
			err = p.checkITE(expression)
		default:
			err = p.checkAllArguments(expression)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) AssignmentEvent(expression ast.ASTHandle, value bool) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	want := lbool.FromBool(value)
	// context::propagate_relevancy passes bool_var2expr and the assigned
	// literal's value, not find_assignment's normalized NOT-child value.
	assignment, direct := p.context.assignments[expression]
	if !direct {
		assignment = p.assignment(expression)
	}
	if assignment != want {
		return ast.ErrASTDomain
	}
	if p.context.relevant[expression] {
		family, err := p.context.manager.GetFamilyID(expression)
		if err == nil && family == p.context.manager.BasicFamilyID() {
			op, err := p.context.manager.GetDeclKind(expression)
			if err != nil {
				return err
			}
			if op == ast.ASTOpOr {
				if err = p.propagateOr(expression); err != nil {
					return err
				}
			} else if op == ast.ASTOpAnd {
				if err = p.propagateAnd(expression); err != nil {
					return err
				}
			}
		}
	}
	watches := p.negativeWatch[expression]
	if value {
		watches = p.positiveWatch[expression]
	}
	for index := len(watches); index > 0; index-- {
		if err := p.invoke(watches[index-1]); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) PushScope() error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	p.scopes = append(p.scopes, smtRelevancyScope{relevantLimit: len(p.relevant), trailLimit: len(p.trail)})
	return nil
}

func (p *SMTRelevancyPropagator) PopScopes(count uint32) error {
	if p == nil || p.context == nil || count == 0 || count > uint32(len(p.scopes)) {
		return ast.ErrASTDomain
	}
	newLevel := len(p.scopes) - int(count)
	scope := p.scopes[newLevel]
	for index := len(p.relevant); index > scope.relevantLimit; index-- {
		delete(p.context.relevant, p.relevant[index-1])
	}
	p.relevant = p.relevant[:scope.relevantLimit]
	p.qhead = len(p.relevant)
	kept := p.newlyRelevant[:0]
	for _, expression := range p.newlyRelevant {
		if p.context.relevant[expression] {
			kept = append(kept, expression)
		}
	}
	p.newlyRelevant = kept
	for index := len(p.trail); index > scope.trailLimit; index-- {
		entry := p.trail[index-1]
		var handlers []smtRelevancyHandler
		switch entry.kind {
		case smtRelevancyHandlerTrail:
			handlers = p.handlers[entry.expression]
			p.handlers[entry.expression] = handlers[:len(handlers)-1]
		case smtRelevancyPositiveWatchTrail:
			handlers = p.positiveWatch[entry.expression]
			p.positiveWatch[entry.expression] = handlers[:len(handlers)-1]
		case smtRelevancyNegativeWatchTrail:
			handlers = p.negativeWatch[entry.expression]
			p.negativeWatch[entry.expression] = handlers[:len(handlers)-1]
		default:
			return ast.ErrASTDomain
		}
	}
	p.trail = p.trail[:scope.trailLimit]
	p.scopes = p.scopes[:newLevel]
	return nil
}
