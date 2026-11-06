package smtcore

// Source: src/smt/theory_str.cpp final_check_eh free-variable scan and the
// !needToAssignFreeVars conversion/path-completeness branch.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTStringTheory) unassignedStringVariables(analysis *smtStringDependencyAnalysis) ([]ast.ASTHandle, error) {
	var free []ast.ASTHandle
	for _, variable := range stringOrderedCounts(analysis.variables) {
		if _, internal := t.internalVariables[variable]; internal {
			continue
		}
		if _, regex := t.regexVariables[variable]; regex {
			continue
		}
		_, assigned, err := t.eqcStringValue(variable)
		if err != nil {
			return nil, err
		}
		if !assigned {
			free = append(free, variable)
		}
	}
	return free, nil
}

func (t *SMTStringTheory) finalCheckAssignedStrings() (SMTFinalCheckStatus, error) {
	added := false
	// Source observes vector growth during agreement callbacks; keep the
	// dynamic indexed bound instead of ranging over a fixed slice header.
	for index := 0; index < len(t.conversions); index++ {
		expression := t.conversions[index]
		op, own, err := t.operation(expression)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if !own {
			return SMTFinalCheckGiveUp, ast.ErrASTDomain
		}
		var changed bool
		switch op {
		case 28:
			changed, err = t.finalCheckStringToInt(expression)
		case 27:
			changed, err = t.finalCheckIntToString(expression)
		default:
			return SMTFinalCheckGiveUp, ast.ErrASTDomain
		}
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		added = added || changed
	}
	if added {
		return SMTFinalCheckContinue, nil
	}
	enabled, err := t.regexAutomataEnabled()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if enabled {
		var terms []ast.ASTHandle
		for member := range t.regexTerms {
			terms = append(terms, member)
		}
		stringSortHandles(terms)
		frame := smtStringRegexStage{theory: t}
		for _, member := range terms {
			// Native extracts these arguments before reading the assignment.
			if _, _, matched, err := t.state.manager.SequenceBinaryOperationArguments(member, 12); err != nil || !matched {
				if err == nil {
					err = ast.ErrASTDomain
				}
				return SMTFinalCheckGiveUp, err
			}
			assignment, err := frame.assignment(member)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if assignment == lbool.Undef {
				continue
			}
			if _, done := t.regexStage.pathDone[member]; !done {
				return SMTFinalCheckContinue, nil
			}
		}
	}
	// unused_internal_variables is declared empty and never populated in the
	// retained source. Its decoy-assignment arm is unreachable.
	return SMTFinalCheckDone, nil
}
