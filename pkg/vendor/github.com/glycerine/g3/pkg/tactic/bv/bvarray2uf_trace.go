package bv

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
	"io"
)

func (t *BVArray2UFTactic) tags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}
func (t *BVArray2UFTactic) formatArray(h ast.ASTHandle) (text string, result error) {
	f, e := t.manager.SMT2PPExprTree(h, nil)
	if e != nil {
		if k, x := t.manager.GetKind(h); x == nil && k == ast.ASTFuncDecl {
			f, e = t.manager.SMT2FdeclRef(h)
		}
	}
	if e != nil {
		return "", e
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if e = t.manager.PrintFormat(out, f, nil, nil, nil); e != nil {
		return "", e
	}
	return out.String()
}
func (t *BVArray2UFTactic) tracePair(prefix string, a ast.ASTHandle, middle string, b ast.ASTHandle, line int) {
	tags := t.tags()
	_ = tags.EmitConditional("bvarray2uf_rw", "mk_uf_for_array", "src/tactic/bv/bvarray2uf_rewriter.cpp", line, func() bool { return true }, func(w io.Writer) error {
		x, e := t.formatArray(a)
		if e != nil {
			return e
		}
		y, e := t.formatArray(b)
		if e != nil {
			return e
		}
		_, e = io.WriteString(w, prefix+x+middle+y+"\n")
		return e
	})
}
