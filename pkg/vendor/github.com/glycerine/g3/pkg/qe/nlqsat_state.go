package qe

// Source: src/qe/nlqsat.cpp:103-176,224-257,278-310,566-587.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type nlqState struct {
	self                *nlqState
	solver              *nlsat.Solver
	realModel           *nlsat.Assignment
	booleanModel        []lbool.LBool
	answerReal          *nlsat.Assignment
	answerBoolean       []lbool.LBool
	answerValid         bool
	validModel          bool
	cached, assumptions []sat.Literal
	limits              []int
	predicates          []*nlsat.LiteralVector
	realLevels          []MaxLevel
	booleanLevels       map[sat.BoolVar]MaxLevel
	closed              bool
}

func newNLQState(solver *nlsat.Solver, realLevels []MaxLevel) (state *nlqState, err error) {
	if solver == nil {
		return nil, arena.ErrConfig
	}
	am, err := solver.AlgebraicManager()
	if err != nil {
		return nil, err
	}
	n := &nlqState{solver: solver, realLevels: append([]MaxLevel(nil), realLevels...), booleanLevels: make(map[sat.BoolVar]MaxLevel)}
	n.self = n
	defer func() {
		if err != nil {
			err = errors.Join(err, n.close())
		}
	}()
	n.realModel, err = nlsat.NewAssignment(am)
	if err != nil {
		return n, err
	}
	n.answerReal, err = nlsat.NewAssignment(am)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (n *nlqState) open() error {
	if n == nil || n.self != n || n.solver == nil {
		return arena.ErrConfig
	}
	if n.closed {
		return arena.ErrClosed
	}
	_, err := n.solver.PolynomialManager()
	return err
}

func (n *nlqState) level() uint32 { return uint32(len(n.limits)) }
func nlqExists(level uint32) bool { return level&1 == 0 }

func (n *nlqState) push() error {
	if err := n.open(); err != nil {
		return err
	}
	n.limits = append(n.limits, len(n.cached))
	return nil
}

func (n *nlqState) clearModel() error {
	n.validModel = false
	if err := n.realModel.Reset(); err != nil {
		return err
	}
	n.booleanModel = n.booleanModel[:0]
	return n.solver.SetRealValues(n.realModel)
}

func (n *nlqState) pop(scopes uint32) error {
	if err := n.open(); err != nil {
		return err
	}
	if scopes == 0 || scopes > n.level() {
		return arena.ErrBounds
	}
	if err := n.clearModel(); err != nil {
		return err
	}
	level := n.level() - scopes
	n.cached = n.cached[:n.limits[level]]
	n.limits = n.limits[:level]
	return nil
}

func (n *nlqState) saveModel() error {
	if err := n.solver.GetRealValues(n.realModel); err != nil {
		return err
	}
	if err := n.solver.GetBooleanValues(&n.booleanModel); err != nil {
		return err
	}
	n.validModel = true
	if nlqExists(n.level()) {
		if err := n.answerReal.Copy(n.realModel); err != nil {
			return err
		}
		n.answerBoolean = append(n.answerBoolean[:0], n.booleanModel...)
		n.answerValid = true
	}
	return nil
}

func (n *nlqState) saveAssumptionModel(assumptions []sat.Literal) error {
	if err := n.saveModel(); err != nil {
		return err
	}
	// CheckAssumptions removes its temporary unit clauses before returning.
	// Variables used only to guard a projected QE clause can consequently be
	// absent from the solver's saved Boolean assignment even though their
	// assumption fixes the value needed by the model-based projection.
	for _, literal := range assumptions {
		variable := int(literal.Variable())
		for len(n.booleanModel) <= variable {
			n.booleanModel = append(n.booleanModel, lbool.Undef)
		}
		value := lbool.True
		if literal.Sign() {
			value = lbool.False
		}
		if current := n.booleanModel[variable]; current == lbool.Undef {
			n.booleanModel[variable] = value
		} else if current != value {
			return arena.ErrConfig
		}
	}
	if nlqExists(n.level()) {
		n.answerBoolean = append(n.answerBoolean[:0], n.booleanModel...)
	}
	return nil
}

func (n *nlqState) restoreModel() error {
	if !n.validModel {
		return arena.ErrConfig
	}
	if err := n.solver.SetRealValues(n.realModel); err != nil {
		return err
	}
	return n.solver.SetBooleanValues(n.booleanModel)
}

func (n *nlqState) restoreAnswerModel() error {
	if !n.answerValid {
		return arena.ErrConfig
	}
	if err := n.solver.SetRealValues(n.answerReal); err != nil {
		return err
	}
	return n.solver.SetBooleanValues(n.answerBoolean)
}

func (n *nlqState) addCurrentLiteral(out *[]sat.Literal, literal sat.Literal) error {
	value, err := n.solver.LiteralValue(literal)
	if err != nil {
		return err
	}
	switch value {
	case lbool.True:
		*out = append(*out, literal)
	case lbool.False:
		*out = append(*out, literal.Negated())
	default:
		return arena.ErrConfig
	}
	return nil
}

func (n *nlqState) setLevel(variable sat.BoolVar, level MaxLevel) error {
	if err := n.open(); err != nil {
		return err
	}
	n.booleanLevels[variable] = level
	maximum := level.Max()
	if maximum == NoQuantifierLevel {
		return nil
	}
	for uint64(len(n.predicates)) <= uint64(maximum) {
		vector, err := nlsat.NewLiteralVector(n.solver)
		if err != nil {
			return err
		}
		n.predicates = append(n.predicates, vector)
	}
	return n.predicates[maximum].Push(sat.MakeLiteral(variable, false))
}

func (n *nlqState) getLevel(literal sat.Literal) (MaxLevel, error) {
	if level, found := n.booleanLevels[literal.Variable()]; found {
		return level, nil
	}
	level := EmptyMaxLevel()
	var variables []uint32
	if err := n.solver.LiteralVariables(literal, &variables); err != nil {
		return level, err
	}
	for _, variable := range variables {
		if uint64(variable) >= uint64(len(n.realLevels)) {
			return level, arena.ErrBounds
		}
		level.Merge(n.realLevels[variable])
	}
	if err := n.setLevel(literal.Variable(), level); err != nil {
		return level, err
	}
	return level, nil
}

func (n *nlqState) initAssumptions(isTrue sat.Literal, extra []sat.Literal) ([]sat.Literal, error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	level := n.level()
	n.assumptions = n.assumptions[:0]
	if nlqExists(level) {
		n.assumptions = append(n.assumptions, isTrue)
	} else {
		n.assumptions = append(n.assumptions, isTrue.Negated())
	}
	n.assumptions = append(n.assumptions, extra...)
	if !n.validModel {
		n.assumptions = append(n.assumptions, n.cached...)
		return n.assumptions, nil
	}
	if err := n.restoreModel(); err != nil {
		return nil, err
	}
	if level == 0 {
		if len(n.cached) != 0 {
			return nil, arena.ErrConfig
		}
		return n.assumptions, n.saveModel()
	}
	if uint64(level) <= uint64(len(n.predicates)) {
		var literals []sat.Literal
		if err := n.predicates[level-1].Literals(&literals); err != nil {
			return nil, err
		}
		for _, literal := range literals {
			if err := n.addCurrentLiteral(&n.cached, literal); err != nil {
				return nil, err
			}
		}
	}
	n.assumptions = append(n.assumptions, n.cached...)
	for i := level + 1; uint64(i) < uint64(len(n.predicates)); i += 2 {
		var literals []sat.Literal
		if err := n.predicates[i].Literals(&literals); err != nil {
			return nil, err
		}
		for _, literal := range literals {
			predicateLevel := n.booleanLevels[literal.Variable()]
			use := predicateLevel.Universal == i && (predicateLevel.Existential == NoQuantifierLevel || predicateLevel.Existential < level) ||
				predicateLevel.Existential == i && (predicateLevel.Universal == NoQuantifierLevel || predicateLevel.Universal < level)
			if use {
				if err := n.addCurrentLiteral(&n.assumptions, literal); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := n.saveModel(); err != nil {
		return nil, err
	}
	return n.assumptions, nil
}

func (n *nlqState) close() (err error) {
	if n == nil || n.self != n {
		return arena.ErrConfig
	}
	if n.closed {
		return nil
	}
	for len(n.predicates) > 0 {
		i := len(n.predicates) - 1
		if err = n.predicates[i].Close(); err != nil {
			return err
		}
		n.predicates = n.predicates[:i]
	}
	if n.answerReal != nil {
		if err = n.answerReal.Close(); err != nil {
			return err
		}
		n.answerReal = nil
	}
	if n.realModel != nil {
		if err = n.realModel.Close(); err != nil {
			return err
		}
		n.realModel = nil
	}
	n.solver = nil
	n.closed = true
	return nil
}
