package smtcore

// Source: src/smt/smt_case_split_queue.cpp activity-based case selection used
// by src/smt/smt_context.cpp decide().

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// SMTActivityCaseSplitQueue selects the retained search strategy. All
// variants use concrete context-owned queues/heaps and lifecycle callbacks;
// strategy 6 adds theory priorities.
type SMTActivityCaseSplitQueue struct {
	liveParams                            *SMTParams
	state                                 *SMTInternalizerState
	phaseSelection, cachingOn, cachingOff uint32
	randomVariableFrequency               float64
	strategy                              uint32
	relCaseSplitOrder                     uint32
	lookaheadDiseq                        bool
	phaseCounter                          uint32
	cacheEnabled, phaseDefault            bool
}

func NewSMTActivityCaseSplitQueue(state *SMTInternalizerState) (*SMTActivityCaseSplitQueue, error) {
	if state == nil || state.open() != nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTActivityCaseSplitQueue{state: state, phaseSelection: 3, cachingOn: 400, cachingOff: 100, cacheEnabled: true}, nil
}

func (q *SMTActivityCaseSplitQueue) ConfigurePhases(state SMTParamsState) error {
	if q == nil || q.state == nil || state.PhaseSelection > 6 {
		return ast.ErrASTDomain
	}
	if err := q.updateLivePhases(&state); err != nil {
		return err
	}
	q.strategy = effectiveSMTCaseSplitStrategy(state)
	q.phaseCounter = 0
	q.cacheEnabled = true
	q.phaseDefault = false
	if q.strategy == 1 {
		return q.state.ensureDelayedActivityQueue()
	}
	if q.strategy == 2 {
		return q.state.ensureCachedActivityQueue()
	}
	if q.strategy == 0 {
		return q.state.ensureBasicActivityQueue()
	}
	if q.strategy == 5 {
		return q.state.ensureRelevanceGoalQueue()
	}
	if q.strategy == 4 {
		return q.state.ensureRelevanceActivityQueue()
	}
	if q.strategy == 6 {
		return q.state.ensureTheoryAwareBranching()
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) updateLivePhases(state *SMTParamsState) error {
	if state.PhaseSelection > 6 {
		return ast.ErrASTDomain
	}
	q.phaseSelection = state.PhaseSelection
	q.cachingOn, q.cachingOff = state.PhaseCachingOn, state.PhaseCachingOff
	q.randomVariableFrequency = state.RandomVarFreq
	q.relCaseSplitOrder = state.RelCaseSplitOrder
	q.lookaheadDiseq = state.LookaheadDiseq
	return nil
}

func (q *SMTActivityCaseSplitQueue) RecordConflict() error {
	if err := q.refreshLivePhases(); err != nil {
		return err
	}
	q.phaseCounter++
	limit := q.cachingOff
	if q.cacheEnabled {
		limit = q.cachingOn
	}
	if q.phaseCounter < limit {
		return nil
	}
	q.phaseCounter = 0
	q.cacheEnabled = !q.cacheEnabled
	if q.phaseSelection == 4 {
		q.phaseDefault = !q.phaseDefault
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) PrepareConflict() error {
	if q == nil || q.state == nil {
		return ast.ErrASTHandle
	}
	if err := q.refreshLivePhases(); err != nil {
		return err
	}
	if q.phaseSelection != 3 && q.phaseSelection != 4 {
		return nil
	}
	start := 0
	if q.state.scopeLevel != 0 {
		if len(q.state.scopes) == 0 {
			return ast.ErrASTDomain
		}
		start = q.state.scopes[len(q.state.scopes)-1].assignedLimit
	}
	for _, literal := range q.state.assignedLiterals[start:] {
		q.state.booleanData[literal.Var()].PhaseAvailable = false
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) occurrencePhase(variable int32) bool {
	positive, negative := 0, 0
	for i := range q.state.clauses {
		record := &q.state.clauses[i]
		literals := record.Literals
		if record.clause != nil {
			literals = record.clause.literals
		}
		for _, literal := range literals {
			if literal.Var() != variable {
				continue
			}
			if literal.Sign() {
				negative++
			} else {
				positive++
			}
		}
	}
	return positive > negative
}

// NextCaseSplit returns the highest-activity unassigned Boolean variable and
// the source phase preference. Ties are deterministic in Go variable order.
func (q *SMTActivityCaseSplitQueue) NextCaseSplit() (int32, lbool.LBool, error) {
	if q == nil || q.state == nil {
		return SMTNullBoolVar, lbool.Undef, ast.ErrASTHandle
	}
	if err := q.state.open(); err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	if err := q.refreshLivePhases(); err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	next, preferred, err := q.selectVariable()
	if err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	if next == SMTNullBoolVar {
		return next, lbool.Undef, nil
	}
	if preferred != lbool.Undef {
		return next, preferred, nil
	}
	data := &q.state.booleanData[next]
	if data.IsQuantifier() {
		return next, lbool.False, nil
	}
	if data.TryTrueFirst() {
		return next, lbool.True, nil
	}
	phase := false
	switch q.phaseSelection {
	case 0:
	case 1:
		phase = true
	case 2, 3, 4:
		phase = q.phaseDefault
		if q.cacheEnabled && data.PhaseAvailable {
			phase = data.Phase
		}
	case 5:
		phase = q.state.random.Next()%2 == 0
	case 6:
		phase = q.occurrencePhase(next)
	default:
		return SMTNullBoolVar, lbool.Undef, ast.ErrASTDomain
	}
	if phase {
		return next, lbool.True, nil
	}
	return next, lbool.False, nil
}

func (q *SMTActivityCaseSplitQueue) selectVariable() (int32, lbool.LBool, error) {
	switch q.strategy {
	case 0, 2:
		return q.nextBasicActivity()
	case 1:
		return q.nextDelayedActivity()
	case 6:
		variable, err := q.state.theoryAwareBranching.next(q.randomVariableFrequency)
		return variable, lbool.Undef, err
	case 3:
		next, phase, err := q.nextRelevanceFIFO()
		if err == nil && next != SMTNullBoolVar && phase != lbool.False && q.lookaheadDiseq && q.selectedEqualityDisequal(next) {
			phase = lbool.False
		}
		return next, phase, err
	case 4:
		return q.nextRelevanceActivity()
	case 5:
		return q.nextRelevanceGoal()
	default:
		return q.nextBasicActivity()
	}
}

// The Go no-progress check needs availability, not a consumed decision.
// Strategies 4 and 6 remove selected heap entries in next_case_split.
func (q *SMTActivityCaseSplitQueue) hasCaseSplit() (bool, error) {
	if q == nil || q.state == nil {
		return false, ast.ErrASTHandle
	}
	if err := q.state.open(); err != nil {
		return false, err
	}
	if err := q.refreshLivePhases(); err != nil {
		return false, err
	}
	if q.strategy <= 2 {
		values, err := q.state.basicActivity.AppendValues(nil)
		if err != nil {
			return false, err
		}
		for _, variable := range values {
			value, err := q.state.Assignment(NewSMTLiteral(variable, false))
			if err != nil {
				return false, err
			}
			if value == lbool.Undef {
				return true, nil
			}
		}
		if q.strategy == 1 {
			values, err := q.state.delayedActivity.AppendValues(nil)
			if err != nil {
				return false, err
			}
			for _, v := range values {
				value, err := q.state.Assignment(NewSMTLiteral(v, false))
				if err != nil {
					return false, err
				}
				if value == lbool.Undef {
					return true, nil
				}
			}
		}
		// Random selection may reach an unassigned variable already erased
		// from the heap, without consuming the heap/RNG in this probe.
		if int(q.randomVariableFrequency*float64(z3rand.Max)) > 0 {
			for v := SMTFirstBoolVar; int(v) < len(q.state.boolExpr); v++ {
				value, err := q.state.Assignment(NewSMTLiteral(v, false))
				if err != nil {
					return false, err
				}
				if value == lbool.Undef {
					return true, nil
				}
			}
		}
		return false, nil
	}
	if q.strategy == 4 {
		// Native's unconditional 2% random branch can select any unassigned
		// variable, including one absent from the relevance/delayed queues.
		// Probe availability without consuming its RNG, queue head or heap.
		for variable := SMTFirstBoolVar; int(variable) < len(q.state.boolExpr); variable++ {
			value, err := q.state.Assignment(NewSMTLiteral(variable, false))
			if err != nil {
				return false, err
			}
			if value == lbool.Undef {
				return true, nil
			}
		}
		return false, nil
	}

	if q.strategy == 3 {
		f := &q.state.relevanceFIFO
		for _, entries := range [][]ast.ASTHandle{f.first[f.firstHead:], f.second[f.secondHead:]} {
			for _, expression := range entries {
				if ok, err := q.relevantCandidateAvailable(expression); err != nil || ok {
					return ok, err
				}
			}
		}
		return false, nil
	}
	if q.strategy == 5 {
		f := &q.state.relevanceGoal
		for _, expression := range f.first[f.head:] {
			if ok, err := q.relevantCandidateAvailable(expression); err != nil || ok {
				return ok, err
			}
		}
		indices, err := f.heap.AppendValues(nil)
		if err != nil {
			return false, err
		}
		for _, index := range indices {
			if ok, err := q.relevantCandidateAvailable(f.second[index].expression); err != nil || ok {
				return ok, err
			}
		}
		return false, nil
	}

	values, err := q.state.theoryAwareBranching.heap.AppendValues(nil)
	if err != nil {
		return false, err
	}
	for _, variable := range values {
		assignment, err := q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return false, err
		}
		if assignment == lbool.Undef {
			return true, nil
		}
	}
	return false, nil
}

func (q *SMTActivityCaseSplitQueue) selectedEqualityDisequal(variable int32) bool {
	if variable < SMTFirstBoolVar || int(variable) >= len(q.state.boolExpr) {
		return false
	}
	node := q.state.context.enodes[q.state.boolExpr[variable]]
	if node == nil || !node.IsEquality() || node.NumArgs() != 2 {
		return false
	}
	left, leftErr := node.Arg(0)
	right, rightErr := node.Arg(1)
	return leftErr == nil && rightErr == nil && q.extendedDisequal(left, right, 2)
}

func (q *SMTActivityCaseSplitQueue) extendedDisequal(first, second *SMTEnode, depth uint32) bool {
	if first == nil || second == nil || first.Root() == second.Root() {
		return false
	}
	firstRoot, secondRoot := first.Root(), second.Root()
	if firstRoot.IsInterpreted() && secondRoot.IsInterpreted() || q.state.context.isDisequal(firstRoot, secondRoot) {
		return true
	}
	if depth == 0 {
		return false
	}
	if firstRoot.NumParents() > secondRoot.NumParents() {
		firstRoot, secondRoot = secondRoot, firstRoot
	}
	for _, firstParent := range firstRoot.Parents() {
		if firstParent == nil || firstParent.IsEquality() || !firstParent.IsCongruenceRoot() ||
			q.state.relevancy != nil && !q.state.relevancy.IsRelevant(firstParent.Owner()) {
			continue
		}
		for _, secondParent := range secondRoot.Parents() {
			if secondParent == nil || secondParent.IsEquality() || !secondParent.IsCongruenceRoot() ||
				q.state.relevancy != nil && !q.state.relevancy.IsRelevant(secondParent.Owner()) ||
				firstParent.Root() == secondParent.Root() || firstParent.Declaration() != secondParent.Declaration() ||
				firstParent.NumArgs() != secondParent.NumArgs() {
				continue
			}
			almostCongruent := true
			for index := uint32(0); index < firstParent.NumArgs(); index++ {
				firstArg, firstErr := firstParent.Arg(index)
				secondArg, secondErr := secondParent.Arg(index)
				if firstErr != nil || secondErr != nil {
					almostCongruent = false
					break
				}
				firstArg, secondArg = firstArg.Root(), secondArg.Root()
				if firstArg == secondArg {
					continue
				}
				firstOriginal := firstArg == firstRoot || firstArg == secondRoot
				secondOriginal := secondArg == firstRoot || secondArg == secondRoot
				if !firstOriginal || !secondOriginal {
					almostCongruent = false
					break
				}
			}
			if almostCongruent && q.extendedDisequal(firstParent, secondParent, depth-1) {
				return true
			}
		}
	}
	return false
}

func (q *SMTActivityCaseSplitQueue) randomVariable(frequency float64) (int32, error) {
	threshold := int(frequency * float64(z3rand.Max))
	if int(q.state.random.Next()) >= threshold || len(q.state.boolExpr) == 0 {
		return SMTNullBoolVar, nil
	}
	candidate := int32(q.state.random.Next() % uint32(len(q.state.boolExpr)))
	value, err := q.state.Assignment(NewSMTLiteral(candidate, false))
	if err != nil {
		return SMTNullBoolVar, err
	}
	line := 118
	if q.strategy == 4 {
		line = 638
	}
	if err = q.state.traceRandomSplit(line, candidate, value); err != nil {
		return SMTNullBoolVar, err
	}
	if value != lbool.Undef {
		return SMTNullBoolVar, err
	}
	return candidate, nil
}

func (q *SMTActivityCaseSplitQueue) relevantCandidate(expression ast.ASTHandle) (int32, lbool.LBool, bool, error) {
	boolean, err := q.state.isBoolean(expression)
	if err != nil || !boolean {
		return SMTNullBoolVar, lbool.Undef, false, err
	}
	op, basic, err := q.state.basicOperation(expression)
	if err != nil {
		return SMTNullBoolVar, lbool.Undef, false, err
	}
	isOr, isAnd := basic && op == ast.ASTOpOr, basic && op == ast.ASTOpAnd
	variable, internalized := q.state.BoolVar(expression)
	internLine, orLine, varLine := q.candidateAssertionLines()
	if err := smtCaseSplitAssert(internLine, "intern || is_or", func() bool { return internalized || isOr }); err != nil {
		return SMTNullBoolVar, lbool.Undef, false, err
	}
	if !internalized && !isOr {
		return SMTNullBoolVar, lbool.Undef, false, nil
	}
	value := lbool.True
	if !internalized {
		if err := smtCaseSplitAssert(orLine, "is_or", func() bool { return isOr }); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
	}
	if internalized {
		value, err = q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
	}
	if isOr && value == lbool.True || isAnd && value == lbool.False {
		child, found, err := q.relevantChild(expression, value)
		if err != nil || !found {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		if err = q.traceDecideAndOr(expression, child); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		if err = q.traceCaseSplitCandidate(expression, child); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		literal, err := q.state.Literal(child)
		if err != nil {
			if err = q.state.Internalize(child, false); err != nil {
				return SMTNullBoolVar, lbool.Undef, false, err
			}
			literal, err = q.state.Literal(child)
		}
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		phase := lbool.True
		if literal.Sign() {
			phase = lbool.False
		}
		if err = q.traceCaseSplitQueue(true); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		return literal.Var(), phase, true, nil
	}
	if internalized && value == lbool.Undef {
		if err = smtCaseSplitAssert(varLine, "intern && m_context.get_bool_var(curr) == next", func() bool {
			checked, ok := q.state.BoolVar(expression)
			return internalized && ok && checked == variable
		}); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		if err = q.traceCaseSplitCandidate(expression, ast.ASTHandle{}); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		if err = q.traceCaseSplitQueue(false); err != nil {
			return SMTNullBoolVar, lbool.Undef, false, err
		}
		return variable, lbool.Undef, true, nil
	}
	return SMTNullBoolVar, lbool.Undef, false, nil
}

func (q *SMTActivityCaseSplitQueue) relevantChild(parent ast.ASTHandle, parentValue lbool.LBool) (ast.ASTHandle, bool, error) {
	count, err := q.state.manager.GetNumArgs(parent)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	undefined := make([]ast.ASTHandle, 0, count)
	for index := uint32(0); index < count; index++ {
		child, err := q.state.manager.GetArg(parent, index)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		literal, err := q.state.Literal(child)
		if err != nil {
			continue
		}
		value, err := q.state.Assignment(literal)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if value == parentValue {
			return ast.ASTHandle{}, false, nil
		}
		if value == lbool.Undef {
			if q.relCaseSplitOrder != 0 || len(undefined) == 0 {
				undefined = append(undefined, child)
			}
		}
	}
	if len(undefined) == 0 {
		return ast.ASTHandle{}, false, nil
	}
	if q.relCaseSplitOrder == 1 && len(undefined) > 1 {
		return undefined[q.state.random.Next()%uint32(len(undefined))], true, nil
	}
	return undefined[len(undefined)-1], true, nil
}

func (s *SMTInternalizerState) expressionGeneration(expression ast.ASTHandle) (uint32, error) {
	stack := []ast.ASTHandle{expression}
	seen := make(map[ast.ASTHandle]bool)
	var maximum uint32
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[current] {
			continue
		}
		seen[current] = true
		if node := s.context.enodes[current]; node != nil {
			if node.Generation() > maximum {
				maximum = node.Generation()
			}
			continue
		}
		kind, err := s.manager.GetKind(current)
		if err != nil {
			return 0, err
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := s.manager.GetNumArgs(current)
		if err != nil {
			return 0, err
		}
		for index := uint32(0); index < count; index++ {
			child, err := s.manager.GetArg(current, index)
			if err != nil {
				return 0, err
			}
			stack = append(stack, child)
		}
	}
	return maximum, nil
}

// Go's final-check progress guard is observational: it must not perform a
// source decision, advance a cursor/RNG, or emit decision trace records.
func (q *SMTActivityCaseSplitQueue) relevantCandidateAvailable(expression ast.ASTHandle) (bool, error) {
	op, basic, err := q.state.basicOperation(expression)
	if err != nil {
		return false, err
	}
	isOr, isAnd := basic && op == ast.ASTOpOr, basic && op == ast.ASTOpAnd
	variable, internalized := q.state.BoolVar(expression)
	if !internalized && !isOr {
		return false, nil
	}
	value := lbool.True
	if internalized {
		value, err = q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return false, err
		}
	}
	if isOr && value == lbool.True || isAnd && value == lbool.False {
		count, err := q.state.manager.GetNumArgs(expression)
		if err != nil {
			return false, err
		}
		undefined := false
		for i := uint32(0); i < count; i++ {
			child, err := q.state.manager.GetArg(expression, i)
			if err != nil {
				return false, err
			}
			literal, err := q.state.Literal(child)
			if err != nil {
				continue
			}
			childValue, err := q.state.Assignment(literal)
			if err != nil {
				return false, err
			}
			if childValue == value {
				return false, nil
			}
			undefined = undefined || childValue == lbool.Undef
		}
		return undefined, nil
	}
	return internalized && value == lbool.Undef, nil
}
