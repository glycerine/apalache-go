package ast

// Source: src/ast/ast.cpp:1958-2030; retained util/warning.cpp caller.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type checkSortFailure struct {
	message          string
	name             usymbol.Symbol
	position         uint32
	expected, actual ASTHandle
}

// CheckSorts performs the source shallow application check. Only the selected
// left/right associative child is checked recursively; other children are not.
func (m *ASTManager) CheckSorts(h ASTHandle, warnings *diagnostic.WarningService) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	var failure checkSortFailure
	var e error
	if h.IsNull() {
		failure.message = "expression is null"
	} else {
		var r astNodeRef
		r, e = m.internal(h)
		if e == nil {
			failure, e = m.checkSortsCore(r)
		}
	}
	m.mu.Unlock()
	if e != nil {
		return false, e
	}
	if failure.message == "" && failure.position == 0 {
		return true, nil
	}
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	message := failure.message
	if failure.position != 0 {
		name, err := m.Symbols.String(failure.name)
		if err != nil {
			return false, err
		}
		expected, err := m.checkSortText(failure.expected)
		if err != nil {
			return false, err
		}
		actual, err := m.checkSortText(failure.actual)
		if err != nil {
			return false, err
		}
		message = "invalid function application for " + name + ", sort mismatch on argument at position " + strconv.FormatUint(uint64(failure.position), 10) + ", expected " + expected + " but given " + actual
	}
	_ = warnings.Warning(message)
	return false, nil
}

func (m *ASTManager) checkSortText(h ASTHandle) (value string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	format, err := m.SMT2PPSort(h)
	if err != nil {
		return "", err
	}
	if err = m.PrintFormat(out, format, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}

func (m *ASTManager) checkSortsCore(r astNodeRef) (checkSortFailure, error) {
	if e := m.validNode(r); e != nil {
		return checkSortFailure{}, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return checkSortFailure{}, nil
	}
	a := m.appAt(r.slot)
	if e := m.validNode(a.decl); e != nil {
		return checkSortFailure{}, e
	}
	d := m.declAt(a.decl.slot)
	flags, e := m.funcFlags(a.decl)
	if e != nil {
		return checkSortFailure{}, e
	}
	associative := flags.LeftAssociative && flags.RightAssociative
	if !associative && a.count != d.arity {
		return checkSortFailure{message: "invalid function application, wrong number of arguments"}, nil
	}
	for i := uint32(0); i < a.count; i++ {
		index := i
		if associative {
			index = 0
		}
		if index >= d.arity {
			return checkSortFailure{}, ErrASTDomain
		}
		expected, e := m.readInternal(d.domain, index)
		if e != nil {
			return checkSortFailure{}, e
		}
		arg, e := m.readInternal(a.args, i)
		if e != nil {
			return checkSortFailure{}, e
		}
		actual, e := m.sortOf(arg)
		if e != nil {
			return checkSortFailure{}, e
		}
		compatible := expected == actual
		if !compatible && m.coercions && m.arithmeticPlugin != nil {
			x, _, xe := m.declIdentity(expected)
			y, _, ye := m.declIdentity(actual)
			if xe != nil {
				return checkSortFailure{}, xe
			}
			if ye != nil {
				return checkSortFailure{}, ye
			}
			compatible = x == m.arithmeticPlugin.family && y == x
		}
		if !compatible {
			return checkSortFailure{name: d.name, position: i + 1, expected: m.external(expected), actual: m.external(actual)}, nil
		}
	}
	if a.count == 2 && !flags.FlatAssociative {
		for _, child := range []struct {
			on    bool
			index uint32
		}{{flags.RightAssociative, 1}, {flags.LeftAssociative, 0}} {
			if child.on {
				arg, e := m.readInternal(a.args, child.index)
				if e != nil {
					return checkSortFailure{}, e
				}
				failure, e := m.checkSortsCore(arg)
				if e != nil || failure.message != "" || failure.position != 0 {
					return failure, e
				}
			}
		}
	}
	return checkSortFailure{}, nil
}
