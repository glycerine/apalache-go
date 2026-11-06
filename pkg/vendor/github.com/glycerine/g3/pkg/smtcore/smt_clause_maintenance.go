package smtcore

// Source: src/smt/smt_context.cpp learned-clause activity and inactive-lemma
// deletion. The retained search has no restart loop, so fixed/geometric
// conflict-triggered collection is the live path.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type SMTClauseMaintenanceConfig struct {
	Strategy                   uint32
	Half                       bool
	Recent, Initial            uint32
	Factor, InverseDecay       float64
	NewOldRatio                uint32
	NewActivity, OldActivity   uint32
	NewRelevancy, OldRelevancy uint32
}

func defaultSMTClauseMaintenanceConfig() SMTClauseMaintenanceConfig {
	return SMTClauseMaintenanceConfig{
		Strategy: 0, Recent: 100, Initial: 5000, Factor: 1.1, InverseDecay: 1,
		NewOldRatio: 16, NewActivity: 10, OldActivity: 500, NewRelevancy: 45, OldRelevancy: 6,
	}
}

func (s *SMTInternalizerState) ConfigureClauseMaintenance(config SMTClauseMaintenanceConfig) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.clauses) != 0 || len(s.scopes) != 0 || config.Strategy > 3 || config.NewOldRatio == 0 || config.Factor < 1 || config.InverseDecay <= 0 {
		return ast.ErrASTDomain
	}
	s.clauseMaintenance = config
	s.lemmaGCThreshold = config.Initial
	s.conflictsSinceLemmaGC = 0
	return nil
}

func (s *SMTInternalizerState) clauseIsReason(clause *SMTClause) bool {
	for variable := int32(SMTFirstBoolVar); int(variable) < len(s.booleanData); variable++ {
		value, ok := s.booleanData[variable].Justification().Clause()
		if ok && value == clause {
			return true
		}
	}
	return false
}

func (s *SMTInternalizerState) clauseHasUnknowns(clause *SMTClause, threshold uint32) (bool, error) {
	if threshold == 0 {
		return true, nil
	}
	for _, literal := range clause.literals {
		value, err := s.Assignment(literal)
		if err != nil {
			return false, err
		}
		if value == lbool.Undef {
			threshold--
			if threshold == 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *SMTInternalizerState) deleteLearnedClauseIndexes(remove map[int]struct{}) error {
	if len(remove) == 0 {
		return nil
	}
	removedBefore := make([]int, len(s.clauses)+1)
	kept := 0
	for index := range s.clauses {
		removedBefore[index+1] = removedBefore[index]
		if _, found := remove[index]; found {
			if err := s.closeClause(&s.clauses[index]); err != nil {
				return err
			}
			removedBefore[index+1]++
			continue
		}
		if kept != index {
			s.clauses[kept] = s.clauses[index]
		}
		kept++
	}
	for index := range s.scopes {
		limit := s.scopes[index].clauseLimit
		if limit < 0 || limit >= len(removedBefore) {
			return ast.ErrASTDomain
		}
		s.scopes[index].clauseLimit -= removedBefore[limit]
	}
	s.statistics.NumDelClause += uint32(len(remove))
	s.clauses = s.clauses[:kept]
	return nil
}

func (s *SMTInternalizerState) garbageCollectLearnedClauses() error {
	if err := s.open(); err != nil {
		return err
	}
	config := s.clauseMaintenance
	if config.Strategy == 3 {
		return nil
	}
	learned := make([]int, 0)
	for index := range s.clauses {
		clause := s.clauses[index].clause
		if clause != nil && clause.IsLemma() && !clause.InReinitStack() {
			learned = append(learned, index)
		}
	}
	if uint32(len(learned)) <= config.Recent {
		return nil
	}
	oldEnd := len(learned) - int(config.Recent)
	remove := make(map[int]struct{})
	if config.Half {
		candidates := append([]int(nil), learned[:oldEnd]...)
		sort.SliceStable(candidates, func(i, j int) bool {
			left, _ := s.clauses[candidates[i]].clause.Activity()
			right, _ := s.clauses[candidates[j]].clause.Activity()
			return left > right
		})
		for _, index := range candidates[len(candidates)/2:] {
			if !s.clauseIsReason(s.clauses[index].clause) {
				remove[index] = struct{}{}
			}
		}
	} else {
		newFirst := len(learned) / int(config.NewOldRatio) * int(config.NewOldRatio-1)
		for position, index := range learned {
			clause := s.clauses[index].clause
			if s.clauseIsReason(clause) {
				continue
			}
			activity, err := clause.Activity()
			if err != nil {
				return err
			}
			activityThreshold := config.OldActivity
			if len(learned) != 0 && config.OldActivity >= config.NewActivity {
				activityThreshold -= (config.OldActivity - config.NewActivity) * uint32(position) / uint32(len(learned))
			}
			relevancy := config.OldRelevancy
			if position >= newFirst {
				relevancy = config.NewRelevancy
			}
			unknowns, err := s.clauseHasUnknowns(clause, relevancy)
			if err != nil {
				return err
			}
			if activity < activityThreshold && unknowns {
				remove[index] = struct{}{}
			}
			if config.InverseDecay != 1 {
				if err = clause.SetActivity(uint32(float64(activity) / config.InverseDecay)); err != nil {
					return err
				}
			}
		}
	}
	return s.deleteLearnedClauseIndexes(remove)
}

func (s *SMTInternalizerState) recordConflictForClauseMaintenance() error {
	s.conflictsSinceLemmaGC++
	if s.clauseMaintenance.Strategy > 1 || s.conflictsSinceLemmaGC <= s.lemmaGCThreshold {
		return nil
	}
	if err := s.garbageCollectLearnedClauses(); err != nil {
		return err
	}
	s.conflictsSinceLemmaGC = 0
	if s.clauseMaintenance.Strategy == 1 {
		s.lemmaGCThreshold = uint32(float64(s.lemmaGCThreshold) * s.clauseMaintenance.Factor)
	}
	return nil
}
