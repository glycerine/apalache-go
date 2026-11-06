package opt

// Source: src/opt/pb_sls.cpp:187-521.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *PBSLS) evalClause(clause *pbSLSClause) bool {
	clause.value.SetInt64(0)
	for i, literal := range clause.literals {
		if s.literalValue(literal) {
			clause.value.Add(&clause.value, &clause.weights[i])
		}
	}
	if clause.equality {
		return clause.value.Cmp(&clause.bound) == 0
	}
	return clause.value.Cmp(&clause.bound) >= 0
}

func (s *PBSLS) initializeOccurrences(clauses []pbSLSClause, occurrences *[][]uint32) {
	for i := range clauses {
		for _, literal := range clauses[i].literals {
			for len(*occurrences) <= int(literal.variable) {
				*occurrences = append(*occurrences, nil)
			}
			(*occurrences)[literal.variable] = append((*occurrences)[literal.variable], uint32(i))
		}
	}
}

func (s *PBSLS) initializeSearch() error {
	s.bestAssignment = nil
	s.best.SetInt64(0)
	s.hardFalse.reset()
	s.softFalse.reset()
	s.hardOccurrences = make([][]uint32, len(s.variableExprs)+1)
	s.softOccurrences = make([][]uint32, len(s.variableExprs)+1)
	s.penalty.SetInt64(0)
	s.initializeOccurrences(s.hard, &s.hardOccurrences)
	s.initializeOccurrences(s.soft, &s.softOccurrences)
	for i := range s.hard {
		if !s.evalClause(&s.hard[i]) {
			s.hardFalse.insert(uint32(i))
			if s.originalModel == nil {
				return ErrPBSLS
			}
		}
	}
	for i := range s.soft {
		if !s.evalClause(&s.soft[i]) {
			s.softFalse.insert(uint32(i))
			s.penalty.Add(&s.penalty, &s.softWeights[i])
		}
	}
	s.best.Set(&s.penalty)
	return nil
}

// flipVariable toggles a variable and returns the hard-clause break delta.
func (s *PBSLS) flipVariable(variable uint32) int {
	s.assignment[variable] = !s.assignment[variable]
	breakCount := 0
	for _, index := range s.hardOccurrences[variable] {
		clause := &s.hard[index]
		wasFalse := s.hardFalse.contains(index)
		if s.evalClause(clause) {
			if wasFalse {
				breakCount--
				s.hardFalse.remove(index)
			}
		} else if !wasFalse {
			breakCount++
			s.hardFalse.insert(index)
		}
	}
	for _, index := range s.softOccurrences[variable] {
		clause := &s.soft[index]
		wasFalse := s.softFalse.contains(index)
		if s.evalClause(clause) {
			if wasFalse {
				s.penalty.Sub(&s.penalty, &s.softWeights[index])
				s.softFalse.remove(index)
			}
		} else if !wasFalse {
			s.penalty.Add(&s.penalty, &s.softWeights[index])
			s.softFalse.insert(index)
		}
	}
	return breakCount
}

func (s *PBSLS) pickHard() (*pbSLSClause, error) {
	index, err := s.hardFalse.choose(&s.random)
	if err != nil {
		return nil, err
	}
	return &s.hard[index], nil
}

func (s *PBSLS) pickSoft() (*pbSLSClause, error) {
	index, err := s.softFalse.choose(&s.random)
	if err != nil {
		return nil, err
	}
	return &s.soft[index], nil
}

func (s *PBSLS) flipHard() (pbSLSLiteral, error) {
	clause, err := s.pickHard()
	if err != nil || len(clause.literals) == 0 {
		return pbSLSNullLiteral, ErrPBSLS
	}
	minimum, selected := math.MaxInt, 0
	for i, literal := range clause.literals {
		count := s.flipVariable(literal.variable)
		if count < minimum || count == minimum && s.random.Bounded(5) == 1 {
			minimum, selected = count, i
		}
		if undo := s.flipVariable(literal.variable); undo != -count {
			return pbSLSNullLiteral, ErrPBSLS
		}
	}
	literal := clause.literals[selected]
	if s.random.Bounded(100) <= s.nonGreedyPercent {
		literal = clause.literals[s.random.Bounded(uint32(len(clause.literals)))]
	}
	s.flipVariable(literal.variable)
	return literal, nil
}

func (s *PBSLS) flipSoft() (pbSLSLiteral, error) {
	clause, err := s.pickSoft()
	if err != nil || len(clause.literals) == 0 {
		return pbSLSNullLiteral, ErrPBSLS
	}
	minimum, selected := math.MaxInt, 0
	minimumPenalty := new(big.Rat).Set(&s.penalty)
	for i, literal := range clause.literals {
		before := new(big.Rat).Set(&s.penalty)
		count := s.flipVariable(literal.variable)
		if count < 0 {
			return pbSLSNullLiteral, ErrPBSLS
		}
		if count == 0 && s.penalty.Cmp(before) < 0 {
			return literal, nil
		}
		if count < minimum || count == minimum && s.penalty.Cmp(minimumPenalty) < 0 {
			minimum, selected = count, i
			minimumPenalty.Set(&s.penalty)
		}
		if undo := s.flipVariable(literal.variable); undo != -count {
			return pbSLSNullLiteral, ErrPBSLS
		}
	}
	literal := clause.literals[selected]
	if s.random.Bounded(100) <= s.nonGreedyPercent {
		literal = clause.literals[s.random.Bounded(uint32(len(clause.literals)))]
	}
	s.flipVariable(literal.variable)
	return literal, nil
}

func (s *PBSLS) flipStep() (pbSLSLiteral, error) {
	s.stats.Flips++
	var literal pbSLSLiteral
	var err error
	if s.hardFalse.empty() {
		literal, err = s.flipSoft()
	} else {
		literal, err = s.flipHard()
	}
	if err != nil {
		return literal, err
	}
	if s.hardFalse.empty() && s.best.Cmp(&s.penalty) > 0 {
		s.bestAssignment = append(s.bestAssignment[:0], s.assignment...)
		s.best.Set(&s.penalty)
		s.stats.Improvements++
		s.maxFlips = 200
	}
	if !s.assignment[literal.variable] {
		literal = literal.negate()
	}
	return literal, nil
}

func (s *PBSLS) restoreRound() {
	s.softFalse.reset()
	s.hardFalse.reset()
	s.penalty.SetInt64(0)
	for i := range s.soft {
		if !s.evalClause(&s.soft[i]) {
			s.softFalse.insert(uint32(i))
			s.penalty.Add(&s.penalty, &s.softWeights[i])
		}
	}
	for i := range s.hard {
		if !s.evalClause(&s.hard[i]) {
			s.hardFalse.insert(uint32(i))
		}
	}
}

func (s *PBSLS) Check() (lbool.LBool, error) {
	if err := s.open(); err != nil || s.originalModel == nil {
		if err != nil {
			return lbool.Undef, err
		}
		return lbool.Undef, ErrPBSLS
	}
	if err := s.initializeSearch(); err != nil {
		return lbool.Undef, err
	}
	// There is no move to select when the starting assignment already satisfies
	// every hard clause and has zero soft penalty.
	if s.hardFalse.empty() && s.best.Sign() == 0 {
		return lbool.True, nil
	}
	baseline := append([]bool(nil), s.assignment...)
	for round := uint32(0); round < 40; round++ {
		s.maxFlips = 200
		for s.maxFlips > 0 {
			s.maxFlips--
			if _, err := s.flipStep(); err != nil {
				return lbool.Undef, err
			}
			canceled, err := s.manager.Canceled()
			if err != nil || canceled {
				if err != nil {
					return lbool.Undef, err
				}
				return lbool.Undef, nil
			}
			if s.hardFalse.empty() && s.best.Sign() == 0 {
				break
			}
		}
		if s.hardFalse.empty() && s.best.Sign() == 0 {
			break
		}
		if len(s.bestAssignment) != 0 {
			baseline = append(baseline[:0], s.bestAssignment...)
			round = 0
		}
		s.assignment = append(s.assignment[:0], baseline...)
		s.bestAssignment = nil
		s.restoreRound()
	}
	s.assignment = append(s.assignment[:0], baseline...)
	s.penalty.Set(&s.best)
	return lbool.True, nil
}

func (s *PBSLS) SoftHolds(index uint32) (bool, error) {
	if err := s.open(); err != nil || int(index) >= len(s.soft) {
		if err != nil {
			return false, err
		}
		return false, ErrPBSLS
	}
	return s.evalClause(&s.soft[index]), nil
}
