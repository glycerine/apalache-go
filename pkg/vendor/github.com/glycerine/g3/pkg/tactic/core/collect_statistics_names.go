package core

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *collectStatisticsTraversal) sort(s ast.ASTHandle) error {
	m := c.owner.manager
	uninterpreted, err := m.IsUninterpretedSort(s)
	if err != nil {
		return err
	}
	if uninterpreted {
		if !c.seenSort[s] {
			c.seenSort[s] = true
			c.add("uninterpreted-sorts", 1)
		}
		c.add("uninterpreted-sort-occurrences", 1)
		return nil
	}
	text, err := collectStatisticsFormat(m, s, true)
	if err != nil {
		return err
	}
	c.add("(declare-sort "+text+")", 1)
	n, err := m.GetSortNumParameters(s)
	if err != nil {
		return err
	}
	if n > 0 {
		name, e := m.GetASTName(s)
		if e != nil {
			return e
		}
		value, e := m.Symbols.String(name)
		if e != nil {
			return e
		}
		c.add("(declare-sort (_ "+value+" *))", 1)
	}
	return nil
}

func (c *collectStatisticsTraversal) declaration(d ast.ASTHandle) error {
	m := c.owner.manager
	arity, err := m.GetArity(d)
	if err != nil {
		return err
	}
	for i := uint32(0); i < arity; i++ {
		s, e := m.GetDomain(d, i)
		if e != nil {
			return e
		}
		if e = c.sort(s); e != nil {
			return e
		}
	}
	rangeSort, err := m.GetRange(d)
	if err != nil {
		return err
	}
	if err = c.sort(rangeSort); err != nil {
		return err
	}
	family, err := m.GetFamilyID(d)
	if err != nil {
		return err
	}
	if family == ast.NullFamilyID {
		if !c.seenDecl[d] {
			c.seenDecl[d] = true
			if arity == 0 {
				c.add("uninterpreted-constants", 1)
			} else {
				c.add("uninterpreted-functions", 1)
			}
		}
		if arity > 0 {
			c.add("uninterpreted-function-occurrences", 1)
		}
	} else {
		text, e := collectStatisticsFormat(m, d, false)
		if e != nil {
			return e
		}
		c.add(text, 1)
		name, e := m.GetASTName(d)
		if e != nil {
			return e
		}
		value, e := m.Symbols.String(name)
		if e != nil {
			return e
		}
		parameters, e := m.GetDeclParametersCopy(d)
		if e != nil {
			return e
		}
		for i := range parameters {
			e = errors.Join(e, m.DelParameter(&parameters[i]))
		}
		if e != nil {
			return e
		}
		if len(parameters) > 0 {
			c.add("(declare-fun (_ "+value+" *) *)", 1)
		} else {
			c.add("(declare-fun "+value+" *)", 1)
		}
	}
	c.add("function-applications", 1)
	return nil
}

func collectStatisticsFormat(m *ast.ASTManager, h ast.ASTHandle, sort bool) (text string, result error) {
	var format ast.ASTHandle
	var err error
	if sort {
		format, err = m.SMT2PPSort(h)
	} else {
		format, err = m.SMT2FdeclRef(h)
	}
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = m.PrintFormat(out, format, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}
