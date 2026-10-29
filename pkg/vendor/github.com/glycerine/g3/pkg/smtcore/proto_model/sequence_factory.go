package protomodel

// Source: src/smt/theory_seq_empty.h seq_factory.

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// SequenceFactory supplies canonical and fresh values for sequence and
// regular-expression sorts during SMT model construction.
type SequenceFactory struct {
	manager   *ast.ASTManager
	family    ast.FamilyID
	model     *ProtoModel
	strings   map[string]struct{}
	next      uint32
	delimiter string
}

func NewSequenceFactory(model *ProtoModel, family ast.FamilyID) (*SequenceFactory, error) {
	if model == nil || family == ast.NullFamilyID {
		return nil, ast.ErrASTDomain
	}
	if err := model.open(); err != nil {
		return nil, err
	}
	return &SequenceFactory{
		manager: model.manager, family: family, model: model,
		strings: map[string]struct{}{"": {}, "a": {}, "b": {}}, delimiter: "!",
	}, nil
}

func (f *SequenceFactory) FamilyID() ast.FamilyID { return f.family }

func (f *SequenceFactory) classify(sort ast.ASTHandle) (sequence, regex bool, element ast.ASTHandle, err error) {
	if f == nil || f.manager == nil || f.model == nil {
		return false, false, ast.ASTHandle{}, ast.ErrASTHandle
	}
	element, sequence, err = f.manager.SequenceSortElement(sort)
	if err != nil || sequence {
		return sequence, false, element, err
	}
	element, regex, err = f.manager.RegexSequenceSort(sort)
	return false, regex, element, err
}

func (f *SequenceFactory) SomeValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	sequence, regex, element, err := f.classify(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sequence {
		return f.manager.MkSequenceEmpty(sort)
	}
	if regex {
		empty, err := f.manager.MkSequenceEmpty(element)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.manager.MkSequenceToRegex(empty)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}

func (f *SequenceFactory) SomeValues(sort ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if f == nil || f.manager == nil {
		return false, ast.ErrASTHandle
	}
	stringSort, err := f.manager.IsStringSort(sort)
	if err != nil {
		return false, err
	}
	if stringSort {
		value, err := f.manager.MkString("a")
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(first, value); err != nil {
			return false, err
		}
		value, err = f.manager.MkString("b")
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, value); err != nil {
			return false, err
		}
		return true, nil
	}
	sequence, _, element, err := f.classify(sort)
	if err != nil {
		return false, err
	}
	if !sequence {
		return false, diagnostic.BorrowProcessDebugContext().NotImplementedYet("src/smt/theory_seq_empty.h", 94)
	}
	ok, err := f.model.SomeValues(element, first, second)
	if err != nil || !ok {
		return false, err
	}
	value, err := f.manager.RefGet(first)
	if err != nil {
		return false, err
	}
	value, err = f.manager.MkSequenceUnit(value)
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(first, value); err != nil {
		return false, err
	}
	value, err = f.manager.RefGet(second)
	if err != nil {
		return false, err
	}
	value, err = f.manager.MkSequenceUnit(value)
	if err != nil {
		return false, err
	}
	if err = f.manager.SetRef(second, value); err != nil {
		return false, err
	}
	return true, nil
}

func (f *SequenceFactory) freshString() (ast.ASTHandle, error) {
	for {
		text := f.delimiter + strconv.FormatUint(uint64(f.next), 16) + f.delimiter
		f.next++
		if _, found := f.strings[text]; found {
			continue
		}
		f.strings[text] = struct{}{}
		return f.manager.MkString(text)
	}
}

func (f *SequenceFactory) FreshValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if f == nil || f.manager == nil || f.model == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	stringSort, err := f.manager.IsStringSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if stringSort {
		return f.freshString()
	}
	sequence, regex, element, err := f.classify(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if regex {
		value, err := f.FreshValue(element)
		if err != nil || value.IsNull() {
			return ast.ASTHandle{}, err
		}
		return f.manager.MkSequenceToRegex(value)
	}
	if sequence {
		value, err := f.model.FreshValue(element)
		if err != nil || value.IsNull() {
			return ast.ASTHandle{}, err
		}
		return f.manager.MkSequenceUnit(value)
	}
	character, err := f.manager.IsCharacterSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if character {
		return f.manager.BitVectorNumeralUnsigned('a', 8)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}

func (f *SequenceFactory) RegisterValue(value ast.ASTHandle) error {
	if f == nil || f.manager == nil {
		return ast.ErrASTHandle
	}
	text, matched, err := f.manager.SequenceStringValue(value)
	if err != nil || !matched {
		return err
	}
	f.strings[text] = struct{}{}
	if !strings.Contains(text, f.delimiter) {
		return nil
	}
	for {
		f.delimiter += "!"
		collision := false
		for existing := range f.strings {
			if strings.Contains(existing, f.delimiter) {
				collision = true
				break
			}
		}
		if !collision {
			return nil
		}
	}
}

func (f *SequenceFactory) Close() error {
	if f != nil {
		f.manager = nil
		f.model = nil
		f.strings = nil
	}
	return nil
}
