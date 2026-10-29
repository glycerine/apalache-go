package protomodel

// Source: src/smt/theory_str.h str_value_factory.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// StringFactory belongs to the separate theory_str backend. In particular,
// register_value is intentionally ignored; SequenceFactory's collision and
// delimiter policy is not the policy of this native factory.
type StringFactory struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	strings map[string]struct{}
	next    uint32
}

func NewStringFactory(model *ProtoModel, family ast.FamilyID) (*StringFactory, error) {
	if model == nil || family == ast.NullFamilyID {
		return nil, ast.ErrASTDomain
	}
	if err := model.open(); err != nil {
		return nil, err
	}
	return &StringFactory{manager: model.manager, family: family, strings: make(map[string]struct{})}, nil
}

func (f *StringFactory) FamilyID() ast.FamilyID { return f.family }

func (f *StringFactory) stringValue(text string) (ast.ASTHandle, error) {
	if f == nil || f.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	value, err := f.manager.MkString(text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return value, nil
}

func (f *StringFactory) SomeValue(ast.ASTHandle) (ast.ASTHandle, error) {
	return f.stringValue("some value")
}

func (f *StringFactory) SomeValues(_ ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	value, err := f.stringValue("value 1")
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(first, value); err != nil {
		return false, err
	}
	value, err = f.stringValue("value 2")
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(second, value); err != nil {
		return false, err
	}
	return true, nil
}

func (f *StringFactory) FreshValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if f == nil || f.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	stringSort, err := f.manager.IsStringSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if stringSort {
		for {
			text := "!" + strconv.FormatUint(uint64(f.next), 16) + "!"
			f.next++
			if _, found := f.strings[text]; found {
				continue
			}
			f.strings[text] = struct{}{}
			return f.stringValue(text)
		}
	}
	sequenceSort, regex, err := f.manager.RegexSequenceSort(sort)
	if err != nil || !regex {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	value, err := f.FreshValue(sequenceSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	regexValue, err := f.manager.MkSequenceToRegex(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return regexValue, nil
}

func (*StringFactory) RegisterValue(ast.ASTHandle) error { return nil }

func (f *StringFactory) Close() error {
	if f != nil {
		f.strings = nil
		f.manager = nil
	}
	return nil
}

var _ ValueFactory = (*StringFactory)(nil)
