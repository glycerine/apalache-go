package smtcore

// Source: src/smt/smt_conflict_resolution.cpp:255-304.

import "github.com/glycerine/g3/pkg/ast"

func smtLiteralAssignmentLevel(literal SMTLiteral, levels []uint32) (uint32, error) {
	variable := literal.Var()
	if variable < 0 || int(variable) >= len(levels) {
		return 0, ast.ErrASTDomain
	}
	return levels[variable], nil
}

func SMTJustificationMaxLevel(expander *SMTAntecedentExpander, justification SMTJustification, levels []uint32) (uint32, error) {
	if expander == nil || justification == nil {
		return 0, ast.ErrASTHandle
	}
	antecedents, err := expander.ExpandJustification(justification)
	if err != nil {
		return 0, err
	}
	var maximum uint32
	for _, literal := range antecedents {
		level, err := smtLiteralAssignmentLevel(literal, levels)
		if err != nil {
			return 0, err
		}
		if level > maximum {
			maximum = level
		}
	}
	return maximum, nil
}

func SMTBooleanJustificationMaxLevel(expander *SMTAntecedentExpander, consequent SMTLiteral, justification SMTBooleanJustification, levels []uint32) (uint32, error) {
	var maximum uint32
	if consequent != SMTFalseLiteral {
		level, err := smtLiteralAssignmentLevel(consequent, levels)
		if err != nil {
			return 0, err
		}
		maximum = level
	}
	update := func(literal SMTLiteral) error {
		level, err := smtLiteralAssignmentLevel(literal, levels)
		if err != nil {
			return err
		}
		if level > maximum {
			maximum = level
		}
		return nil
	}
	switch justification.Kind() {
	case SMTBooleanJustificationClause:
		clause, ok := justification.Clause()
		if !ok || clause == nil || len(clause.literals) < 2 {
			return 0, ast.ErrASTDomain
		}
		i := 0
		if consequent != SMTFalseLiteral {
			if clause.literals[0] == consequent {
				i = 1
			} else if clause.literals[1] == consequent {
				if err := update(clause.literals[0]); err != nil {
					return 0, err
				}
				i = 2
			} else {
				return 0, ast.ErrASTDomain
			}
		}
		for ; i < len(clause.literals); i++ {
			if err := update(clause.literals[i]); err != nil {
				return 0, err
			}
		}
		if clause.justification != nil {
			level, err := SMTJustificationMaxLevel(expander, clause.justification, levels)
			if err != nil {
				return 0, err
			}
			if level > maximum {
				maximum = level
			}
		}
	case SMTBooleanJustificationBinaryClause:
		literal, ok := justification.Literal()
		if !ok {
			return 0, ast.ErrASTDomain
		}
		if err := update(literal); err != nil {
			return 0, err
		}
	case SMTBooleanJustificationAxiom:
	case SMTBooleanJustificationObject:
		object, ok := justification.Justification()
		if !ok || object == nil {
			return 0, ast.ErrASTDomain
		}
		level, err := SMTJustificationMaxLevel(expander, object, levels)
		if err != nil {
			return 0, err
		}
		if level > maximum {
			maximum = level
		}
	default:
		return 0, ast.ErrASTDomain
	}
	return maximum, nil
}
