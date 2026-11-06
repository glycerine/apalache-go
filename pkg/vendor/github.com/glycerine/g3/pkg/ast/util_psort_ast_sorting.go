package ast

// Source: src/util/sorting_network.h:597-1046.

import (
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func psortTraceText(w io.Writer, part string) { _, _ = io.WriteString(w, part) }
func psortTraceUint(w io.Writer, n uint32) {
	psortTraceText(w, strconv.FormatUint(uint64(n), 10))
}
func (s *PsortASTNetwork) tracePBEntry(line int, function string, body func(io.Writer) error) {
	tags := s.trace
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("pb", function, "src/util/sorting_network.h", line,
		func() bool { return true }, body)
}
func (s *PsortASTNetwork) traceCardEntry(k, n uint32) {
	s.tracePBEntry(598, "card", func(w io.Writer) error {
		psortTraceText(w, "card k: ")
		psortTraceUint(w, k)
		psortTraceText(w, " n: ")
		psortTraceUint(w, n)
		psortTraceText(w, "\n")
		return nil
	})
}

func (s *PsortASTNetwork) card(k, n uint32, p psortASTPointer, out *ASTLiteralVector) (err error) {
	s.traceCardEntry(k, n)
	if n <= k {
		return s.sorting(n, p, out)
	}
	direct, e := s.useDirectCard(k, n)
	if e != nil {
		return e
	}
	if direct {
		return s.directSort(k, n, p, out)
	}
	f, e := s.beginFrame(2)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	a, b := &f.vectors[0], &f.vectors[1]
	l := n / 2
	if e = s.card(k, l, p, a); e != nil {
		return e
	}
	if e = s.card(k, n-l, p.add(l), b); e != nil {
		return e
	}
	az, e := a.Size()
	if e != nil {
		return e
	}
	ap, e := psortVectorPointer(a)
	if e != nil {
		return e
	}
	bz, e := b.Size()
	if e != nil {
		return e
	}
	bp, e := psortVectorPointer(b)
	if e != nil {
		return e
	}
	return s.mergeSmall(k, az, ap, bz, bp, out)
}
func (s *PsortASTNetwork) sorting(n uint32, p psortASTPointer, out *ASTLiteralVector) (err error) {
	s.tracePBEntry(744, "sorting", func(w io.Writer) error {
		psortTraceText(w, "sorting: ")
		psortTraceUint(w, n)
		psortTraceText(w, "\n")
		return nil
	})
	switch n {
	case 0:
		return nil
	case 1:
		h, e := p.at(0)
		if e != nil {
			return e
		}
		return out.Push(h)
	case 2:
		return s.merge(1, p, 1, p.add(1), out)
	}
	direct, e := s.useDirectSort(n)
	if e != nil {
		return e
	}
	if direct {
		return s.directSort(n, n, p, out)
	}
	f, e := s.beginFrame(2)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	a, b := &f.vectors[0], &f.vectors[1]
	l := n / 2
	if e = s.sorting(l, p, a); e != nil {
		return e
	}
	if e = s.sorting(n-l, p.add(l), b); e != nil {
		return e
	}
	az, e := a.Size()
	if e != nil {
		return e
	}
	ap, e := psortVectorPointer(a)
	if e != nil {
		return e
	}
	bz, e := b.Size()
	if e != nil {
		return e
	}
	bp, e := psortVectorPointer(b)
	if e != nil {
		return e
	}
	return s.merge(az, ap, bz, bp, out)
}
func (s *PsortASTNetwork) merge(a uint32, ap psortASTPointer, b uint32, bp psortASTPointer, out *ASTLiteralVector) (err error) {
	s.tracePBEntry(640, "merge", func(w io.Writer) error {
		psortTraceText(w, "merge a: ")
		psortTraceUint(w, a)
		psortTraceText(w, " b: ")
		psortTraceUint(w, b)
		psortTraceText(w, "\n")
		return nil
	})
	if a == 1 && b == 1 {
		x, e := ap.at(0)
		if e != nil {
			return e
		}
		z, e := bp.at(0)
		if e != nil {
			return e
		}
		y1, e := s.maximum(x, z)
		if e != nil {
			return e
		}
		x, e = ap.at(0)
		if e != nil {
			return e
		}
		z, e = bp.at(0)
		if e != nil {
			return e
		}
		y2, e := s.minimum(x, z)
		if e != nil {
			return e
		}
		if e = out.Push(y1); e != nil {
			return e
		}
		if e = out.Push(y2); e != nil {
			return e
		}
		x, e = ap.at(0)
		if e != nil {
			return e
		}
		z, e = bp.at(0)
		if e != nil {
			return e
		}
		return s.compare(x, z, y1, y2)
	}
	if a == 0 {
		return s.appendPointer(out, b, bp)
	}
	if b == 0 {
		return s.appendPointer(out, a, ap)
	}
	direct, e := s.useDirectMerge(a, b, a+b)
	if e != nil {
		return e
	}
	if direct {
		return s.directMerge(a+b, a, ap, b, bp, out)
	}
	if a%2 == 0 && b%2 != 0 {
		return s.merge(b, bp, a, ap, out)
	}
	f, e := s.beginFrame(6)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	ea, oa, eb, ob, x, y := &f.vectors[0], &f.vectors[1], &f.vectors[2], &f.vectors[3], &f.vectors[4], &f.vectors[5]
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on && !(a > 1 || b > 1) {
		if e = s.assertion(664, "a > 1 || b > 1"); e != nil {
			return e
		}
	}
	if e = s.split(a, ap, ea, oa); e != nil {
		return e
	}
	if e = s.split(b, bp, eb, ob); e != nil {
		return e
	}
	if e = s.assertNotEmpty(ea, 667, "!even_a.empty()"); e != nil {
		return e
	}
	if e = s.assertNotEmpty(eb, 668, "!even_b.empty()"); e != nil {
		return e
	}
	if e = s.mergeVectors(ea, eb, x); e != nil {
		return e
	}
	if e = s.mergeVectors(oa, ob, y); e != nil {
		return e
	}
	return s.interleave(x, y, out)
}
func (s *PsortASTNetwork) mergeVectors(a, b, out *ASTLiteralVector) error {
	az, e := a.Size()
	if e != nil {
		return e
	}
	ap, e := psortVectorPointer(a)
	if e != nil {
		return e
	}
	bz, e := b.Size()
	if e != nil {
		return e
	}
	bp, e := psortVectorPointer(b)
	if e != nil {
		return e
	}
	return s.merge(az, ap, bz, bp, out)
}
func (s *PsortASTNetwork) split(n uint32, p psortASTPointer, even, odd *ASTLiteralVector) error {
	for i := uint32(0); i < n; i += 2 {
		h, e := p.at(i)
		if e != nil {
			return e
		}
		if e = even.Push(h); e != nil {
			return e
		}
	}
	for i := uint32(1); i < n; i += 2 {
		h, e := p.at(i)
		if e != nil {
			return e
		}
		if e = odd.Push(h); e != nil {
			return e
		}
	}
	return nil
}
func (s *PsortASTNetwork) assertNotEmpty(v *ASTLiteralVector, line uint32, text string) error {
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on {
		sz, e := v.Size()
		if e != nil {
			return e
		}
		if sz == 0 {
			return s.assertion(line, text)
		}
	}
	return nil
}
func (s *PsortASTNetwork) interleave(a, b, out *ASTLiteralVector) error {
	s.tracePBEntry(712, "interleave", func(w io.Writer) error {
		psortTraceText(w, "interleave: ")
		az, e := a.Size()
		if e != nil {
			return e
		}
		psortTraceUint(w, az)
		psortTraceText(w, " ")
		bz, e := b.Size()
		if e != nil {
			return e
		}
		psortTraceUint(w, bz)
		psortTraceText(w, "\n")
		return nil
	})
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on {
		x, e := a.Size()
		if e != nil {
			return e
		}
		y, e := b.Size()
		if e != nil {
			return e
		}
		if !(x >= y) {
			if e = s.assertion(713, "as.size() >= bs.size()"); e != nil {
				return e
			}
		}
	}
	on, e = s.assertions()
	if e != nil {
		return e
	}
	if on {
		x, e := a.Size()
		if e != nil {
			return e
		}
		y, e := b.Size()
		if e != nil {
			return e
		}
		if !(x <= y+2) {
			if e = s.assertion(714, "as.size() <= bs.size() + 2"); e != nil {
				return e
			}
		}
	}
	if e = s.assertNotEmpty(a, 715, "!as.empty()"); e != nil {
		return e
	}
	h, e := a.At(0)
	if e != nil {
		return e
	}
	if e = out.Push(h); e != nil {
		return e
	}
	az, e := a.Size()
	if e != nil {
		return e
	}
	bz, e := b.Size()
	if e != nil {
		return e
	}
	sz := psortMin(az-1, bz)
	for i := uint32(0); i < sz; i++ {
		x, e := a.At(i + 1)
		if e != nil {
			return e
		}
		z, e := b.At(i)
		if e != nil {
			return e
		}
		y1, e := s.maximum(x, z)
		if e != nil {
			return e
		}
		x, e = a.At(i + 1)
		if e != nil {
			return e
		}
		z, e = b.At(i)
		if e != nil {
			return e
		}
		y2, e := s.minimum(x, z)
		if e != nil {
			return e
		}
		x, e = a.At(i + 1)
		if e != nil {
			return e
		}
		z, e = b.At(i)
		if e != nil {
			return e
		}
		if e = s.compare(x, z, y1, y2); e != nil {
			return e
		}
		if e = out.Push(y1); e != nil {
			return e
		}
		if e = out.Push(y2); e != nil {
			return e
		}
	}
	az, e = a.Size()
	if e != nil {
		return e
	}
	bz, e = b.Size()
	if e != nil {
		return e
	}
	if az == bz {
		h, e := b.At(sz)
		if e != nil {
			return e
		}
		if e = out.Push(h); e != nil {
			return e
		}
	} else {
		az, e = a.Size()
		if e != nil {
			return e
		}
		bz, e = b.Size()
		if e != nil {
			return e
		}
		if az == bz+2 {
			h, e := a.At(sz + 1)
			if e != nil {
				return e
			}
			if e = out.Push(h); e != nil {
				return e
			}
		}
	}
	on, e = s.assertions()
	if e != nil {
		return e
	}
	if on {
		o, e := out.Size()
		if e != nil {
			return e
		}
		az, e := a.Size()
		if e != nil {
			return e
		}
		bz, e := b.Size()
		if e != nil {
			return e
		}
		if o != az+bz {
			return s.assertion(731, "out.size() == as.size() + bs.size()")
		}
	}
	return nil
}
func (s *PsortASTNetwork) mergeSmall(c, a uint32, ap psortASTPointer, b uint32, bp psortASTPointer, out *ASTLiteralVector) (err error) {
	s.tracePBEntry(805, "smerge", func(w io.Writer) error {
		psortTraceText(w, "smerge: c:")
		psortTraceUint(w, c)
		psortTraceText(w, " a:")
		psortTraceUint(w, a)
		psortTraceText(w, " b:")
		psortTraceUint(w, b)
		psortTraceText(w, "\n")
		return nil
	})
	if a == 1 && b == 1 && c == 1 {
		x, e := ap.at(0)
		if e != nil {
			return e
		}
		z, e := bp.at(0)
		if e != nil {
			return e
		}
		y, e := s.maximum(x, z)
		if e != nil {
			return e
		}
		mode, e := s.comparison()
		if e != nil {
			return e
		}
		if mode != psortGE {
			x, e := ap.at(0)
			if e != nil {
				return e
			}
			x, e = s.not(x)
			if e != nil {
				return e
			}
			if e = s.clause2(x, y); e != nil {
				return e
			}
			z, e := bp.at(0)
			if e != nil {
				return e
			}
			z, e = s.not(z)
			if e != nil {
				return e
			}
			if e = s.clause2(z, y); e != nil {
				return e
			}
		}
		mode, e = s.comparison()
		if e != nil {
			return e
		}
		if mode != psortLE {
			h, e := s.not(y)
			if e != nil {
				return e
			}
			x, e := ap.at(0)
			if e != nil {
				return e
			}
			z, e := bp.at(0)
			if e != nil {
				return e
			}
			if e = s.clause3(h, x, z); e != nil {
				return e
			}
		}
		if e = out.Push(y); e != nil {
			return e
		}
	} else if a == 0 {
		if e := s.appendPointer(out, psortMin(c, b), bp); e != nil {
			return e
		}
	} else if b == 0 {
		if e := s.appendPointer(out, psortMin(c, a), ap); e != nil {
			return e
		}
	} else if a > c {
		if e := s.mergeSmall(c, c, ap, b, bp, out); e != nil {
			return e
		}
	} else if b > c {
		if e := s.mergeSmall(c, a, ap, c, bp, out); e != nil {
			return e
		}
	} else if a+b <= c {
		if e := s.merge(a, ap, b, bp, out); e != nil {
			return e
		}
	} else {
		direct, e := s.useDirectMerge(a, b, c)
		if e != nil {
			return e
		}
		if direct {
			if e = s.directMerge(c, a, ap, b, bp, out); e != nil {
				return e
			}
		} else {
			f, e := s.beginFrame(6)
			if e != nil {
				return e
			}
			defer func() { err = s.finishFrame(f, err) }()
			if e = s.defaultVectors(f); e != nil {
				return e
			}
			ea, oa, eb, ob, x, y := &f.vectors[0], &f.vectors[1], &f.vectors[2], &f.vectors[3], &f.vectors[4], &f.vectors[5]
			if e = s.split(a, ap, ea, oa); e != nil {
				return e
			}
			if e = s.split(b, bp, eb, ob); e != nil {
				return e
			}
			if e = s.assertNotEmpty(ea, 844, "!even_a.empty()"); e != nil {
				return e
			}
			if e = s.assertNotEmpty(eb, 845, "!even_b.empty()"); e != nil {
				return e
			}
			c1, c2 := (c+1)/2, (c-1)/2
			if c%2 == 0 {
				c1 = 1 + c/2
				c2 = c / 2
			}
			if e = s.mergeSmallVectors(c1, ea, eb, x); e != nil {
				return e
			}
			if e = s.mergeSmallVectors(c2, oa, ob, y); e != nil {
				return e
			}
			if e = s.assertMergeResult(x, ea, eb, c1, 857, "out1.size() == std::min(even_a.size()+even_b.size(), c1)"); e != nil {
				return e
			}
			if e = s.assertMergeResult(y, oa, ob, c2, 858, "out2.size() == std::min(odd_a.size()+odd_b.size(), c2)"); e != nil {
				return e
			}
			var extra ASTHandle
			if c%2 == 0 {
				xz, e := x.Size()
				if e != nil {
					return e
				}
				z1, e := x.At(xz - 1)
				if e != nil {
					return e
				}
				yz, e := y.Size()
				if e != nil {
					return e
				}
				z2, e := y.At(yz - 1)
				if e != nil {
					return e
				}
				if e = x.PopBack(); e != nil {
					return e
				}
				if e = y.PopBack(); e != nil {
					return e
				}
				extra, e = s.maximum(z1, z2)
				if e != nil {
					return e
				}
				mode, e := s.comparison()
				if e != nil {
					return e
				}
				if mode != psortGE {
					h, e := s.not(z1)
					if e != nil {
						return e
					}
					if e = s.clause2(h, extra); e != nil {
						return e
					}
					h, e = s.not(z2)
					if e != nil {
						return e
					}
					if e = s.clause2(h, extra); e != nil {
						return e
					}
				}
				mode, e = s.comparison()
				if e != nil {
					return e
				}
				if mode != psortLE {
					h, e := s.not(extra)
					if e != nil {
						return e
					}
					if e = s.clause3(h, z1, z2); e != nil {
						return e
					}
				}
			}
			if e = s.interleave(x, y, out); e != nil {
				return e
			}
			if c%2 == 0 {
				if e = out.Push(extra); e != nil {
					return e
				}
			}
		}
	}
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on {
		sz, e := out.Size()
		if e != nil {
			return e
		}
		if sz != psortMin(a+b, c) {
			return s.assertion(884, "out.size() == std::min(a + b, c)")
		}
	}
	return nil
}
func (s *PsortASTNetwork) mergeSmallVectors(c uint32, a, b, out *ASTLiteralVector) error {
	az, e := a.Size()
	if e != nil {
		return e
	}
	ap, e := psortVectorPointer(a)
	if e != nil {
		return e
	}
	bz, e := b.Size()
	if e != nil {
		return e
	}
	bp, e := psortVectorPointer(b)
	if e != nil {
		return e
	}
	return s.mergeSmall(c, az, ap, bz, bp, out)
}
func (s *PsortASTNetwork) assertMergeResult(out, a, b *ASTLiteralVector, c, line uint32, text string) error {
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on {
		z, e := out.Size()
		if e != nil {
			return e
		}
		x, e := a.Size()
		if e != nil {
			return e
		}
		y, e := b.Size()
		if e != nil {
			return e
		}
		if z != psortMin(x+y, c) {
			return s.assertion(line, text)
		}
	}
	return nil
}
func (s *PsortASTNetwork) directMerge(c, a uint32, ap psortASTPointer, b uint32, bp psortASTPointer, out *ASTLiteralVector) (err error) {
	s.tracePBEntry(923, "dsmerge", func(w io.Writer) error {
		psortTraceText(w, "dsmerge: c:")
		psortTraceUint(w, c)
		psortTraceText(w, " a:")
		psortTraceUint(w, a)
		psortTraceText(w, " b:")
		psortTraceUint(w, b)
		psortTraceText(w, "\n")
		return nil
	})
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on && !(a <= c) {
		if e = s.assertion(924, "a <= c"); e != nil {
			return e
		}
	}
	on, e = s.assertions()
	if e != nil {
		return e
	}
	if on && !(b <= c) {
		if e = s.assertion(925, "b <= c"); e != nil {
			return e
		}
	}
	on, e = s.assertions()
	if e != nil {
		return e
	}
	if on && !(a+b >= c) {
		if e = s.assertion(926, "a + b >= c"); e != nil {
			return e
		}
	}
	for i := uint32(0); i < c; i++ {
		h, e := s.fresh()
		if e != nil {
			return e
		}
		if e = out.Push(h); e != nil {
			return e
		}
	}
	mode, e := s.comparison()
	if e != nil {
		return e
	}
	if mode != psortGE {
		for i := uint32(0); i < a; i++ {
			x, e := ap.at(i)
			if e != nil {
				return e
			}
			x, e = s.not(x)
			if e != nil {
				return e
			}
			y, e := out.At(i)
			if e != nil {
				return e
			}
			if e = s.clause2(x, y); e != nil {
				return e
			}
		}
		for i := uint32(0); i < b; i++ {
			x, e := bp.at(i)
			if e != nil {
				return e
			}
			x, e = s.not(x)
			if e != nil {
				return e
			}
			y, e := out.At(i)
			if e != nil {
				return e
			}
			if e = s.clause2(x, y); e != nil {
				return e
			}
		}
		for i := uint32(1); i <= a; i++ {
			for j := uint32(1); j <= b && i+j <= c; j++ {
				x, e := ap.at(i - 1)
				if e != nil {
					return e
				}
				x, e = s.not(x)
				if e != nil {
					return e
				}
				y, e := bp.at(j - 1)
				if e != nil {
					return e
				}
				y, e = s.not(y)
				if e != nil {
					return e
				}
				z, e := out.At(i + j - 1)
				if e != nil {
					return e
				}
				if e = s.clause3(x, y, z); e != nil {
					return e
				}
			}
		}
	}
	mode, e = s.comparison()
	if e != nil {
		return e
	}
	if mode != psortLE {
		f, e := s.beginFrame(1)
		if e != nil {
			return e
		}
		defer func() { err = s.finishFrame(f, err) }()
		if e = s.defaultVectors(f); e != nil {
			return e
		}
		ls := &f.vectors[0]
		for k := uint32(0); k < c; k++ {
			if e = ls.Reset(); e != nil {
				return e
			}
			h, e := out.At(k)
			if e != nil {
				return e
			}
			h, e = s.not(h)
			if e != nil {
				return e
			}
			if e = ls.Push(h); e != nil {
				return e
			}
			if a <= k {
				x, e := out.At(k)
				if e != nil {
					return e
				}
				x, e = s.not(x)
				if e != nil {
					return e
				}
				y, e := bp.at(k - a)
				if e != nil {
					return e
				}
				if e = s.clause2(x, y); e != nil {
					return e
				}
			}
			if b <= k {
				x, e := out.At(k)
				if e != nil {
					return e
				}
				x, e = s.not(x)
				if e != nil {
					return e
				}
				y, e := ap.at(k - b)
				if e != nil {
					return e
				}
				if e = s.clause2(x, y); e != nil {
					return e
				}
			}
			for i := uint32(0); i < psortMin(a, k+1); i++ {
				j := k - i
				on, e := s.assertions()
				if e != nil {
					return e
				}
				if on && !(i+j == k) {
					if e = s.assertion(956, "i + j == k"); e != nil {
						return e
					}
				}
				if j < b {
					x, e := ap.at(i)
					if e != nil {
						return e
					}
					if e = ls.Push(x); e != nil {
						return e
					}
					y, e := bp.at(j)
					if e != nil {
						return e
					}
					if e = ls.Push(y); e != nil {
						return e
					}
					if e = s.clauseVector(ls); e != nil {
						return e
					}
					if e = ls.PopBack(); e != nil {
						return e
					}
					if e = ls.PopBack(); e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}
func (s *PsortASTNetwork) directSort(m, n uint32, p psortASTPointer, out *ASTLiteralVector) (err error) {
	s.tracePBEntry(982, "dsorting", func(w io.Writer) error {
		psortTraceText(w, "dsorting m: ")
		psortTraceUint(w, m)
		psortTraceText(w, " n: ")
		psortTraceUint(w, n)
		psortTraceText(w, "\n")
		return nil
	})
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on && !(m <= n) {
		if e = s.assertion(983, "m <= n"); e != nil {
			return e
		}
	}
	f, e := s.beginFrame(1)
	if e != nil {
		return e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return e
	}
	lits := &f.vectors[0]
	for i := uint32(0); i < m; i++ {
		h, e := s.fresh()
		if e != nil {
			return e
		}
		if e = out.Push(h); e != nil {
			return e
		}
	}
	mode, e := s.comparison()
	if e != nil {
		return e
	}
	if mode != psortGE {
		for k := uint32(1); k <= m; k++ {
			h, e := out.At(k - 1)
			if e != nil {
				return e
			}
			if e = lits.Push(h); e != nil {
				return e
			}
			if e = s.addSubset(true, k, 0, lits, n, p); e != nil {
				return e
			}
			if e = lits.PopBack(); e != nil {
				return e
			}
		}
	}
	mode, e = s.comparison()
	if e != nil {
		return e
	}
	if mode != psortLE {
		for k := uint32(1); k <= m; k++ {
			h, e := out.At(k - 1)
			if e != nil {
				return e
			}
			h, e = s.not(h)
			if e != nil {
				return e
			}
			if e = lits.Push(h); e != nil {
				return e
			}
			if e = s.addSubset(false, n-k+1, 0, lits, n, p); e != nil {
				return e
			}
			if e = lits.PopBack(); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *PsortASTNetwork) addSubset(polarity bool, k, offset uint32, lits *ASTLiteralVector, n uint32, p psortASTPointer) error {
	on, e := s.assertions()
	if e != nil {
		return e
	}
	if on && !(k+offset <= n) {
		if e = s.assertion(1019, "k + offset <= n"); e != nil {
			return e
		}
	}
	if k == 0 {
		return s.clauseVector(lits)
	}
	for i := offset; i < n-k+1; i++ {
		h, e := p.at(i)
		if e != nil {
			return e
		}
		if polarity {
			h, e = s.not(h)
			if e != nil {
				return e
			}
		}
		if e = lits.Push(h); e != nil {
			return e
		}
		if e = s.addSubset(polarity, k-1, i+1, lits, n, p); e != nil {
			return e
		}
		if e = lits.PopBack(); e != nil {
			return e
		}
	}
	return nil
}
