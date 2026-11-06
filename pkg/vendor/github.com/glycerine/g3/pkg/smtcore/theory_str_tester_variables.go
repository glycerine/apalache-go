package smtcore

// Source: src/smt/theory_str.cpp mk_internal_lenTest_var and
// mk_internal_valTest_var. Reuse project SMT2 formatting and String skolems.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *SMTStringTheory) stringTesterSubjectName(subject ast.ASTHandle) (name string, result error) {
	m := t.state.manager
	format, err := m.SMT2PPExprTree(subject, nil)
	if err != nil {
		return "", err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = m.PrintFormat(stream, format, nil, nil, nil); err != nil {
		return "", err
	}
	return stream.String()
}

func (t *SMTStringTheory) freshStringLengthTester(subject ast.ASTHandle, tries int32) (ast.ASTHandle, error) {
	printed, err := t.stringTesterSubjectName(subject)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name := "$$_len_" + printed + "_" + strconv.FormatInt(int64(tries), 10) + "_" + strconv.FormatInt(int64(t.testers.lengthID), 10)
	t.testers.lengthID++
	variable, err := t.freshStringVariable(name)
	if err != nil {
		return variable, err
	}
	t.lengthTestVariables[variable] = struct{}{}
	return variable, t.retain(variable) // Native explicit push after mk_str_var's own root.
}

func (t *SMTStringTheory) freshStringValueTester(subject ast.ASTHandle, length, tries int32) (ast.ASTHandle, error) {
	printed, err := t.stringTesterSubjectName(subject)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name := "$$_val_" + printed + "_" + strconv.FormatInt(int64(length), 10) + "_" + strconv.FormatInt(int64(tries), 10) + "_" + strconv.FormatInt(int64(t.testers.valueID), 10)
	t.testers.valueID++
	variable, err := t.freshStringVariable(name)
	if err != nil {
		return variable, err
	}
	t.valueTestVariables[variable] = struct{}{}
	return variable, t.retain(variable)
}
