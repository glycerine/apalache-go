package api

// Source: src/api/api_context.cpp:284-307; src/util/warning.cpp:141-168.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

const apiSortErrorCode uint32 = 1 // Z3_SORT_ERROR in src/api/z3_api.h.

// setErrorCode publishes the code before invoking the borrowed handler, as
// context::set_error_code does. The full API context constructor supplies its
// default handler; this selected member owner has no default handler yet.
func (c *apiContextResults) setErrorCode(code uint32) {
	c.errorCode = code
	if code != 0 && c.errorHandler != nil {
		c.errorHandler(c, code)
	}
}

// checkSorts performs the source context's second warning after the AST
// manager's caught shallow check, then publishes Z3_SORT_ERROR. Constructors
// that already own an application may call it after saving the AST trail.
func (c *apiContextResults) checkSorts(h ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	valid, e := c.manager.CheckSorts(h, c.warnings)
	if e != nil || valid {
		return e
	}
	if h.IsNull() {
		// The source warns in ast_manager and then dereferences null in the
		// switch. A typed Go boundary preserves the first warning and error.
		c.setErrorCode(apiSortErrorCode)
		return nil
	}
	kind, e := c.manager.GetKind(h)
	if e != nil {
		return e
	}
	if kind == ast.ASTApp {
		message, e := c.sortApplicationWarning(h)
		if e != nil {
			return e
		}
		_ = c.warnings.Warning(message)
	}
	c.setErrorCode(apiSortErrorCode)
	return nil
}

func (c *apiContextResults) sortApplicationWarning(h ast.ASTHandle) (message string, result error) {
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	decl, e := c.manager.GetDecl(h)
	if e != nil {
		return "", e
	}
	formatted, e := c.manager.SMT2PPDeclareFun(decl)
	if e != nil {
		return "", e
	}
	if e = c.manager.PrintFormat(out, formatted, nil, nil, nil); e != nil {
		return "", e
	}
	if e = out.AppendCString(" applied to: "); e != nil {
		return "", e
	}
	n, e := c.manager.GetNumArgs(h)
	if e != nil {
		return "", e
	}
	if n > 1 {
		if e = out.AppendCString("\n"); e != nil {
			return "", e
		}
	}
	for i := uint32(0); i < n; i++ {
		arg, e := c.manager.GetArg(h, i)
		if e != nil {
			return "", e
		}
		if e = c.manager.DisplayBoundedLowLevel(out, arg, 3); e != nil {
			return "", e
		}
		if e = out.AppendCString(" of sort "); e != nil {
			return "", e
		}
		sort, e := c.manager.GetSort(arg)
		if e != nil {
			return "", e
		}
		formatted, e := c.manager.SMT2PPSort(sort)
		if e != nil {
			return "", e
		}
		if e = c.manager.PrintFormat(out, formatted, nil, nil, nil); e != nil {
			return "", e
		}
		if e = out.AppendCString("\n"); e != nil {
			return "", e
		}
	}
	return out.String()
}
