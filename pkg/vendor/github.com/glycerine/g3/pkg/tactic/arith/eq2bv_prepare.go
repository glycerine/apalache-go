package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, cleanup_fd. This phase owns
// the fresh replacement constants; callers own the input roots and bounds.

import (
	"errors"
	"math/bits"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/bitutil"
)

type eq2bvReplacement struct {
	original         ast.ASTHandle
	fresh            ast.ASTRef
	maximum          uint32
	width            uint32
	negativeSentinel bool
}
type eq2bvReplacements struct {
	manager *ast.ASTManager
	entries []eq2bvReplacement
	nonfd   map[ast.ASTHandle]bool
}

func (p *eq2bvReplacements) Close() error {
	if p == nil || p.manager == nil {
		return nil
	}
	for len(p.entries) != 0 {
		i := len(p.entries) - 1
		if err := p.manager.DelRef(&p.entries[i].fresh); err != nil {
			return err
		}
		p.entries = p.entries[:i]
	}
	p.manager = nil
	return nil
}

// eq2bvWidth preserves the source's unsigned 32-bit power-of-two arithmetic.
func eq2bvWidth(maximum uint32, bounded bool) (uint32, uint32) {
	if !bounded {
		maximum++
	}
	v := bitutil.NextPowerOfTwo32(maximum)
	if v <= 1 {
		v = 2
	}
	if maximum == v {
		v *= 2
	}
	if v == 0 {
		return maximum, 0
	}
	return maximum, uint32(bits.Len32(v) - 1)
}

func eq2bvPrepare(m *ast.ASTManager, bounds *BoundManager, roots []ast.ASTHandle) (plan *eq2bvReplacements, err error) {
	if m == nil || bounds == nil || bounds.Manager() != m {
		return nil, ast.ErrASTHandle
	}
	nonfd := make(map[ast.ASTHandle]bool)
	maxima, err := eq2bvCollect(m, roots, nonfd)
	if err != nil {
		return nil, err
	}
	if len(maxima) != 0 {
		if _, err = m.RegisterBitVectorPlugin(); err != nil {
			return nil, err
		}
	}
	type indexed struct {
		h  ast.ASTHandle
		id uint32
	}
	ordered := make([]indexed, 0, len(maxima))
	for h := range maxima {
		id, e := m.GetASTID(h)
		if e != nil {
			return nil, e
		}
		ordered = append(ordered, indexed{h, id})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].id < ordered[j].id })
	plan = &eq2bvReplacements{manager: m, nonfd: nonfd}
	defer func() {
		if err != nil {
			err = errors.Join(err, plan.Close())
			plan = nil
		}
	}()
	for _, entry := range ordered {
		maximum := maxima[entry.h]
		upper, strict, found, e := bounds.Upper(entry.h)
		if e != nil {
			return plan, e
		}
		if found {
			if strict || !upper.IsInt() || upper.Sign() < 0 || upper.Num().BitLen() > 32 {
				return plan, ast.ErrASTDomain
			}
			value := uint32(upper.Num().Uint64())
			if value > maximum {
				maximum = value
			}
		}
		domainMaximum := maximum
		highDomain := maximum >= 1<<31
		maximum, width := eq2bvWidth(maximum, found)
		if highDomain {
			if found {
				width = 32
			} else {
				width = 33
			}
		}
		// The source sizes from equalities and the upper bound only. A larger
		// lower bound would otherwise wrap modulo the bit-vector width.
		lower, lowerStrict, hasLower, e := bounds.Lower(entry.h)
		if e != nil {
			return plan, e
		}
		if hasLower && lower.Sign() >= 0 {
			if lowerStrict || !lower.IsInt() {
				return plan, ast.ErrASTDomain
			}
			if lowerBits := lower.Num().BitLen(); lowerBits > int(width) {
				width = uint32(lowerBits)
			}
		}
		negativeSentinel := found && (!hasLower || lower.Sign() < 0)
		if negativeSentinel && width <= 32 && uint64(domainMaximum) == (uint64(1)<<width)-1 {
			width++
		}
		sortHandle, e := m.BitVectorSort(width)
		if e != nil {
			return plan, e
		}
		fresh, e := m.MkFreshConst("z", sortHandle)
		if e != nil {
			return plan, e
		}
		ref, e := m.NewRef(fresh)
		if e != nil {
			return plan, e
		}
		plan.entries = append(plan.entries, eq2bvReplacement{original: entry.h, fresh: ref, maximum: maximum, width: width, negativeSentinel: negativeSentinel})
	}
	return plan, nil
}
