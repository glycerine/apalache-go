package api

// Source: src/api/api_seq.cpp:24-171.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) sequenceManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Manager()
}

// SequenceSort constructs Seq elementSort.
func (c *SMTContext) SequenceSort(elementSort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.sequenceManager()
	if err != nil {
		return result, err
	}
	family, err := m.SequenceFamilyID()
	if err != nil {
		return result, err
	}
	p, err := m.NewParameterAST(elementSort)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p)) }()
	return m.MkFamilySort(family, 0, []ast.ASTParameter{p})
}

// RegexSort constructs Re sequenceSort.
func (c *SMTContext) RegexSort(sequenceSort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.sequenceManager()
	if err != nil {
		return result, err
	}
	family, err := m.SequenceFamilyID()
	if err != nil {
		return result, err
	}
	p, err := m.NewParameterAST(sequenceSort)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p)) }()
	return m.MkFamilySort(family, 1, []ast.ASTParameter{p})
}

func (c *SMTContext) StringSort() (ast.ASTHandle, error) {
	m, err := c.sequenceManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	v, err := m.MkString("")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetSort(v)
}
func (c *SMTContext) String(value string) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkString(value)
}
func (c *SMTContext) StringValue(value ast.ASTHandle) (string, bool, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return "", false, e
	}
	return m.SequenceStringValue(value)
}
func (c *SMTContext) IsSequenceSort(sort ast.ASTHandle) (bool, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return false, e
	}
	_, yes, e := m.SequenceSortElement(sort)
	return yes, e
}
func (c *SMTContext) IsRegexSort(sort ast.ASTHandle) (bool, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return false, e
	}
	_, yes, e := m.RegexSequenceSort(sort)
	return yes, e
}
func (c *SMTContext) IsStringSort(sort ast.ASTHandle) (bool, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return false, e
	}
	return m.IsStringSort(sort)
}

func (c *SMTContext) SequenceEmpty(sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceEmpty(sort)
}
func (c *SMTContext) SequenceUnit(value ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceUnit(value)
}

func foldAST(values []ast.ASTHandle, binary func(ast.ASTHandle, ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	if len(values) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	r := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		var e error
		r, e = binary(values[i], r)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return r, nil
}
func (c *SMTContext) SequenceConcat(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return foldAST(values, m.MkSequenceConcat)
}
func (c *SMTContext) SequencePrefix(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequencePrefix(a, b)
}
func (c *SMTContext) SequenceSuffix(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceSuffix(a, b)
}
func (c *SMTContext) SequenceContains(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceContains(a, b)
}
func (c *SMTContext) SequenceExtract(s, o, n ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceExtract(s, o, n)
}
func (c *SMTContext) SequenceReplace(s, a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceReplace(s, a, b)
}
func (c *SMTContext) SequenceAt(s, i ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceAt(s, i)
}
func (c *SMTContext) SequenceLength(s ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceLength(s)
}
func (c *SMTContext) SequenceIndex(s, sub, off ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceIndex(s, sub, off)
}
func (c *SMTContext) SequenceToRegex(s ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceToRegex(s)
}
func (c *SMTContext) SequenceInRegex(s, r ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkSequenceInRegex(s, r)
}
func (c *SMTContext) IntToString(v ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIntToString(v)
}
func (c *SMTContext) StringToInt(v ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkStringToInt(v)
}

func (c *SMTContext) RegexLoop(r ast.ASTHandle, lower int32, upper ...int32) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexLoop(r, lower, upper...)
}
func (c *SMTContext) RegexPlus(r ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexPlus(r)
}
func (c *SMTContext) RegexStar(r ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexStar(r)
}
func (c *SMTContext) RegexOption(r ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexOption(r)
}
func (c *SMTContext) RegexComplement(r ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexComplement(r)
}
func (c *SMTContext) RegexRange(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexRange(a, b)
}
func (c *SMTContext) RegexConcat(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return foldAST(values, m.MkRegexConcat)
}
func (c *SMTContext) RegexUnion(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return foldAST(values, m.MkRegexUnion)
}
func (c *SMTContext) RegexIntersect(values ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return foldAST(values, m.MkRegexIntersect)
}
func (c *SMTContext) RegexEmpty(sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexEmpty(sort)
}
func (c *SMTContext) RegexFull(sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.sequenceManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkRegexFull(sort)
}
