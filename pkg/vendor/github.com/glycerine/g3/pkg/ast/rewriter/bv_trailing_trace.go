package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:39,46,51,87,95-96,346,362,370,372; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVTrailing) traceLifecycle(destructor bool) {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return
	}
	function, line, body := "imp", 39, "ctor\n"
	if destructor {
		function, line, body = "~imp", 46, "dtor\n"
	}
	_ = tags.Emit("bv-trailing", function, "src/ast/rewriter/bv_trailing.cpp", line, body)
}

func (p *BVTrailing) traceCount(arg ast.ASTHandle, min, max uint32) error {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	err := tags.EmitConditional("bv-trailing", "count_trailing", "src/ast/rewriter/bv_trailing.cpp", 87,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeBVTraceSMT2(p.manager, w, arg, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n:"+strconv.FormatUint(uint64(min), 10)+" - "+strconv.FormatUint(uint64(max), 10)+"\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *BVTrailing) traceEq(left, right ast.ASTHandle) error {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	err := tags.EmitConditional("bv-trailing", "eq_remove_trailing", "src/ast/rewriter/bv_trailing.cpp", 51,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeBVTraceSMT2(p.manager, w, left, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n=\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(p.manager, w, right, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *BVTrailing) traceRemove(arg ast.ASTHandle, out ast.ASTRef) error {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return nil
	}
	result, err := p.manager.RefGet(out)
	if err != nil {
		return err
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	line := 95
	if result.IsNull() {
		line = 96
	}
	err = tags.EmitConditional("bv-trailing", "remove_trailing", "src/ast/rewriter/bv_trailing.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeBVTraceSMT2(p.manager, w, arg, globals); err != nil {
				return err
			}
			if result.IsNull() {
				_, err := io.WriteString(w, "\n---> [EMPTY]\n")
				return err
			}
			if _, err := io.WriteString(w, "\n--->\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(p.manager, w, result, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *BVTrailing) traceCacheEntry(inserting bool, depth uint32, arg ast.ASTHandle, min, max uint32) error {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	function, line, prefix := "is_cached", 362, "cached@"
	if inserting {
		function, line, prefix = "cache", 346, "caching@"
	}
	err := tags.EmitConditional("bv-trailing", function, "src/ast/rewriter/bv_trailing.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, prefix+strconv.FormatUint(uint64(depth), 10)+": "); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(p.manager, w, arg, globals); err != nil {
				return err
			}
			width, err := p.manager.BitVectorSize(arg)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, "["+strconv.FormatUint(uint64(width), 10)+"]\n: "+
				strconv.FormatUint(uint64(min), 10)+"-"+strconv.FormatUint(uint64(max), 10)+"\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *BVTrailing) traceCacheReset(depth, condition uint32, actual bool) {
	tags := p.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv-trailing") {
		return
	}
	line, body := 370, "may reset cache "+strconv.FormatUint(uint64(depth), 10)+" "+strconv.FormatUint(uint64(condition), 10)+"\n"
	if actual {
		line, body = 372, "reset cache "+strconv.FormatUint(uint64(depth), 10)+"\n"
	}
	_ = tags.Emit("bv-trailing", "reset_cache", "src/ast/rewriter/bv_trailing.cpp", line, body)
}
