package qe

// Source: src/qe/nlqsat.cpp:80-102,178-223,312-423.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/nlsat"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type nlqSearch struct {
	self            *nlqSearch
	solver          *nlsat.Solver
	state           *nlqState
	isTrue          sat.Literal
	boundReal       [][]uint32
	boundBoolean    [][]sat.BoolVar
	frontendBoolean map[sat.BoolVar]bool
	mode            qsatMode
	manager         *ast.ASTManager
	converter       *nlsattactic.GoalConverter
	answerSimplify  *astrewriter.ExprSafeReplace
	assumptionForms map[sat.BoolVar]ast.ASTRef
	answers         []ast.ASTRef
	extra           []sat.Literal
	first, second   *nlsat.LiteralVector
	rounds          uint32
	closed          bool
}

func newNLQSearch(solver *nlsat.Solver, isTrue sat.Literal, realLevels []MaxLevel, boundReal [][]uint32, boundBoolean [][]sat.BoolVar, frontendBoolean map[sat.BoolVar]bool) (search *nlqSearch, err error) {
	return newNLQSearchMode(solver, isTrue, realLevels, boundReal, boundBoolean, frontendBoolean, qsatSAT, nil, nil)
}

func newNLQSearchMode(solver *nlsat.Solver, isTrue sat.Literal, realLevels []MaxLevel, boundReal [][]uint32, boundBoolean [][]sat.BoolVar, frontendBoolean map[sat.BoolVar]bool, mode qsatMode, manager *ast.ASTManager, converter *nlsattactic.GoalConverter) (search *nlqSearch, err error) {
	if solver == nil {
		return nil, arena.ErrConfig
	}
	if mode != qsatSAT && mode != qsatQE || mode == qsatQE && (manager == nil || converter == nil) {
		return nil, arena.ErrConfig
	}
	s := &nlqSearch{
		solver: solver, isTrue: isTrue,
		boundReal: append([][]uint32(nil), boundReal...), boundBoolean: append([][]sat.BoolVar(nil), boundBoolean...),
		frontendBoolean: make(map[sat.BoolVar]bool, len(frontendBoolean)),
		mode:            mode, manager: manager, converter: converter, assumptionForms: make(map[sat.BoolVar]ast.ASTRef),
	}
	s.self = s
	for variable, present := range frontendBoolean {
		s.frontendBoolean[variable] = present
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.close())
		}
	}()
	s.state, err = newNLQState(solver, realLevels)
	if err != nil {
		return s, err
	}
	if mode == qsatQE {
		s.answerSimplify, err = astrewriter.NewExprSafeReplace(manager)
		if err != nil {
			return s, err
		}
	}
	for level, variables := range boundBoolean {
		value := EmptyMaxLevel()
		if level&1 == 0 {
			value.Existential = uint32(level)
		} else {
			value.Universal = uint32(level)
		}
		for _, variable := range variables {
			if err = s.state.setLevel(variable, value); err != nil {
				return s, err
			}
		}
	}
	s.first, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return s, err
	}
	s.second, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return s, err
	}
	return s, nil
}

func (s *nlqSearch) open() error {
	if s == nil || s.self != s || s.solver == nil || s.state == nil {
		return arena.ErrConfig
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.state.open()
}

func (s *nlqSearch) extractProjection(level uint32) (variables []uint32, freeBoolean map[sat.BoolVar]bool) {
	freeBoolean = make(map[sat.BoolVar]bool)
	for i, block := range s.boundReal {
		if uint32(i) >= level {
			variables = append(variables, block...)
		}
	}
	for i, block := range s.boundBoolean {
		if uint32(i) < level {
			for _, variable := range block {
				freeBoolean[variable] = true
			}
		}
	}
	return variables, freeBoolean
}

func (s *nlqSearch) projectCube(core []sat.Literal, projectionLevel uint32) (level MaxLevel, clause []sat.Literal, err error) {
	if err = s.state.restoreModel(); err != nil {
		return level, nil, fmt.Errorf("restore projection model: %w", err)
	}
	if err = s.first.Reset(); err != nil {
		return level, nil, err
	}
	if err = s.second.Reset(); err != nil {
		return level, nil, err
	}
	variables, freeBoolean := s.extractProjection(projectionLevel)
	for _, literal := range core {
		variable := literal.Variable()
		if !s.frontendBoolean[variable] || freeBoolean[variable] {
			if err = s.first.Push(literal); err != nil {
				return level, nil, err
			}
		}
	}
	current, next := s.first, s.second
	var values []sat.Literal
	for i := len(variables); i > 0; i-- {
		if err = current.Literals(&values); err != nil {
			return level, nil, err
		}
		for _, literal := range values {
			value, cause := s.solver.LiteralValue(literal)
			if cause != nil {
				return level, nil, fmt.Errorf("read projection literal %v: %w", literal, cause)
			}
			if value != lbool.True {
				return level, nil, fmt.Errorf("projection literal %v has model value %v: %w", literal, value, arena.ErrConfig)
			}
		}
		if err = s.solver.Project(variables[i-1], values, next); err != nil {
			return level, nil, fmt.Errorf("project real variable %d: %w", variables[i-1], err)
		}
		current, next = next, current
	}
	if err = current.Literals(&values); err != nil {
		return level, nil, err
	}
	level = EmptyMaxLevel()
	clause = make([]sat.Literal, len(values), len(values)+1)
	for i, literal := range values {
		clause[i] = literal.Negated()
		literalLevel, cause := s.state.getLevel(clause[i])
		if cause != nil {
			return level, nil, fmt.Errorf("read projected literal level: %w", cause)
		}
		level.Merge(literalLevel)
	}
	return level, clause, nil
}

func (s *nlqSearch) enforceParity(clause []sat.Literal) []sat.Literal {
	if nlqExists(s.state.level()) {
		clause = append(clause, s.isTrue.Negated())
	} else {
		clause = append(clause, s.isTrue)
	}
	return clause
}

func (s *nlqSearch) projectCore(core []sat.Literal) (level MaxLevel, clause []sat.Literal, err error) {
	level, clause, err = s.projectCube(core, s.state.level()-1)
	if err == nil {
		clause = s.enforceParity(clause)
	}
	return level, clause, err
}

func (s *nlqSearch) projectConflict(core []sat.Literal) error {
	if !s.state.validModel {
		return s.state.pop(1)
	}
	if s.state.level() < 2 {
		return arena.ErrConfig
	}
	level, clause, err := s.projectCore(core)
	if err != nil {
		return err
	}
	if len(clause) == 0 {
		return arena.ErrConfig
	}
	if err = s.solver.AddClause(clause, nlsat.Assumption{}); err != nil {
		return err
	}
	var scopes uint32
	maximum := level.Max()
	if maximum == NoQuantifierLevel {
		scopes = 2 * (s.state.level() / 2)
	} else {
		if uint64(maximum)+2 > uint64(s.state.level()) {
			return arena.ErrConfig
		}
		scopes = s.state.level() - maximum
		if scopes&1 != 0 {
			scopes--
		}
		if scopes < 2 {
			return arena.ErrConfig
		}
	}
	return s.state.pop(scopes)
}

func (s *nlqSearch) clauseFormula(clause []sat.Literal) (result ast.ASTRef, err error) {
	refs := make([]ast.ASTRef, 0, len(clause))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, s.manager.DelRef(&refs[i]))
		}
	}()
	handles := make([]ast.ASTHandle, 0, len(clause))
	for _, literal := range clause {
		var ref ast.ASTRef
		if mapped, found := s.assumptionForms[literal.Variable()]; found {
			h, cause := s.manager.RefGet(mapped)
			if cause != nil {
				return result, cause
			}
			if literal.Sign() {
				h, cause = s.manager.MkNot(h)
				if cause != nil {
					return result, cause
				}
			}
			ref, cause = s.manager.NewRef(h)
			if cause != nil {
				return result, cause
			}
		} else {
			var cause error
			ref, cause = s.converter.ConvertLiteral(literal)
			if cause != nil {
				return result, cause
			}
		}
		refs = append(refs, ref)
		h, cause := s.manager.RefGet(ref)
		if cause != nil {
			return result, cause
		}
		handles = append(handles, h)
	}
	formula, err := s.manager.MkOrArray(handles)
	if err != nil {
		return result, err
	}
	return s.manager.NewRef(formula)
}

func (s *nlqSearch) projectQE(core []sat.Literal) (err error) {
	level := s.state.level()
	if !s.state.validModel {
		return s.state.pop(1)
	}
	projectionLevel := level - 1
	if projectionLevel < 1 {
		projectionLevel = 1
	}
	dependency, clause, err := s.projectCube(core, projectionLevel)
	if err != nil {
		return fmt.Errorf("project cube: %w", err)
	}
	formula, err := s.clauseFormula(clause)
	if err != nil {
		return fmt.Errorf("convert projected clause: %w", err)
	}
	defer func() {
		if formula != (ast.ASTRef{}) {
			err = errors.Join(err, s.manager.DelRef(&formula))
		}
	}()
	if level == 1 || dependency.Max() == 0 {
		variable, cause := s.solver.MkBoolVar()
		if cause != nil {
			return fmt.Errorf("create projection assumption: %w", cause)
		}
		clause = append(clause, sat.MakeLiteral(variable, true))
		s.extra = append(s.extra, sat.MakeLiteral(variable, false))
		if cause = s.state.setLevel(variable, EmptyMaxLevel()); cause != nil {
			return fmt.Errorf("set projection assumption level: %w", cause)
		}
		h, cause := s.manager.RefGet(formula)
		if cause != nil {
			return fmt.Errorf("read projected formula: %w", cause)
		}
		stored, cause := s.manager.NewRef(h)
		if cause != nil {
			return fmt.Errorf("retain projected formula: %w", cause)
		}
		s.assumptionForms[variable] = stored
	} else {
		clause = s.enforceParity(clause)
	}
	if err = s.solver.AddClause(clause, nlsat.Assumption{}); err != nil {
		return fmt.Errorf("add projected clause: %w", err)
	}
	if level == 1 {
		if err = s.addAnswer(&formula); err != nil {
			return fmt.Errorf("retain simplified projection answer: %w", err)
		}
		if err = s.state.pop(1); err != nil {
			return fmt.Errorf("pop final projection: %w", err)
		}
		return nil
	}
	if err = s.state.pop(2); err != nil {
		return fmt.Errorf("pop projection pair: %w", err)
	}
	return nil
}

func (s *nlqSearch) answer() (ast.ASTRef, error) {
	if err := s.open(); err != nil {
		return ast.ASTRef{}, err
	}
	handles := make([]ast.ASTHandle, len(s.answers))
	for i, ref := range s.answers {
		h, err := s.manager.RefGet(ref)
		if err != nil {
			return ast.ASTRef{}, err
		}
		handles[i] = h
	}
	formula, err := s.manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return s.manager.NewRef(formula)
}

func (s *nlqSearch) addAnswer(formula *ast.ASTRef) error {
	if formula == nil || *formula == (ast.ASTRef{}) || s.answerSimplify == nil {
		return arena.ErrConfig
	}
	if err := s.answerSimplify.ApplyRef(*formula); err != nil {
		return err
	}
	h, err := s.manager.RefGet(*formula)
	if err != nil {
		return err
	}
	negative, inner, err := s.manager.IsNot(h)
	if err != nil {
		return err
	}
	if negative {
		falsity, cause := s.manager.MkFalse()
		if cause != nil {
			return cause
		}
		if err = s.answerSimplify.Insert(inner, falsity); err != nil {
			return err
		}
	} else {
		truth, cause := s.manager.MkTrue()
		if cause != nil {
			return cause
		}
		if err = s.answerSimplify.Insert(h, truth); err != nil {
			return err
		}
	}
	s.answers = append(s.answers, *formula)
	*formula = ast.ASTRef{}
	return nil
}

func (s *nlqSearch) check() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	for {
		s.rounds++
		assumptions, err := s.state.initAssumptions(s.isTrue, s.extra)
		if err != nil {
			return lbool.Undef, fmt.Errorf("nlqsat initialize assumptions at level %d: %w", s.state.level(), err)
		}
		input := append([]sat.Literal(nil), assumptions...)
		status, err := s.solver.CheckAssumptions(&input)
		if err != nil {
			return lbool.Undef, fmt.Errorf("nlqsat check assumptions at level %d: %w", s.state.level(), err)
		}
		switch status {
		case lbool.True:
			if err = s.state.saveAssumptionModel(assumptions); err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat save model at level %d: %w", s.state.level(), err)
			}
			if err = s.state.push(); err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat push level %d: %w", s.state.level(), err)
			}
		case lbool.False:
			if s.state.level() == 0 {
				return lbool.False, nil
			}
			if s.state.level() == 1 && s.mode == qsatSAT {
				return lbool.True, nil
			}
			if s.mode == qsatQE {
				err = s.projectQE(input)
			} else {
				err = s.projectConflict(input)
			}
			if err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat project at level %d: %w", s.state.level(), err)
			}
		case lbool.Undef:
			return lbool.Undef, nil
		}
	}
}

func (s *nlqSearch) close() (err error) {
	if s == nil || s.self != s {
		return arena.ErrConfig
	}
	if s.closed {
		return nil
	}
	if s.second != nil {
		if err = s.second.Close(); err != nil {
			return err
		}
		s.second = nil
	}
	if s.first != nil {
		if err = s.first.Close(); err != nil {
			return err
		}
		s.first = nil
	}
	for len(s.answers) != 0 {
		i := len(s.answers) - 1
		if err = s.manager.DelRef(&s.answers[i]); err != nil {
			return err
		}
		s.answers = s.answers[:i]
	}
	for variable, ref := range s.assumptionForms {
		if err = s.manager.DelRef(&ref); err != nil {
			return err
		}
		delete(s.assumptionForms, variable)
	}
	if s.answerSimplify != nil {
		if err = s.answerSimplify.Close(); err != nil {
			return err
		}
		s.answerSimplify = nil
	}
	if s.state != nil {
		if err = s.state.close(); err != nil {
			return err
		}
		s.state = nil
	}
	s.solver = nil
	s.closed = true
	return nil
}
