package ast

// Source: src/util/scoped_numeral_vector.h:25-72; src/ast/fpa_decl_plugin.cpp:51-65.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Native slice storage owns concrete numerals. Copy and ordered destruction
// operate on the elements; only their numerical allocations use Runtime.
type astScopedFloatVector struct {
	copyChain                              unumeral.ArbitraryFloatValueCopyChain
	copyReceipt                            unumeral.ArbitraryFloatValueCopyReceipt
	copyFalseVisited, copyTrueAcknowledged bool
	copyRow                                uint32
	values                                 []unumeral.ArbitraryFloat
	manager                                *unumeral.ArbitraryFloatManager
	owner                                  *astFloatingPointPluginOwner
	destructorIndex                        uint32
	sourceDone                             bool
	bodyCause, sourceCause, ownCause       error
}

func (v *astScopedFloatVector) bind(p *astFloatingPointPluginOwner) {
	v.owner = p
	v.manager = p.numeric
}
func (v *astScopedFloatVector) guard() error {
	if v == nil || v.owner == nil || v.owner.self != v.owner || &v.owner.values != v || v.manager != v.owner.numeric {
		return ErrASTHandle
	}
	if v.sourceDone {
		return arena.ErrClosed
	}
	return nil
}
func (v *astScopedFloatVector) at(i uint32) (unumeral.ArbitraryFloat, error) {
	if e := v.guard(); e != nil {
		return unumeral.ArbitraryFloat{}, e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return unumeral.ArbitraryFloat{}, arena.ErrBounds
	}
	return v.values[i], nil
}
func (v *astScopedFloatVector) reserve(n uint32) error {
	if e := v.guard(); e != nil {
		return e
	}
	m := v.owner.owner
	if m == nil || m.Self != m || m.formatExecuting {
		return ErrASTHandle
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) > len(v.values) {
		v.values = append(v.values, make([]unumeral.ArbitraryFloat, int(n)-len(v.values))...)
	}
	return nil
}

func (v *astScopedFloatVector) set(i uint32, source *unumeral.ArbitraryFloat) error {
	if e := v.guard(); e != nil {
		return e
	}
	m := v.owner.owner
	if m.formatExecuting {
		return ErrASTPending
	}
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	var returned unumeral.ArbitraryFloatValueCopyReceipt
	if e := v.copyChain.Bind(v.manager); e != nil {
		return e
	}
	e := v.copyChain.CopyWithReceipt(&v.values[i], source, &returned)
	// Keep the latest allocating call for diagnostics; the chain retains all
	// older exact allocation receipts, including stopped calls.
	if returned != (unumeral.ArbitraryFloatValueCopyReceipt{}) {
		v.copyReceipt = returned
		v.copyRow = i
		v.copyFalseVisited = false
		v.copyTrueAcknowledged = false
	}
	if e != nil && v.bodyCause == nil {
		v.bodyCause = e
	}
	return e
}
func (v *astScopedFloatVector) del(i uint32) error {
	if e := v.guard(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	return v.manager.Del(&v.values[i])
}
func (v *astScopedFloatVector) copyFalsePhase() error {
	if v.copyFalseVisited {
		return nil
	}
	if e := v.copyChain.Cleanup(false); e != nil {
		return e
	}
	v.copyFalseVisited = true
	return nil
}
func (v *astScopedFloatVector) copyTruePhase() error {
	if v.copyTrueAcknowledged {
		return nil
	}
	if e := v.copyChain.Cleanup(true); e != nil {
		return e
	}
	if e := v.copyChain.End(); e != nil {
		return e
	}
	v.copyTrueAcknowledged = true
	return nil
}
func (v *astScopedFloatVector) destroy(own bool) error {
	if v.sourceDone {
		if own {
			if e := v.copyFalsePhase(); e != nil {
				return e
			}
			return v.copyTruePhase()
		}
		return nil
	}
	if e := v.guard(); e != nil {
		return e
	}
	if !own && v.sourceCause != nil {
		return v.sourceCause
	}
	stop := func(e error) error {
		if e != nil {
			if own {
				if v.ownCause == nil {
					v.ownCause = e
				}
			} else if v.sourceCause == nil {
				v.sourceCause = e
			}
		}
		return e
	}
	if own {
		if e := v.copyFalsePhase(); e != nil {
			return stop(e)
		}
	}
	for uint64(v.destructorIndex) < uint64(len(v.values)) {
		value := &v.values[v.destructorIndex]
		var e error
		if own {
			e = v.manager.CleanupValue(value)
		} else {
			e = v.manager.Del(value)
		}
		if e != nil {
			return stop(e)
		}
		v.destructorIndex++
	}
	if own {
		if e := v.copyTruePhase(); e != nil {
			return stop(e)
		}
	}
	v.values = nil
	v.sourceDone = true
	return nil
}
