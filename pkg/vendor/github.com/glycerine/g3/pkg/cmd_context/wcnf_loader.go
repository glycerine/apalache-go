package cmd_context

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

type WCNFLoadSummary struct{ Variables, HardClauses, SoftClauses int }

func (c *Context) LoadWCNF(document opt.WCNFDocument) (WCNFLoadSummary, error) {
	if c == nil {
		return WCNFLoadSummary{}, fmt.Errorf("wcnf: nil command context")
	}
	_, err := c.Manager()
	if err != nil {
		return WCNFLoadSummary{}, err
	}
	decls := make(map[uint32]ast.ASTHandle)
	summary := WCNFLoadSummary{}
	for i := range document.Clauses {
		clause, e := c.makeWCNFClause(document.Clauses[i].Literals, decls)
		if e != nil {
			return WCNFLoadSummary{}, fmt.Errorf("wcnf: clause %d: %w", i+1, e)
		}
		if document.Clauses[i].Weight >= document.TopWeight {
			e = c.Assert(clause, "")
			summary.HardClauses++
		} else {
			weight := new(big.Rat).SetInt(new(big.Int).SetUint64(document.Clauses[i].Weight))
			e = c.AddSoftConstraint(clause, weight, "")
			summary.SoftClauses++
		}
		if e != nil {
			return WCNFLoadSummary{}, fmt.Errorf("wcnf: clause %d: %w", i+1, e)
		}
	}
	// Native parsing creates only variables reached by clauses.
	summary.Variables = len(decls)
	return summary, nil
}

func (c *Context) makeWCNFClause(literals []opt.WCNFLiteral, decls map[uint32]ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	terms := make([]ast.ASTHandle, len(literals))
	for i, literal := range literals {
		decl, found := decls[literal.Variable]
		if !found {
			sort, e := m.MkBoolSort()
			if e != nil {
				return ast.ASTHandle{}, e
			}
			decl, e = c.DeclareFunction(strconv.FormatUint(uint64(literal.Variable), 10), nil, sort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			decls[literal.Variable] = decl
		}
		terms[i], err = m.MkApp(decl, nil)
		if err == nil && literal.Negative {
			terms[i], err = m.MkNot(terms[i])
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.MkOrArray(terms)
}
