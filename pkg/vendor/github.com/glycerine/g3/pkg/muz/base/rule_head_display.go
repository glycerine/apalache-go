package base

// Source: dl_rule.cpp check_valid_head uses mk_pp.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *RuleManager) ruleHeadSourceDisplay(head ast.ASTHandle) (text string, resultErr error) {
	owner := &contextSMT2Owner{manager: m.manager}
	defer func() { resultErr = owner.finish(resultErr) }()
	var err error
	owner.formats, err = m.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	owner.format, err = owner.formats.NewRef(ast.ASTHandle{})
	if err != nil {
		return "", err
	}
	ctx, globals, err := m.manager.RewriterParameterServices()
	if err != nil {
		return "", err
	}
	var format ast.ASTHandle
	if ctx != nil && globals != nil {
		format, err = m.manager.SMT2PPExprTreeWithParams(head, nil, ctx, globals, nil)
	} else {
		format, err = m.manager.SMT2PPExprTree(head, nil)
	}
	if !format.IsNull() {
		err = errors.Join(err, owner.formats.SetRef(owner.format, format))
	}
	if err != nil {
		return "", err
	}
	owner.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.manager.Budget()})
	if err != nil {
		return "", err
	}
	if ctx != nil && globals != nil {
		empty := ctx.EmptyValue()
		err = m.manager.PrintFormat(owner.stream, format, ctx, &empty, globals)
	} else {
		err = m.manager.PrintFormat(owner.stream, format, nil, nil, nil)
	}
	if err != nil {
		return "", err
	}
	return owner.stream.String()
}
