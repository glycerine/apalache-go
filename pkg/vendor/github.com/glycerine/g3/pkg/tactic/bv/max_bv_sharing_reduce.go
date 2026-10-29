package bv

// Source: src/tactic/bv/max_bv_sharing_tactic.cpp:29-226.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type expressionPair struct{ left, right ast.ASTHandle }

// SharingReducer is the concrete expr-pair cache and AC reducer used by the
// retained max-BV-sharing tactic. The manager is borrowed. Close releases the
// roots that keep cached operands and intermediate applications alive.
type SharingReducer struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	maxArgs uint32
	pairs   [4]map[expressionPair]struct{}
	roots   []ast.ASTRef
	trace   *diagnostic.TraceTags
	closed  bool
}

func NewSharingReducer(m *ast.ASTManager, family ast.FamilyID, maxArgs uint32) (*SharingReducer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	fid, e := m.RegisterBitVectorPlugin()
	if e != nil {
		return nil, e
	}
	if family != fid {
		return nil, ast.ErrASTDomain
	}
	return &SharingReducer{manager: m, family: family, maxArgs: maxArgs}, nil
}

func sharingIndex(kind int32) int {
	switch kind {
	case 4:
		return 0 // OP_BADD
	case 6:
		return 1 // OP_BMUL
	case 33:
		return 2 // OP_BXOR
	case 31:
		return 3 // OP_BOR
	default:
		return -1
	}
}

func (r *SharingReducer) keep(h ast.ASTHandle) error {
	ref, e := r.manager.NewRef(h)
	if e == nil {
		r.roots = append(r.roots, ref)
	}
	return e
}

func (r *SharingReducer) remember(index int, a, b ast.ASTHandle) error {
	p := expressionPair{a, b}
	if _, found := r.pairs[index][p]; found {
		return nil
	}
	if e := r.keep(a); e != nil {
		return e
	}
	if e := r.keep(b); e != nil {
		return e
	}
	if r.pairs[index] == nil {
		r.pairs[index] = make(map[expressionPair]struct{})
	}
	r.pairs[index][p] = struct{}{}
	return nil
}

func (r *SharingReducer) makePair(decl, a, b ast.ASTHandle) (ast.ASTHandle, error) {
	h, e := r.manager.MkApp(decl, []ast.ASTHandle{a, b})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return h, r.keep(h)
}

// ReduceApp returns changed=false for the source's BR_FAILED case. A changed
// result is borrowed from the reducer and must be rooted by its caller before
// Close. The ordered pair sets are separate for each of the four BV operators.
func (r *SharingReducer) ReduceApp(decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if r == nil || r.closed || r.manager == nil {
		return ast.ASTHandle{}, false, ast.ErrASTHandle
	}
	family, e := r.manager.GetFamilyID(decl)
	if e != nil {
		return ast.ASTHandle{}, false, e
	}
	kind, e := r.manager.GetDeclKind(decl)
	if e != nil {
		return ast.ASTHandle{}, false, e
	}
	index := sharingIndex(kind)
	if family != r.family || index < 0 || len(args) < 2 {
		return ast.ASTHandle{}, false, nil
	}
	if len(args) == 2 {
		n0, e := r.manager.IsBitVectorOperation(args[0], 0)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		n1, e := r.manager.IsBitVectorOperation(args[1], 0)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !n0 && !n1 {
			if e := r.remember(index, args[0], args[1]); e != nil {
				return ast.ASTHandle{}, false, e
			}
		}
		return ast.ASTHandle{}, false, nil
	}
	work := make([]ast.ASTHandle, 0, len(args))
	var numeral ast.ASTHandle
	first := false
	for i, arg := range args {
		isNumeral, e := r.manager.IsBitVectorOperation(arg, 0)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if numeral.IsNull() && isNumeral {
			numeral, first = arg, i == 0
		} else {
			work = append(work, arg)
		}
	}
	// Valid AC applications contain at least two arguments. With three or more
	// arguments, removing the one deferred numeral leaves a nonempty worklist.
	for len(work) > 1 && uint32(len(work)) < r.maxArgs {
		reused := false
		for i := 0; i+1 < len(work) && !reused; i++ {
			for j := i + 1; j < len(work); j++ {
				a, b := work[i], work[j]
				if _, ok := r.pairs[index][expressionPair{a, b}]; !ok {
					if _, ok = r.pairs[index][expressionPair{b, a}]; !ok {
						continue
					}
					a, b = b, a
				}
				h, e := r.makePair(decl, a, b)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				if e = r.traceDetail(132, fmt.Sprintf("reusing args: %d %d\n", i, j)); e != nil {
					return ast.ASTHandle{}, false, e
				}
				work[i] = h
				work = append(work[:j], work[j+1:]...)
				reused = true
				break
			}
		}
		if !reused {
			break
		}
	}
	if len(work) == 1 {
		if e = r.traceDetail(176, "tree-loop: num_args: 1\n"); e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	for len(work) > 1 {
		if e = r.traceDetail(176, fmt.Sprintf("tree-loop: num_args: %d\n", len(work))); e != nil {
			return ast.ASTHandle{}, false, e
		}
		next := make([]ast.ASTHandle, 0, (len(work)+1)/2)
		for i := 0; i < len(work); i += 2 {
			if i+1 == len(work) {
				next = append(next, work[i])
				continue
			}
			if e := r.remember(index, work[i], work[i+1]); e != nil {
				return ast.ASTHandle{}, false, e
			}
			h, e := r.makePair(decl, work[i], work[i+1])
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			next = append(next, h)
		}
		work = next
	}
	result := work[0]
	if !numeral.IsNull() {
		a, b := result, numeral
		if first {
			a, b = numeral, result
		}
		result, e = r.makePair(decl, a, b)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	return result, true, nil
}

func (r *SharingReducer) traceDetail(line int, text string) error {
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("bv_sharing_detail", "reduce_ac_app", "src/tactic/bv/max_bv_sharing_tactic.cpp", line, func() bool { return true }, func(w io.Writer) error { _, err := io.WriteString(w, text); return err })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (r *SharingReducer) clearPairs() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	for len(r.roots) > 0 {
		i := len(r.roots) - 1
		if r.roots[i] == (ast.ASTRef{}) {
			r.roots = r.roots[:i]
			continue
		}
		if e := r.manager.DelRef(&r.roots[i]); e != nil {
			return e
		}
		r.roots = r.roots[:i]
	}
	r.pairs = [4]map[expressionPair]struct{}{}
	return nil
}

func (r *SharingReducer) Close() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if err := r.clearPairs(); err != nil {
		return err
	}
	r.closed = true
	return nil
}
