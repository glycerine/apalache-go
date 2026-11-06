package smtcore

// Source: theory_pb variable/literal watches, assign_watch, assign_ineq,
// assign_watch_ge and their scope trails. Numerals are concrete exact rationals.

import (
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTPseudoBooleanTheory) compileArgument(expression ast.ASTHandle) (SMTLiteral, error) {
	s := t.state
	m := s.manager
	negative := false
	if op, basic, err := s.basicOperation(expression); err != nil {
		return SMTNullLiteral, err
	} else if basic && op == ast.ASTOpNot {
		expression, err = m.GetArg(expression, 0)
		if err != nil {
			return SMTNullLiteral, err
		}
		negative = true
	}
	// Source strips exactly one leading negation before this guard, even
	// when the remaining expression already has a Boolean variable.
	if smtcoreSourceDebug {
		if op, basic, err := s.basicOperation(expression); err != nil {
			return SMTNullLiteral, err
		} else if basic && op == ast.ASTOpNot {
			return SMTNullLiteral, ast.ErrASTDomain
		}
	}
	if _, found := s.boolVar[expression]; !found {
		if err := s.Internalize(expression, false); err != nil {
			return SMTNullLiteral, err
		}
	}
	literal, err := s.Literal(expression)
	if err != nil {
		return SMTNullLiteral, err
	}
	if literal != SMTTrueLiteral && literal != SMTFalseLiteral {
		data, err := s.BooleanData(literal.Var())
		if err != nil {
			return SMTNullLiteral, err
		}
		family, err := m.GetFamilyID(expression)
		if err != nil {
			return SMTNullLiteral, err
		}
		if family == ast.NullFamilyID && !data.IsTheoryAtom() {
			if err = s.SetVariableTheory(literal.Var(), t.FamilyID()); err != nil {
				return SMTNullLiteral, err
			}
		}
		if data.Theory() != t.FamilyID() {
			proxy, err := m.MkFreshPseudoBooleanAux()
			if err != nil {
				return SMTNullLiteral, err
			}
			iff, err := m.MkIff(proxy, expression)
			if err != nil {
				return SMTNullLiteral, err
			}
			if err = t.emitPB("compile_arg", 644, func(out io.Writer) error {
				_, _ = io.WriteString(out, "create proxy ")
				// The source prints the IFF before it is internalized.
				return t.displayPBApplication(out, iff, true)
			}); err != nil {
				return SMTNullLiteral, err
			}
			if err = s.Internalize(iff, false); err != nil {
				return SMTNullLiteral, err
			}
			definition, err := s.Literal(iff)
			if err != nil {
				return SMTNullLiteral, err
			}
			if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{definition}); err != nil {
				return SMTNullLiteral, err
			}
			if err = s.markRelevant(proxy); err != nil {
				return SMTNullLiteral, err
			}
			literal, err = s.Literal(proxy)
			if err != nil {
				return SMTNullLiteral, err
			}
		}
	}
	if negative {
		literal = literal.Negated()
	}
	return literal, nil
}

type smtPBConstraint struct {
	literal                            SMTLiteral
	equality                           bool
	args                               [2]smtPBArgs
	negative, active                   bool
	watchSize                          int
	watchSum, maxWatch, minSum, maxSum big.Rat
	fixed                              int
	propagations                       uint32
	compilationThreshold               uint32
	compiled                           lbool.LBool
}

func (c *smtPBConstraint) current() *smtPBArgs {
	if c.negative {
		return &c.args[1]
	}
	return &c.args[0]
}
func (c *smtPBConstraint) lit() SMTLiteral {
	if c.negative {
		return c.literal.Negated()
	}
	return c.literal
}

type smtPBVariableWatches struct {
	literals   [2][]*smtPBConstraint
	variables  []*smtPBConstraint
	constraint *smtPBConstraint
}

func (t *SMTPseudoBooleanTheory) watchVariables() []int32 {
	var variables []int32
	for v := range t.watches {
		variables = append(variables, v)
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i] < variables[j] })
	return variables
}

// Source pb_justification retains the inequality used for propagation.
type smtPBJustification struct {
	*SMTTheoryPropagationJustification
	constraint *smtPBConstraint
}

func (t *SMTPseudoBooleanTheory) info(v int32) *smtPBVariableWatches {
	if t.watches == nil {
		t.watches = make(map[int32]*smtPBVariableWatches)
	}
	i := t.watches[v]
	if i == nil {
		i = &smtPBVariableWatches{}
		t.watches[v] = i
	}
	return i
}

func smtPBSign(l SMTLiteral) int {
	if l.Sign() {
		return 1
	}
	return 0
}

func smtPBRemove(list []*smtPBConstraint, c *smtPBConstraint) []*smtPBConstraint {
	for i, p := range list {
		if p == c {
			list[i] = list[len(list)-1]
			list[len(list)-1] = nil
			return list[:len(list)-1]
		}
	}
	return list
}

func (t *SMTPseudoBooleanTheory) clearWatch(c *smtPBConstraint) {
	for _, term := range c.current().terms {
		i := t.info(term.literal.Var())
		i.variables = smtPBRemove(i.variables, c)
		sign := smtPBSign(term.literal)
		i.literals[sign] = smtPBRemove(i.literals[sign], c)
	}
	c.watchSize = 0
	c.watchSum.SetInt64(0)
	c.maxWatch.SetInt64(0)
	c.minSum.SetInt64(0)
	c.maxSum.SetInt64(0)
	c.fixed = 0
}

func (t *SMTPseudoBooleanTheory) initVariableWatch(c *smtPBConstraint) {
	c.minSum.SetInt64(0)
	c.maxSum.SetInt64(0)
	c.fixed = 0
	for _, term := range c.current().terms {
		i := t.info(term.literal.Var())
		i.variables = append(i.variables, c)
		c.maxSum.Add(&c.maxSum, &term.coeff)
	}
}

func (t *SMTPseudoBooleanTheory) addWatch(c *smtPBConstraint, index int) {
	a := c.current()
	term := a.terms[index]
	c.watchSum.Add(&c.watchSum, &term.coeff)
	a.terms[index], a.terms[c.watchSize] = a.terms[c.watchSize], a.terms[index]
	c.watchSize++
	if term.coeff.Cmp(&c.maxWatch) > 0 {
		c.maxWatch.Set(&term.coeff)
	}
	i := t.info(term.literal.Var())
	sign := smtPBSign(term.literal)
	i.literals[sign] = append(i.literals[sign], c)
}

func (t *SMTPseudoBooleanTheory) initLiteralWatch(c *smtPBConstraint) error {
	if smtcoreSourceDebug && c.equality {
		return ast.ErrASTDomain
	}
	c.watchSize = 0
	c.watchSum.SetInt64(0)
	c.maxWatch.SetInt64(0)
	for i := 0; i < len(c.current().terms); i++ {
		value, err := t.state.Assignment(c.current().terms[i].literal)
		if err != nil {
			return err
		}
		if value != lbool.False {
			t.addWatch(c, i)
			if c.watchSum.Cmp(new(big.Rat).Add(&c.current().bound, &c.maxWatch)) >= 0 {
				break
			}
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) helpful(c *smtPBConstraint, negate bool) ([]SMTLiteral, error) {
	var out []SMTLiteral
	sum := new(big.Rat)
	for _, term := range c.current().terms {
		if sum.Cmp(&c.current().bound) >= 0 {
			break
		}
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v == lbool.True {
			l := term.literal
			if negate {
				l = l.Negated()
			}
			out = append(out, l)
			sum.Add(sum, &term.coeff)
		}
	}
	if smtcoreSourceDebug && sum.Cmp(&c.current().bound) < 0 {
		return nil, ast.ErrASTDomain
	}
	return out, nil
}

func (t *SMTPseudoBooleanTheory) unhelpful(c *smtPBConstraint, negate bool) ([]SMTLiteral, error) {
	var out []SMTLiteral
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v == lbool.False {
			l := term.literal
			if negate {
				l = l.Negated()
			}
			out = append(out, l)
		}
	}
	return out, nil
}

func (t *SMTPseudoBooleanTheory) allLiterals(c *smtPBConstraint) ([]SMTLiteral, error) {
	var out []SMTLiteral
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v != lbool.Undef {
			l := term.literal
			if v == lbool.False {
				l = l.Negated()
			}
			out = append(out, l)
		}
	}
	return out, nil
}

func (t *SMTPseudoBooleanTheory) addAssign(c *smtPBConstraint, premises []SMTLiteral, consequent SMTLiteral) error {
	t.incPropagations(c)
	t.statistics.propagations++
	if err := t.traceReason("add_assign", 1431, c, premises, consequent); err != nil {
		return err
	}
	base, err := NewSMTTheoryPropagationJustification(t.state.manager, t.FamilyID(), premises, consequent, nil)
	if err != nil {
		return err
	}
	owner := &smtPBJustification{SMTTheoryPropagationJustification: base, constraint: c}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	return t.state.AssignLiteral(consequent, wrapped)
}

func (t *SMTPseudoBooleanTheory) addConflict(c *smtPBConstraint, clause []SMTLiteral) error {
	t.incPropagations(c)
	t.statistics.conflicts++
	t.statistics.clauses++
	if err := t.traceReason("add_clause", 1454, c, clause, SMTNullLiteral); err != nil {
		return err
	}
	frequency := t.controls.ConflictFrequency
	if frequency == 0 || frequency-1 == c.propagations%frequency {
		if _, err := t.resolveConflict(c); err != nil {
			return err
		}
	}
	var owner SMTJustification
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		owner, err = NewSMTTheoryLemmaJustification(t.state.manager, t.FamilyID(), clause, t.state.boolExpr, nil)
		if err != nil {
			return err
		}
	}
	// Source conflict clauses are auxiliary lemmas, whose false literals
	// remain available to resolution even without proof production.
	if err = t.traceClause("add_clause", 1469, clause); err != nil {
		return err
	}
	return t.state.addClauseKindWithJustificationAndDeleteHandler(clause, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
}

func (t *SMTPseudoBooleanTheory) assignVariable(v int32, isTrue bool, c *smtPBConstraint) error {
	asgn, err := t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	a := c.current()
	if c.maxSum.Cmp(&a.bound) < 0 && asgn == lbool.False || !c.equality && c.minSum.Cmp(&a.bound) >= 0 && asgn == lbool.True {
		return nil
	}
	index := -1
	for i, term := range a.terms {
		if term.literal.Var() == v {
			index = i
			break
		}
	}
	if err = t.emitPB("assign_watch", 1018, func(out io.Writer) error {
		_, _ = io.WriteString(out, "assign watch ")
		if err := smtPBWriteLiteral(out, NewSMTLiteral(v, !isTrue)); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " ")
		return t.displayInequality(out, c, true)
	}); err != nil {
		return err
	}
	if index < 0 {
		return ast.ErrASTDomain
	}
	oldMin, oldMax, oldFixed := new(big.Rat).Set(&c.minSum), new(big.Rat).Set(&c.maxSum), c.fixed
	_ = t.trail.PushUndo(func() { c.minSum.Set(oldMin); c.maxSum.Set(oldMax); c.fixed = oldFixed })
	if a.terms[index].literal.Sign() == isTrue {
		c.maxSum.Sub(&c.maxSum, &a.terms[index].coeff)
	} else {
		c.minSum.Add(&c.minSum, &a.terms[index].coeff)
	}
	if err = t.validateVariableSums(c); err != nil {
		return err
	}
	c.fixed++
	if smtcoreSourceDebug && c.fixed > len(a.terms) {
		return ast.ErrASTDomain
	}
	if !c.equality && c.minSum.Cmp(&a.bound) >= 0 && asgn != lbool.True {
		if err = t.traceAssignment("assign_watch", 1047, "Set ", c.lit(), c, true); err != nil {
			return err
		}
		ls, err := t.helpful(c, false)
		if err != nil {
			return err
		}
		return t.addAssign(c, ls, c.lit())
	}
	if c.maxSum.Cmp(&a.bound) < 0 && asgn != lbool.False {
		if err = t.traceAssignment("assign_watch", 1051, "Set ", c.lit().Negated(), c, true); err != nil {
			return err
		}
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return err
		}
		return t.addAssign(c, ls, c.lit().Negated())
	}
	if c.equality && c.fixed == len(a.terms) {
		holds := c.minSum.Cmp(&a.bound) == 0
		if holds && asgn != lbool.True || !holds && asgn != lbool.False {
			line := 1055
			literal := c.lit()
			if !holds {
				line = 1059
				literal = literal.Negated()
			}
			if err = t.traceAssignment("assign_watch", line, "Set ", literal, c, true); err != nil {
				return err
			}
			ls, err := t.allLiterals(c)
			if err != nil {
				return err
			}
			l := c.lit()
			if !holds {
				l = l.Negated()
			}
			return t.addAssign(c, ls, l)
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) assignInequality(c *smtPBConstraint, isTrue bool) error {
	if c.equality || c.active {
		return nil
	}
	oldMin, oldMax, oldFixed := new(big.Rat).Set(&c.minSum), new(big.Rat).Set(&c.maxSum), c.fixed
	oldNegative := c.negative
	_ = t.trail.PushUndo(func() {
		t.clearWatch(c)
		c.negative = oldNegative
		c.active = false
		t.initVariableWatch(c)
		c.minSum.Set(oldMin)
		c.maxSum.Set(oldMax)
		c.fixed = oldFixed
	})
	t.clearWatch(c)
	c.negative = !isTrue
	c.active = true
	maxsum, mininc := new(big.Rat), new(big.Rat)
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if v != lbool.False {
			maxsum.Add(maxsum, &term.coeff)
		}
		if v == lbool.Undef && (mininc.Sign() == 0 || mininc.Cmp(&term.coeff) > 0) {
			mininc.Set(&term.coeff)
		}
	}
	if err := t.traceAssignment("assign_ineq", 949, "assign: ", c.lit(), c, false); err != nil {
		return err
	}
	if maxsum.Cmp(&c.current().bound) < 0 {
		ls, err := t.unhelpful(c, false)
		if err != nil {
			return err
		}
		return t.addConflict(c, append(ls, c.lit().Negated()))
	}
	if err := t.initLiteralWatch(c); err != nil {
		return err
	}
	if err := t.validateWatch(c); err != nil {
		return err
	}
	if new(big.Rat).Sub(maxsum, mininc).Cmp(&c.current().bound) < 0 {
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return err
		}
		ls = append(ls, c.lit())
		for _, term := range c.current().terms {
			v, err := t.state.Assignment(term.literal)
			if err != nil {
				return err
			}
			if v == lbool.Undef {
				if err = t.validateAssign(c, ls, term.literal); err != nil {
					return err
				}
				if err = t.addAssign(c, ls, term.literal); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) assignLiteralWatch(v int32, isTrue bool, c *smtPBConstraint, watchIndex int) (bool, error) {
	a := c.current()
	if smtcoreSourceDebug && (c.equality || c.watchSize < 0 || c.watchSize > len(a.terms)) {
		return false, ast.ErrASTDomain
	}
	w := -1
	for i := 0; i < c.watchSize; i++ {
		if a.terms[i].literal.Var() == v {
			w = i
			break
		}
	}
	if w < 0 {
		return false, ast.ErrASTDomain
	}
	if smtcoreSourceDebug {
		value, err := t.state.Assignment(c.lit())
		if err != nil {
			return false, err
		}
		if value != lbool.True || isTrue != a.terms[w].literal.Sign() {
			return false, ast.ErrASTDomain
		}
	}
	kCoeff := new(big.Rat).Add(&a.bound, &a.terms[w].coeff)
	for i := c.watchSize; c.watchSum.Cmp(new(big.Rat).Add(kCoeff, &c.maxWatch)) < 0 && i < len(a.terms); i++ {
		value, err := t.state.Assignment(a.terms[i].literal)
		if err != nil {
			return false, err
		}
		if value != lbool.False {
			t.addWatch(c, i)
		}
	}
	if c.watchSum.Cmp(kCoeff) < 0 {
		ls, err := t.unhelpful(c, false)
		if err != nil {
			return false, err
		}
		if err = t.addConflict(c, append(ls, c.lit().Negated())); err != nil {
			return false, err
		}
		return false, t.traceAssignment("assign_watch_ge", 1157, "assign: ", NewSMTLiteral(v, !isTrue), c, false)
	}
	term := a.terms[w]
	info := t.info(v)
	sign := 0
	if isTrue {
		sign = 1
	}
	list := info.literals[sign]
	list[watchIndex] = list[len(list)-1]
	list[len(list)-1] = nil
	info.literals[sign] = list[:len(list)-1]
	a.terms[w], a.terms[c.watchSize-1] = a.terms[c.watchSize-1], a.terms[w]
	c.watchSize--
	c.watchSum.Sub(&c.watchSum, &term.coeff)
	if term.coeff.Cmp(&c.maxWatch) == 0 {
		c.maxWatch.SetInt64(0)
		for i := 0; i < c.watchSize; i++ {
			if a.terms[i].coeff.Cmp(&c.maxWatch) > 0 {
				c.maxWatch.Set(&a.terms[i].coeff)
			}
		}
	}
	if smtcoreSourceDebug && c.watchSum.Cmp(&a.bound) < 0 {
		return true, ast.ErrASTDomain
	}
	if c.watchSum.Cmp(new(big.Rat).Add(&a.bound, &c.maxWatch)) < 0 {
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return true, err
		}
		ls = append(ls, c.lit())
		deficit := new(big.Rat).Sub(&c.watchSum, &a.bound)
		for _, term := range a.terms {
			value, err := t.state.Assignment(term.literal)
			if err != nil {
				return true, err
			}
			if value == lbool.Undef && deficit.Cmp(&term.coeff) < 0 {
				if err = t.validateAssign(c, ls, term.literal); err != nil {
					return true, err
				}
				if err = t.addAssign(c, ls, term.literal); err != nil {
					return true, err
				}
			}
		}
	}
	if err := t.traceAssignment("assign_watch_ge", 1157, "assign: ", NewSMTLiteral(v, !isTrue), c, false); err != nil {
		return true, err
	}
	return true, nil
}

func (t *SMTPseudoBooleanTheory) assignNative(v int32, isTrue bool) error {
	i := t.info(v)
	if err := t.traceAssignment("assign_eh", 785, "assign: ", NewSMTLiteral(v, !isTrue), nil, false); err != nil {
		return err
	}
	sign := 0
	if isTrue {
		sign = 1
	}
	if t.controls.EnableSimplex && i.literals[sign] != nil {
		ok, err := t.simplexAssignArgument(v, isTrue)
		if err != nil || !ok {
			return err
		}
	}
	for n := 0; n < len(i.literals[sign]); {
		removed, err := t.assignLiteralWatch(v, isTrue, i.literals[sign][n], n)
		if err != nil {
			return err
		}
		if !removed {
			n++
		}
	}
	for _, c := range i.variables {
		if err := t.assignVariable(v, isTrue, c); err != nil {
			return err
		}
	}
	if i.constraint != nil {
		if t.controls.EnableSimplex {
			ok, err := t.simplexAssignPredicate(i.constraint, isTrue)
			if err != nil || !ok {
				return err
			}
		}
		return t.assignInequality(i.constraint, isTrue)
	}
	return nil
}
