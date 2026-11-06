package opt

import (
	"fmt"
	"io"
	"math"
)

func ParseOPB(input io.Reader) (OPBDocument, error) {
	if input == nil {
		return OPBDocument{}, fmt.Errorf("opb: nil input")
	}
	s := newOPBScanner(input)
	var result OPBDocument
	for {
		err := s.skipWhitespace()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return OPBDocument{}, err
		}
		if b, _ := s.peek(); b == '*' {
			if err = s.skipComment(); err != nil && err != io.EOF {
				return OPBDocument{}, err
			}
			continue
		}
		if yes, _ := s.consume("min:"); yes {
			objective, e := parseOPBObjective(s, true)
			if e != nil {
				return OPBDocument{}, e
			}
			result.Objectives = append(result.Objectives, objective)
			continue
		}
		if yes, _ := s.consume("max:"); yes {
			objective, e := parseOPBObjective(s, false)
			if e != nil {
				return OPBDocument{}, e
			}
			result.Objectives = append(result.Objectives, objective)
			continue
		}
		constraint, e := parseOPBConstraint(s)
		if e != nil {
			return OPBDocument{}, e
		}
		result.Constraints = append(result.Constraints, constraint)
	}
}

func parseOPBLiteral(s *opbScanner) (OPBLiteral, error) {
	negative, _ := s.consume("~")
	x, err := s.consume("x")
	if err != nil || !x {
		return OPBLiteral{}, fmt.Errorf("opb:%d: expected x", s.line)
	}
	id, err := s.integer()
	if err != nil {
		return OPBLiteral{}, err
	}
	if !id.IsInt64() || id.Int64() < math.MinInt32 || id.Int64() > math.MaxInt32 {
		return OPBLiteral{}, fmt.Errorf("opb:%d: variable identifier overflows int32", s.line)
	}
	return OPBLiteral{Variable: int32(id.Int64()), Negative: negative}, nil
}

func parseOPBTerm(s *opbScanner) (OPBTerm, error) {
	coefficient, err := s.integer()
	if err != nil {
		return OPBTerm{}, err
	}
	literal, err := parseOPBLiteral(s)
	if err != nil {
		return OPBTerm{}, err
	}
	term := OPBTerm{Literals: []OPBLiteral{literal}}
	term.Coefficient.Set(&coefficient)
	for {
		if err = s.skipWhitespace(); err != nil {
			if err == io.EOF {
				return term, nil
			}
			return OPBTerm{}, fmt.Errorf("opb:%d: unterminated term: %w", s.line, err)
		}
		b, _ := s.peek()
		if b != '~' && b != 'x' {
			return term, nil
		}
		literal, err = parseOPBLiteral(s)
		if err != nil {
			return OPBTerm{}, err
		}
		term.Literals = append(term.Literals, literal)
	}
}

func parseOPBObjective(s *opbScanner, minimize bool) (OPBObjective, error) {
	result := OPBObjective{MinimizeSyntax: minimize}
	for {
		yes, err := s.consume(";")
		if yes || err == io.EOF {
			if len(result.Terms) == 0 {
				return OPBObjective{}, fmt.Errorf("opb:%d: empty objective", s.line)
			}
			return result, nil
		}
		if err != nil {
			return OPBObjective{}, err
		}
		term, err := parseOPBTerm(s)
		if err != nil {
			return OPBObjective{}, err
		}
		result.Terms = append(result.Terms, term)
	}
}

func parseOPBConstraint(s *opbScanner) (OPBConstraint, error) {
	var result OPBConstraint
	for {
		for _, relation := range []struct {
			text string
			kind OPBRelation
		}{{">=", OPBGreaterEqual}, {"=", OPBEqual}, {"<=", OPBLessEqual}} {
			if yes, _ := s.consume(relation.text); yes {
				if len(result.Terms) == 0 {
					return OPBConstraint{}, fmt.Errorf("opb:%d: empty constraint", s.line)
				}
				bound, err := s.integer()
				if err != nil {
					return OPBConstraint{}, err
				}
				if _, err = s.consume(";"); err != nil && err != io.EOF {
					return OPBConstraint{}, err
				}
				result.Relation = relation.kind
				result.Bound.Set(&bound)
				return result, nil
			}
		}
		term, err := parseOPBTerm(s)
		if err != nil {
			return OPBConstraint{}, err
		}
		result.Terms = append(result.Terms, term)
	}
}
