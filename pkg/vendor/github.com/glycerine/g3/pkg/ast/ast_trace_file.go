package ast

// Source: src/ast/ast.cpp:1297-1334,1474-1479.

import (
	"os"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Actual cold file owner; its formatter is embedded, not a second heap wrapper.
// The format manager borrows this exact stream. Go file emission is direct;
// native filebuf buffering and host pathname allocations are OS adaptations.
// No reservation here claims to account for the transient host Go allocations.
type astTraceFile struct {
	self             *astTraceFile
	owner            *ASTManager
	budget           *arena.Budget
	header           uint64
	stream           utext.NumericTextStream
	trace            *udiag.TraceTags
	constructorCause error
	Closed           bool
}

func newASTTraceFile(b *arena.Budget, name uparams.ParamsStringView) (*astTraceFile, error) {
	return newASTTraceFileBorrowed(b, name, nil)
}
func newASTTraceFileBorrowed(b *arena.Budget, name TraceFilename, tags *udiag.TraceTags) (*astTraceFile, error) {
	header := uint64(reflect.TypeOf(astTraceFile{}).Size())
	if e := b.Reserve(header); e != nil {
		return nil, e
	}
	f := &astTraceFile{budget: b, header: header, trace: tags}
	f.self = f
	f.traceAllocated()
	f.stream.InitEmbeddedFile(b)
	n, e := name.Length()
	if e != nil {
		f.constructorCause = e
		return f, e
	}
	// fopen's filename adapter consumes the actual CString at this source leaf.
	// The host string is transient, not retained source storage or a Budget root.
	var path strings.Builder
	for i := uint64(0); i < n; i++ {
		ch, e := name.Byte(i)
		if e != nil {
			f.constructorCause = e
			return f, e
		}
		path.WriteByte(ch)
	}
	fd, e := os.OpenFile(path.String(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if e != nil {
		// Default fstream exceptions mask is empty: failed open is stream failbit,
		// not a constructor exception. Actual output uses the same nonthrowing engine.
		f.stream.BindFile(nil, true)
		return f, nil
	}
	f.stream.BindFile(fd, false)
	return f, nil
}
func (f *astTraceFile) Close() error {
	if f == nil || f.self != f {
		return ErrASTHandle
	}
	if f.Closed {
		return nil
	}
	// This owner is the sole authority for the acquired fd. The formatter's
	// ordinary Close only closes its own formatting storage and never its borrow.
	f.stream.CloseBorrowedFile()
	if e := f.stream.Close(); e != nil {
		return e
	}
	f.budget.Release(f.header)
	f.header = 0
	f.Closed = true
	f.traceDeallocated()
	return nil
}
