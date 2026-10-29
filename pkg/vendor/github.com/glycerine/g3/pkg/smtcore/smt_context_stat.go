package smtcore

// Source: src/smt/smt_context_stat.cpp.

import (
	"io"
	"os"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *SMTInternalizerState) LemmaAverageActivity() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	var sum uint64
	var count uint64
	for i := range s.clauses {
		clause := s.clauses[i].clause
		if clause == nil || clause.Kind() == SMTClauseAux {
			continue
		}
		activity, err := clause.Activity()
		if err != nil {
			return 0, err
		}
		sum += uint64(activity)
		count++
	}
	if count == 0 {
		return 0, nil
	}
	return uint32(sum / count), nil
}

func (s *SMTInternalizerState) clauseLiteralOccurrences() ([]uint32, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]uint32, len(s.assignment))
	for i := range s.clauses {
		clause := s.clauses[i].clause
		if clause == nil {
			continue
		}
		for _, literal := range clause.Literals() {
			if literal.Index() < 0 || int(literal.Index()) >= len(result) {
				return nil, ast.ErrASTDomain
			}
			result[literal.Index()]++
		}
	}
	return result, nil
}

func (s *SMTInternalizerState) DisplayLiteralOccurrences(out io.Writer) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	occurrences, err := s.clauseLiteralOccurrences()
	if err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(s.manager)
	if err != nil {
		return err
	}
	for index, count := range occurrences {
		if count == 0 {
			continue
		}
		literal := SMTLiteralFromIndex(int32(index))
		if _, err = io.WriteString(out, strconv.FormatUint(uint64(count), 10)+" "); err != nil {
			return err
		}
		if literal.Sign() {
			_, err = io.WriteString(out, "1 ")
		} else {
			_, err = io.WriteString(out, "0 ")
		}
		if err != nil {
			return err
		}
		expression, err := s.BoolExpression(literal.Var())
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, expression, nil); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMTInternalizerState) DisplayAssignedLiteralsPerLevel(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "["); err != nil {
		return err
	}
	previous := 0
	for _, scope := range s.scopes {
		if previous > scope.assignedLimit {
			return ast.ErrASTDomain
		}
		if _, err := io.WriteString(out, strconv.Itoa(scope.assignedLimit-previous)+" "); err != nil {
			return err
		}
		previous = scope.assignedLimit
	}
	if previous > len(s.assignedLiterals) {
		return ast.ErrASTDomain
	}
	_, err := io.WriteString(out, strconv.Itoa(len(s.assignedLiterals)-previous)+"]")
	return err
}

func (s *SMTInternalizerState) variableOccurrences() ([]uint32, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]uint32, len(s.booleanData))
	for i := range s.clauses {
		clause := s.clauses[i].clause
		if clause == nil {
			continue
		}
		for _, literal := range clause.Literals() {
			variable := literal.Var()
			if variable < 0 || int(variable) >= len(result) {
				return nil, ast.ErrASTDomain
			}
			result[variable]++
		}
	}
	return result, nil
}

func (s *SMTInternalizerState) DisplayVariableOccurrenceHistogram(out io.Writer) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	occurrences, err := s.variableOccurrences()
	if err != nil {
		return err
	}
	histogram := make([]uint32, 1)
	for _, count := range occurrences {
		if int(count) >= len(histogram) {
			histogram = append(histogram, make([]uint32, int(count)+1-len(histogram))...)
		}
		histogram[count]++
	}
	if _, err = io.WriteString(out, "number of atoms having k occs:\n"); err != nil {
		return err
	}
	for count := 1; count < len(histogram); count++ {
		if histogram[count] != 0 {
			if _, err = io.WriteString(out, strconv.Itoa(count)+":"+strconv.FormatUint(uint64(histogram[count]), 10)+" "); err != nil {
				return err
			}
		}
	}
	_, err = io.WriteString(out, "\n")
	return err
}

func (s *SMTInternalizerState) DisplayMinimumVariableOccurrences(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	occurrences := make([]uint32, len(s.booleanData))
	for i := range s.clauses {
		clause := s.clauses[i].clause
		if clause == nil || len(clause.Literals()) == 0 {
			continue
		}
		minimum := clause.Literals()[0].Var()
		for _, literal := range clause.Literals()[1:] {
			if literal.Var() < minimum {
				minimum = literal.Var()
			}
		}
		if minimum < 0 || int(minimum) >= len(occurrences) {
			return ast.ErrASTDomain
		}
		occurrences[minimum]++
	}
	if _, err := io.WriteString(out, "number of min occs:\n"); err != nil {
		return err
	}
	for variable, count := range occurrences {
		if count != 0 {
			if _, err := io.WriteString(out, strconv.Itoa(variable)+":"+strconv.FormatUint(uint64(count), 10)+" "); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}

// DisplayResolutionSubsumptionProfile preserves the source implementation's
// observable quirk: it ignores out and writes both reports to stderr.
func (s *SMTInternalizerState) DisplayResolutionSubsumptionProfile(_ io.Writer) error {
	if err := s.DisplayVariableOccurrenceHistogram(os.Stderr); err != nil {
		return err
	}
	if err := s.DisplayMinimumVariableOccurrences(os.Stderr); err != nil {
		return err
	}
	_, err := io.WriteString(os.Stderr, "\n")
	return err
}

func (s *SMTInternalizerState) DisplayProfile(out io.Writer, resolutionSubsumption bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if !resolutionSubsumption {
		return nil
	}
	return s.DisplayResolutionSubsumptionProfile(out)
}
