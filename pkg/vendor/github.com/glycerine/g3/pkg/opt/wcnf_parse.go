package opt

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func ParseWCNF(input io.Reader) (WCNFDocument, error) {
	if input == nil {
		return WCNFDocument{}, fmt.Errorf("wcnf: nil input")
	}
	s := newWCNFScanner(input)
	var result WCNFDocument
	header := false
	for s.scan() {
		fields := strings.Fields(s.input.Text())
		if len(fields) == 0 || fields[0] == "c" {
			continue
		}
		if fields[0] == "p" {
			if header || len(fields) != 5 || fields[1] != "wcnf" {
				return WCNFDocument{}, s.errorf("expected p wcnf <variables> <clauses> <top>")
			}
			nv, err := s.uint(fields[2], "variable count")
			if err != nil || nv > math.MaxUint32 {
				if err != nil {
					return WCNFDocument{}, err
				}
				return WCNFDocument{}, s.errorf("variable count overflows uint32")
			}
			nc, err := s.uint(fields[3], "clause count")
			if err != nil || nc > math.MaxUint32 {
				if err != nil {
					return WCNFDocument{}, err
				}
				return WCNFDocument{}, s.errorf("clause count overflows uint32")
			}
			top, err := s.uint(fields[4], "top weight")
			if err != nil {
				return WCNFDocument{}, err
			}
			result.NumVariables, result.NumClauses, result.TopWeight = uint32(nv), uint32(nc), top
			header = true
			continue
		}
		if !header {
			return WCNFDocument{}, s.errorf("clause before header")
		}
		if len(fields) < 2 || fields[len(fields)-1] != "0" {
			return WCNFDocument{}, s.errorf("clause is not terminated by 0")
		}
		weight, err := s.uint(fields[0], "weight")
		if err != nil {
			return WCNFDocument{}, err
		}
		clause := WCNFClause{Weight: weight}
		for _, field := range fields[1 : len(fields)-1] {
			lit, e := strconv.ParseInt(field, 10, 64)
			if e != nil || lit == 0 || lit == math.MinInt64 {
				return WCNFDocument{}, s.errorf("invalid literal %q", field)
			}
			negative := lit < 0
			if negative {
				lit = -lit
			}
			if lit > math.MaxUint32 || uint64(lit) > uint64(result.NumVariables) {
				return WCNFDocument{}, s.errorf("literal %q exceeds declared variables", field)
			}
			clause.Literals = append(clause.Literals, WCNFLiteral{Variable: uint32(lit), Negative: negative})
		}
		result.Clauses = append(result.Clauses, clause)
	}
	if err := s.input.Err(); err != nil {
		return WCNFDocument{}, err
	}
	if !header {
		return WCNFDocument{}, fmt.Errorf("wcnf: missing header")
	}
	if uint64(len(result.Clauses)) != uint64(result.NumClauses) {
		return WCNFDocument{}, fmt.Errorf("wcnf: declared %d clauses but parsed %d", result.NumClauses, len(result.Clauses))
	}
	return result, nil
}
