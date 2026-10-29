package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1038,1075; src/util/trace.h.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows a tag stream for reached BV rewriter sites. Nil restores
// the source process-global stream.
func (p *BVRewriter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.polynomial.BindTrace(tags); err != nil {
		return err
	}
	p.trace = tags
	return nil
}

func (p *BVRewriter) traceUnsignedDivisionEntry(hiDiv0 bool) {
	tags := p.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("bv_udiv") {
		return
	}
	body := "hi_div0: 0\n"
	if hiDiv0 {
		body = "hi_div0: 1\n"
	}
	// std::ofstream failures set stream state and do not change the rewrite.
	_ = tags.Emit("bv_udiv", "mk_bv_udiv_core", "src/ast/rewriter/bv_rewriter.cpp", 1038, body)
}

func (p *BVRewriter) traceUnsignedDivisionToMultiply() {
	tags := p.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("udiv2mul") {
		return
	}
	_ = tags.Emit("udiv2mul", "mk_bv_udiv_core", "src/ast/rewriter/bv_rewriter.cpp", 1075, "using udiv2mul\n")
}
