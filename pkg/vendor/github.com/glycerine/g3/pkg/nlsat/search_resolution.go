package nlsat

// Source: src/nlsat/nlsat_solver.cpp:1426-1800.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// searchResolution is the concrete conflict-resolution scratch state. It owns
// lemma literal references and borrows one live search state.
type searchResolution struct {
	search           *searchState
	marks            []bool
	numMarks         uint32
	lemma            scopedLiteralVector
	lazy             scopedLiteralVector
	lemmaAssumptions *AssumptionSet
}

func newSearchResolution(r *searchState) (*searchResolution, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	q := &searchResolution{search: r}
	q.lemma.factory = r.store.atoms
	q.lazy.factory = r.store.atoms
	return q, nil
}

func (q *searchResolution) close() error {
	if q == nil {
		return nil
	}
	if err := q.resetLemmaAssumptions(); err != nil {
		return err
	}
	if err := q.lemma.close(); err != nil {
		return err
	}
	if err := q.lazy.close(); err != nil {
		return err
	}
	q.search = nil
	q.marks = nil
	q.numMarks = 0
	return nil
}

func (q *searchResolution) resetLemmaAssumptions() error {
	if q == nil || q.lemmaAssumptions == nil {
		return nil
	}
	if err := q.search.store.assumptions.decRef(q.lemmaAssumptions); err != nil {
		return err
	}
	q.lemmaAssumptions = nil
	return nil
}

func (q *searchResolution) joinAssumptions(incoming *AssumptionSet) error {
	joined, err := q.search.store.assumptions.mkJoin(incoming, q.lemmaAssumptions)
	if err != nil || joined == q.lemmaAssumptions {
		return err
	}
	if err = q.search.store.assumptions.incRef(joined); err != nil {
		return err
	}
	old := q.lemmaAssumptions
	q.lemmaAssumptions = joined
	if old != nil {
		return q.search.store.assumptions.decRef(old)
	}
	return nil
}

func (q *searchResolution) buildLazyClause(j *LazyJustification, explanation *explanationAssumptions) (err error) {
	if j == nil || explanation == nil || explanation.search != q.search {
		return ErrSearchOwner
	}
	literals, err := j.Literals()
	if err != nil {
		return err
	}
	if err = q.lazy.reset(); err != nil {
		return err
	}
	if err = explanation.reset(); err != nil {
		return err
	}
	if err = explanation.collectExplanation(literals, explanation.fullDimensional); err != nil {
		return err
	}
	for _, literal := range explanation.output.result {
		if err = q.lazy.push(literal); err != nil {
			return err
		}
	}
	if err = explanation.reset(); err != nil {
		return err
	}
	for _, literal := range literals {
		if err = q.lazy.push(literal.Negated()); err != nil {
			return err
		}
	}
	return nil
}

func (q *searchResolution) clearAnalysis() error {
	for i := range q.marks {
		q.marks[i] = false
	}
	q.numMarks = 0
	return errors.Join(q.lemma.reset(), q.resetLemmaAssumptions())
}

// analyze resolves one conflict backwards through the live trail. The returned
// lemma borrows its literals through q until the publication phase consumes it.
func (q *searchResolution) analyze(conflict *Clause, explanation *explanationAssumptions) (foundDecision bool, err error) {
	if q == nil || q.search == nil || conflict == nil {
		return false, ErrSearchOwner
	}
	if q.numMarks != 0 {
		return false, ErrSearchPending
	}
	for _, marked := range q.marks {
		if marked {
			return false, ErrSearchPending
		}
	}
	if err = q.lemma.reset(); err != nil {
		return false, err
	}
	if err = q.resetLemmaAssumptions(); err != nil {
		return false, err
	}
	abort := func(cause error) (bool, error) {
		if cleanup := q.clearAnalysis(); cleanup != nil {
			return false, errors.Join(cause, cleanup)
		}
		return false, cause
	}
	if err = q.resolveLiterals(sat.NullBoolVar, conflict.Literals()); err != nil {
		return abort(err)
	}
	if err = q.joinAssumptions(conflict.Assumptions()); err != nil {
		return abort(err)
	}

	top := len(q.search.trail)
	for {
		foundDecision = false
		for q.numMarks > 0 {
			if top == 0 {
				return abort(arena.ErrBounds)
			}
			top--
			t := q.search.trail[top]
			if t.kind == trailStage {
				return abort(arena.ErrConfig)
			}
			if t.kind != trailBoolean || !q.marked(t.b) {
				continue
			}
			q.numMarks--
			if err = q.resetMark(t.b); err != nil {
				return abort(err)
			}
			if uint64(t.b) >= uint64(len(q.search.store.atoms.bools.justifications)) {
				return abort(arena.ErrBounds)
			}
			justification := q.search.store.atoms.bools.justifications[t.b]
			switch justification.Kind() {
			case ClauseJustification:
				clause, cause := justification.Clause()
				if cause != nil {
					return abort(cause)
				}
				if cause = q.resolveLiterals(t.b, clause.Literals()); cause != nil {
					return abort(cause)
				}
				if cause = q.joinAssumptions(clause.Assumptions()); cause != nil {
					return abort(cause)
				}
			case LazyJustificationKind:
				lazy, cause := justification.Lazy()
				if cause != nil {
					return abort(cause)
				}
				if cause = q.buildLazyClause(lazy, explanation); cause != nil {
					return abort(cause)
				}
				if cause = q.resolveLiterals(t.b, q.lazy.result); cause != nil {
					return abort(cause)
				}
			case DecisionJustification:
				if q.numMarks != 0 || uint64(t.b) >= uint64(len(q.search.store.atoms.bools.values)) {
					return abort(arena.ErrConfig)
				}
				foundDecision = true
				literal := sat.MakeLiteral(t.b, q.search.store.atoms.bools.values[t.b] == lbool.True)
				if cause := q.lemma.push(literal); cause != nil {
					return abort(cause)
				}
			default:
				return abort(arena.ErrConfig)
			}
		}

		if foundDecision {
			break
		}
		previous, cause := q.onlyPreviousStages(q.lemma.result)
		if cause != nil {
			return abort(cause)
		}
		if previous {
			break
		}
		level, cause := q.maxScopeLevel(q.lemma.result)
		if cause != nil {
			return abort(cause)
		}
		if level >= q.search.level {
			return abort(arena.ErrConfig)
		}
		if cause = q.removeLiteralsFromLevel(&q.lemma, level); cause != nil {
			return abort(cause)
		}
		if cause = q.search.undoUntilLevel(level); cause != nil {
			return abort(cause)
		}
		top = len(q.search.trail)
	}

	if err = q.resetMarks(); err != nil {
		return abort(err)
	}
	q.numMarks = 0
	return foundDecision, nil
}

// resolveConflict publishes analyzed lemmas and repeats when a learned clause
// immediately exposes another conflict. False means the empty lemma was derived.
func (q *searchResolution) resolveConflict(conflict *Clause, explanation *explanationAssumptions) (bool, error) {
	if q == nil || q.search == nil || conflict == nil {
		return false, ErrSearchOwner
	}
	for {
		q.search.conflicts++
		foundDecision, err := q.analyze(conflict, explanation)
		if err != nil {
			return false, err
		}
		if len(q.lemma.result) == 0 {
			return false, nil
		}

		if !foundDecision {
			stage, err := q.search.store.maxVar(q.lemma.result)
			if err != nil {
				return false, err
			}
			if err = q.search.undoUntilStage(stage); err != nil {
				return false, err
			}
			if q.search.xk != stage {
				return false, arena.ErrConfig
			}
		} else {
			if q.search.level == 0 {
				return false, arena.ErrConfig
			}
			boolean, err := q.isBooleanLemma(q.lemma.result)
			if err != nil {
				return false, err
			}
			if boolean {
				last := q.lemma.result[len(q.lemma.result)-1].Variable()
				if err = q.search.undoUntilUnassigned(last); err != nil {
					return false, err
				}
			} else {
				level, err := q.findNewArithmeticLevel(q.lemma.result)
				if err != nil {
					return false, err
				}
				if err = q.search.undoUntilLevel(level); err != nil {
					return false, err
				}
			}
		}

		learned, err := q.search.store.mkClause(q.lemma.result, true, q.lemmaAssumptions)
		if err != nil {
			return false, err
		}
		solved, err := q.search.processClause(learned, true)
		if err != nil {
			return false, err
		}
		if solved {
			return true, nil
		}
		conflict = learned
	}
}

func (q *searchResolution) marked(b sat.BoolVar) bool {
	return uint64(b) < uint64(len(q.marks)) && q.marks[b]
}

func (q *searchResolution) mark(b sat.BoolVar) {
	if uint64(b) >= uint64(len(q.marks)) {
		q.marks = append(q.marks, make([]bool, int(uint64(b)+1)-len(q.marks))...)
	}
	q.marks[b] = true
}

func (q *searchResolution) resetMark(b sat.BoolVar) error {
	if uint64(b) >= uint64(len(q.marks)) {
		return arena.ErrBounds
	}
	q.marks[b] = false
	return nil
}

func (q *searchResolution) resetMarks() error {
	for _, literal := range q.lemma.result {
		if err := q.resetMark(literal.Variable()); err != nil {
			return err
		}
	}
	return nil
}

func (q *searchResolution) literalMaxVariable(l sat.Literal) (uint32, error) {
	a, err := q.search.store.literalAtom(l)
	if err != nil || a == nil {
		return NullVariable, err
	}
	return arithmeticAtomHeader(a).maxVar, nil
}

func (q *searchResolution) processAntecedent(antecedent sat.Literal) error {
	r := q.search
	b := antecedent.Variable()
	assigned, err := r.assignedValue(antecedent)
	if err != nil {
		return err
	}
	if assigned == lbool.Undef {
		value, err := r.valueCore(antecedent)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_solver.cpp", 1458, "value(antecedent) == l_false", value == lbool.False); err != nil {
			return err
		}
		if q.marked(b) {
			return nil
		}
		x, err := q.literalMaxVariable(antecedent)
		if err != nil {
			return err
		}
		valid := x != NullVariable && r.xk != NullVariable && x < r.xk
		if err = atomAssert("nlsat_solver.cpp", 1460, "is_arith_atom(b) && max_var(b) < m_xk", valid); err != nil {
			return err
		}
		if !valid {
			return arena.ErrConfig
		}
		q.mark(b)
		return q.lemma.push(antecedent)
	}
	if uint64(b) >= uint64(len(r.store.atoms.bools.levels)) {
		return arena.ErrBounds
	}
	if q.marked(b) {
		return nil
	}
	q.mark(b)
	x, err := q.literalMaxVariable(antecedent)
	if err != nil {
		return err
	}
	if r.store.atoms.bools.levels[b] == r.level && x == r.xk {
		q.numMarks++
		return nil
	}
	return q.lemma.push(antecedent)
}

func (q *searchResolution) resolveLiterals(skip sat.BoolVar, literals []sat.Literal) error {
	for _, literal := range literals {
		if literal.Variable() != skip {
			if err := q.processAntecedent(literal); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *searchResolution) onlyPreviousStages(literals []sat.Literal) (bool, error) {
	for _, literal := range literals {
		x, err := q.literalMaxVariable(literal)
		if err != nil {
			return false, err
		}
		if x == q.search.xk {
			return false, nil
		}
	}
	return true, nil
}

func (q *searchResolution) maxScopeLevel(literals []sat.Literal) (uint32, error) {
	var max uint32
	for _, literal := range literals {
		value, err := q.search.valueCore(literal)
		if err != nil {
			return 0, err
		}
		if err = atomAssert("nlsat_solver.cpp", 1565, "value(ls[i]) == l_false", value == lbool.False); err != nil {
			return 0, err
		}
		assigned, err := q.search.assignedValue(literal)
		if err != nil {
			return 0, err
		}
		if assigned == lbool.False {
			b := literal.Variable()
			if uint64(b) >= uint64(len(q.search.store.atoms.bools.levels)) {
				return 0, arena.ErrBounds
			}
			if level := q.search.store.atoms.bools.levels[b]; level > max {
				max = level
			}
			continue
		}
		x, err := q.literalMaxVariable(literal)
		if err != nil {
			return 0, err
		}
		valid := assigned == lbool.Undef && x != NullVariable && q.search.xk != NullVariable && x < q.search.xk
		if !valid {
			return 0, arena.ErrConfig
		}
	}
	return max, nil
}

func (q *searchResolution) removeLiteralsFromLevel(lemma *scopedLiteralVector, level uint32) error {
	if lemma == nil || lemma.factory != q.search.store.atoms {
		return ErrSearchOwner
	}
	size := len(lemma.result)
	j := uint32(0)
	for i := 0; i < size; i++ {
		literal := lemma.result[i]
		b := literal.Variable()
		marked := q.marked(b)
		if err := atomAssert("nlsat_solver.cpp", 1594, "is_marked(b)", marked); err != nil {
			return err
		}
		if !marked {
			return arena.ErrConfig
		}
		value, err := q.search.valueCore(literal)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_solver.cpp", 1595, "value(lemma[i]) == l_false", value == lbool.False); err != nil {
			return err
		}
		if value != lbool.False {
			return arena.ErrConfig
		}
		assigned, err := q.search.assignedValue(literal)
		if err != nil {
			return err
		}
		x, err := q.literalMaxVariable(literal)
		if err != nil {
			return err
		}
		if assigned == lbool.False && q.search.store.atoms.bools.levels[b] == level && x == q.search.xk {
			q.numMarks++
			continue
		}
		if err = lemma.set(j, literal); err != nil {
			return err
		}
		j++
	}
	return lemma.shrink(j)
}

func (q *searchResolution) isBooleanLemma(literals []sat.Literal) (bool, error) {
	for _, literal := range literals {
		a, err := q.search.store.literalAtom(literal)
		if err != nil {
			return false, err
		}
		if a != nil {
			return false, nil
		}
	}
	return true, nil
}

func (q *searchResolution) findNewArithmeticLevel(literals []sat.Literal) (uint32, error) {
	boolean, err := q.isBooleanLemma(literals)
	if err != nil {
		return 0, err
	}
	if err = atomAssert("nlsat_solver.cpp", 1624, "!is_bool_lemma(sz, lemma)", !boolean); err != nil {
		return 0, err
	}
	if boolean || len(literals) == 0 {
		return 0, arena.ErrConfig
	}
	var level uint32
	found := false
	for _, literal := range literals[:len(literals)-1] {
		x, err := q.literalMaxVariable(literal)
		if err != nil {
			return 0, err
		}
		if x == q.search.xk {
			b := literal.Variable()
			if uint64(b) >= uint64(len(q.search.store.atoms.bools.levels)) {
				return 0, arena.ErrBounds
			}
			candidate := q.search.store.atoms.bools.levels[b]
			if !found || candidate > level {
				level = candidate
				found = true
			}
		}
	}
	valid := !found || level < q.search.level
	if err = atomAssert("nlsat_solver.cpp", 1641, "!found_lvl || new_lvl < scope_lvl()", valid); err != nil {
		return 0, err
	}
	if !valid {
		return 0, arena.ErrConfig
	}
	if !found {
		if q.search.level == 0 {
			return 0, arena.ErrBounds
		}
		level = q.search.level - 1
	}
	return level, nil
}
