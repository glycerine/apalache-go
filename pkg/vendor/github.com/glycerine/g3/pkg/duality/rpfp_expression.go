package duality

// Source: src/duality/duality_rpfp.cpp, Z3User::CountOperators and
// RPFP::SuffixVariable / HideVariable.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// CountOperators counts distinct conjunctions and disjunctions in the source
// Boolean skeleton and adds two per quantified variable. Other applications
// are leaves, including negations and uninterpreted predicates.
func CountOperators(term *Expr) (int, error) {
	root, err := term.Raw()
	if err != nil {
		return 0, err
	}
	m := term.value.object.context.manager
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{root}
	count := 0
	for len(todo) > 0 {
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, e := m.GetKind(h)
		if e != nil {
			return 0, e
		}
		switch kind {
		case ast.ASTQuantifier:
			n, e := m.GetNumDeclarations(h)
			if e != nil {
				return 0, e
			}
			count += 2 * int(n)
			body, e := m.GetQuantifierBody(h)
			if e != nil {
				return 0, e
			}
			todo = append(todo, body)
		case ast.ASTApp:
			family, e := m.GetFamilyID(h)
			if e != nil {
				return 0, e
			}
			if family != m.BasicFamilyID() {
				continue
			}
			op, e := m.GetDeclKind(h)
			if e != nil {
				return 0, e
			}
			if op != ast.ASTOpAnd && op != ast.ASTOpOr {
				continue
			}
			count++
			n, e := m.GetNumArgs(h)
			if e != nil {
				return 0, e
			}
			for j := uint32(0); j < n; j++ {
				child, e := m.GetArg(h, j)
				if e != nil {
					return 0, e
				}
				todo = append(todo, child)
			}
		}
	}
	return count, nil
}

func rpfpVariable(term *Expr, prefix string, number int) (_ *Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	decl, err := term.Decl()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, decl.Close()) }()
	symbol, err := decl.Name()
	if err != nil {
		return nil, err
	}
	name, err := symbol.String()
	if err != nil {
		return nil, err
	}
	sort, err := term.Sort()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, sort.Close()) }()
	context := term.value.object.context
	nameSymbol, err := context.StringSymbol(prefix + name + "_" + strconv.Itoa(number))
	if err != nil {
		return nil, err
	}
	return context.Constant(nameSymbol, sort)
}

// SuffixVariable creates the indexed copy used for a Duality node.
func SuffixVariable(term *Expr, number int) (*Expr, error) { return rpfpVariable(term, "", number) }

// HideVariable creates the private indexed copy used for a Duality edge.
func HideVariable(term *Expr, number int) (*Expr, error) { return rpfpVariable(term, "@p_", number) }
