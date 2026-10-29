package model

// Source: src/model/func_interp.cpp:71-326; src/model/func_interp.h:70-117.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// FunctionInterpretation retains the original ordered function graph, fallback
// expression and cached ITE interpretation. Entry matching calls AreEqual; no
// host map or alternative evaluation algorithm replaces the source traversal.
type FunctionInterpretation struct {
	self                          *FunctionInterpretation
	manager                       *ast.ASTManager
	arity                         uint32
	entries                       []*FunctionEntry
	pending                       *FunctionEntry
	fallback, cached, replacement ast.ASTHandle
	argsAreValues                 bool
	substitution                  *rewriter.VarSubstitution
	bytes                         uint64
	memoryTrace                   *diagnostic.TraceTags
	memoryTraceLive               bool
	closing, closed               bool
	drain                         uint32
	cause                         error
}

func NewFunctionInterpretation(m *ast.ASTManager, arity uint32) (*FunctionInterpretation, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(FunctionInterpretation{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	f := &FunctionInterpretation{manager: m, arity: arity, argsAreValues: true, bytes: bytes}
	f.self = f
	return f, nil
}
func (f *FunctionInterpretation) open() error {
	if f == nil || f.self != f {
		return ast.ErrASTHandle
	}
	if f.closed || f.closing {
		return arena.ErrClosed
	}
	if !f.replacement.IsNull() {
		return ast.ErrASTPending
	}
	if f.cause != nil {
		return f.cause
	}
	_, e := f.manager.ProofsEnabled()
	return e
}
func (f *FunctionInterpretation) Arity() uint32            { return f.arity }
func (f *FunctionInterpretation) NumEntries() uint32       { return uint32(len(f.entries)) }
func (f *FunctionInterpretation) ArgumentsAreValues() bool { return f.argsAreValues }
func (f *FunctionInterpretation) Entry(index uint32) (*FunctionEntry, error) {
	if e := f.open(); e != nil {
		return nil, e
	}
	if uint64(index) >= uint64(len(f.entries)) {
		return nil, arena.ErrBounds
	}
	return f.entries[index], nil
}
func (f *FunctionInterpretation) Else() (ast.ASTHandle, error) {
	if e := f.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	return f.fallback, nil
}
func (f *FunctionInterpretation) FindEntry(args []ast.ASTHandle) (*FunctionEntry, error) {
	return f.FindEntryArguments(ast.SliceArguments(args))
}

// FindEntryArguments borrows the traversal's concrete argument view.
func (f *FunctionInterpretation) FindEntryArguments(args ast.AppArguments) (*FunctionEntry, error) {
	if e := f.open(); e != nil {
		return nil, e
	}
	if uint64(args.Count()) != uint64(f.arity) {
		return nil, ast.ErrASTDomain
	}
	for _, entry := range f.entries {
		same, e := entry.equalArguments(args)
		if e != nil {
			return nil, e
		}
		if same {
			return entry, nil
		}
	}
	return nil, nil
}
func (f *FunctionInterpretation) resetCache() error {
	consumed, e := f.manager.DecRefConsumed(f.cached)
	if consumed {
		f.cached = ast.ASTHandle{}
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ast.ErrASTPending
	}
	return nil
}
func (f *FunctionInterpretation) InsertEntry(args []ast.ASTHandle, result ast.ASTHandle) error {
	if e := f.open(); e != nil {
		return e
	}
	if e := f.resetCache(); e != nil {
		return e
	}
	entry, e := f.FindEntry(args)
	if e != nil {
		return e
	}
	if entry != nil {
		return entry.setResult(result)
	}
	return f.InsertNewEntry(args, result)
}
func (f *FunctionInterpretation) InsertNewEntry(args []ast.ASTHandle, result ast.ASTHandle) (err error) {
	if err = f.open(); err != nil {
		return err
	}
	if uint64(len(args)) != uint64(f.arity) {
		return ast.ErrASTDomain
	}
	if err = f.resetCache(); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		entry, e := f.FindEntry(args)
		if e != nil {
			return e
		}
		if entry != nil {
			if e := f.traceDuplicateNewEntry(entry, args, result); e != nil {
				return e
			}
			return ast.ErrASTDomain
		}
	}
	f.pending, err = newFunctionEntry(f.manager, args, result)
	if err != nil {
		f.cause = err
		return err
	}
	if !f.pending.argsAreValues {
		f.argsAreValues = false
	}
	f.entries = append(f.entries, f.pending)
	f.pending = nil
	return nil
}

// EvalElse uses the original direct-order var_subst lexical owner.
func (f *FunctionInterpretation) EvalElse(args []ast.ASTHandle, out ast.ASTRef) (ok bool, err error) {
	if err = f.open(); err != nil {
		return false, err
	}
	if f.fallback.IsNull() {
		return false, nil
	}
	if uint64(len(args)) != uint64(f.arity) {
		return false, ast.ErrASTDomain
	}
	if f.substitution != nil {
		return false, arena.ErrClosed
	}
	f.substitution, err = rewriter.NewVarSubstitution(f.manager, false)
	defer func() {
		if f.substitution != nil {
			if e := f.substitution.Close(); e != nil {
				if err == nil {
					err = e
				}
				return
			}
			f.substitution = nil
		}
	}()
	if err != nil {
		return false, err
	}
	err = f.substitution.Substitute(f.fallback, args, out)
	return err == nil, err
}
func (f *FunctionInterpretation) IsPartial() (bool, error) {
	if e := f.open(); e != nil {
		return false, e
	}
	return f.fallback.IsNull(), nil
}
func (f *FunctionInterpretation) IsSimple() (bool, error) {
	if e := f.open(); e != nil {
		return false, e
	}
	if f.fallback.IsNull() {
		return true, nil
	}
	return f.manager.IsGround(f.fallback)
}
func (f *FunctionInterpretation) IsConstant() (bool, error) {
	if e := f.open(); e != nil {
		return false, e
	}
	if f.fallback.IsNull() {
		return false, nil
	}
	ground, e := f.manager.IsGround(f.fallback)
	if e != nil || !ground {
		return false, e
	}
	for _, entry := range f.entries {
		if entry.result != f.fallback {
			return false, nil
		}
	}
	return true, nil
}
func (f *FunctionInterpretation) Compress() error {
	if e := f.open(); e != nil {
		return e
	}
	if f.fallback.IsNull() || len(f.entries) == 0 {
		return nil
	}
	ground, e := f.manager.IsGround(f.fallback)
	if e != nil || !ground {
		return e
	}
	j := 0
	f.argsAreValues = true
	for _, entry := range f.entries {
		if entry.result != f.fallback {
			f.entries[j] = entry
			j++
			if !entry.argsAreValues {
				f.argsAreValues = false
			}
		} else {
			if e = entry.close(); e != nil {
				return e
			}
		}
		// Keep the old size through source traversal; clear stale duplicates only
		// when shrinking, after all source entry destruction has completed.
	}
	if j < len(f.entries) {
		if e = f.resetCache(); e != nil {
			return e
		}
		for i := j; i < len(f.entries); i++ {
			f.entries[i] = nil
		}
		f.entries = f.entries[:j]
	}
	return nil
}
func (f *FunctionInterpretation) Copy() (copy *FunctionInterpretation, err error) {
	return f.copyWithMemoryTrace(false)
}

func (f *FunctionInterpretation) copyWithMemoryTrace(traceMemory bool) (copy *FunctionInterpretation, err error) {
	if err = f.open(); err != nil {
		return nil, err
	}
	copy, err = NewFunctionInterpretation(f.manager, f.arity)
	if err != nil {
		return copy, err
	}
	if traceMemory {
		copy.traceCopiedFunctionAllocated(f.memoryTrace)
	}
	for _, entry := range f.entries {
		if err = copy.InsertNewEntry(entry.args, entry.result); err != nil {
			return copy, err
		}
	}
	err = copy.SetElse(f.fallback)
	return copy, err
}
func (f *FunctionInterpretation) Close() error {
	if f == nil || f.self != f {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	f.closing = true
	for f.drain < uint32(len(f.entries)) {
		entry := f.entries[f.drain]
		if entry != nil {
			if e := entry.close(); e != nil {
				return e
			}
		}
		f.drain++
	}
	for _, h := range []*ast.ASTHandle{&f.fallback, &f.cached} {
		consumed, e := f.manager.DecRefConsumed(*h)
		if consumed {
			*h = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	f.entries = nil
	if f.pending != nil {
		if e := f.pending.close(); e != nil {
			return e
		}
		f.pending = nil
	}
	if f.substitution != nil {
		if e := f.substitution.Close(); e != nil {
			return e
		}
		f.substitution = nil
	}
	if !f.replacement.IsNull() {
		consumed, e := f.manager.DecRefConsumed(f.replacement)
		if consumed {
			f.replacement = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	f.manager.Budget().Release(f.bytes)
	f.bytes = 0
	f.closed = true
	return nil
}
