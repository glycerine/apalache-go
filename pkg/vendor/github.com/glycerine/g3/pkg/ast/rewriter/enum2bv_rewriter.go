package rewriter

// Source: src/ast/rewriter/enum2bv_rewriter.cpp;
// src/ast/rewriter/enum2bv_rewriter.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type EnumSortPredicate func(ast.ASTHandle) (bool, error)

type enum2BVEntry struct {
	original, bitVector, definition ast.ASTHandle
	owned                           [3]ast.ASTRef
}

type enum2BVConfiguration struct {
	DefaultConfiguration
	owner   *Enum2BVRewriter
	utility *ast.DatatypeUtil
	sorts   []ast.ASTHandle // source rw_cfg::m_sorts borrowed member buffer
}

type Enum2BVRewriter struct {
	self                *Enum2BVRewriter
	paramsContext       *uparams.ParamsContext
	parameters          uparams.ParamsRef
	parametersDecrement uparams.ParamsDecrementReceipt
	cleanupParameters   uparams.ParamsRef
	cleanupDecrement    uparams.ParamsDecrementReceipt
	manager             *ast.ASTManager
	utility             *ast.DatatypeUtil
	config              enum2BVConfiguration
	traversal           *Traversal
	enum2BV             map[ast.ASTHandle]ast.ASTHandle
	bv2Enum             map[ast.ASTHandle]ast.ASTHandle
	enum2Def            map[ast.ASTHandle]ast.ASTHandle
	entries             []enum2BVEntry
	scopeLimits         []int
	bounds              []ast.ASTHandle
	boundRefs           []ast.ASTRef
	predicate           EnumSortPredicate
	translated          uint32
	header              uint64
	closed              bool
	closing             bool
	pending             []enum2BVPending
}

func NewEnum2BVRewriter(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*Enum2BVRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if c == nil || c.SymbolsField != m.Symbols || c.NumeralsField != m.NumeralService() || c.BudgetField != m.Budget() {
		return nil, uparams.ErrParamsOwner
	}
	bytes := uint64(reflect.TypeOf(Enum2BVRewriter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	r := &Enum2BVRewriter{manager: m, paramsContext: c, header: bytes, closing: true}
	r.self = r
	if err := r.initMembers(p); err != nil {
		closeErr := r.Close()
		if closeErr != nil {
			return r, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return r, nil
}

func (r *Enum2BVRewriter) initMembers(p *uparams.ParamsRef) (err error) {
	r.parameters, err = r.paramsContext.NewValue()
	if err != nil {
		return err
	}
	if err = r.paramsContext.Assign(&r.parameters, p); err != nil {
		return err
	}
	r.enum2BV = make(map[ast.ASTHandle]ast.ASTHandle)
	r.bv2Enum = make(map[ast.ASTHandle]ast.ASTHandle)
	r.enum2Def = make(map[ast.ASTHandle]ast.ASTHandle)
	r.config = enum2BVConfiguration{owner: r}
	r.config.defaults.manager = r.manager
	r.utility, err = ast.NewDatatypeUtil(r.manager)
	if err != nil {
		return err
	}
	r.config.utility, err = ast.NewDatatypeUtil(r.manager)
	if err != nil {
		return err
	}
	proofs, err := r.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	// rw captures m.proofs_enabled() when its rewriter_tpl base is built.
	if proofs {
		r.traversal, err = NewProofTraversal(r.manager, &r.config)
	} else {
		r.traversal, err = NewTraversal(r.manager, &r.config)
	}
	if err != nil {
		return err
	}
	r.closing = false
	return nil
}

func (r *Enum2BVRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.closing || r.manager == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func enum2BVWidth(count uint32) uint32 {
	width := uint32(1)
	for width < 32 && uint64(1)<<width < uint64(count) {
		width++
	}
	return width
}

func enum2BVPowerOfTwo(value uint32) bool { return value != 0 && value&(value-1) == 0 }

func (r *Enum2BVRewriter) isFinite(sort ast.ASTHandle) (bool, error) {
	yes, err := r.utility.IsEnumSort(sort)
	if err != nil || !yes || r.predicate == nil {
		return yes, err
	}
	return r.predicate(sort)
}

func (r *Enum2BVRewriter) reduceArgument(argument ast.ASTHandle, result *ast.ASTRef) (bool, error) {
	m := r.manager
	sort, err := m.GetSort(argument)
	if err != nil {
		return false, err
	}
	finite, err := r.isFinite(sort)
	if err != nil || !finite {
		return false, err
	}
	count, err := m.DatatypeNumConstructors(sort)
	if err != nil {
		return false, err
	}
	width := enum2BVWidth(count)
	kind, err := m.GetKind(argument)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		index, err := m.GetVarIndex(argument)
		if err != nil {
			return false, err
		}
		bvSort, err := m.BitVectorSort(width)
		if err != nil {
			return false, err
		}
		value, err := m.MkVar(index, bvSort)
		if err != nil {
			return false, err
		}
		err = enum2BVAssign(m, result, value)
		return err == nil, err
	}
	if err = enum2BVAppAssertion(kind); err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, r.throwNonFD(argument)
	}
	decl, err := m.GetDecl(argument)
	if err != nil {
		return false, err
	}
	if index, matched, err := m.DatatypeConstructorIndex(decl); err != nil {
		return false, err
	} else if matched {
		value, err := m.BitVectorNumeralUnsigned(index, width)
		if err != nil {
			return false, err
		}
		if err = enum2BVAssign(m, result, value); err != nil {
			return false, err
		}
		r.translated++
		return true, nil
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return false, err
	}
	arity, err := m.GetArity(decl)
	if err != nil {
		return false, err
	}
	if family != ast.NullFamilyID || arity != 0 {
		return false, r.throwNonFD(argument)
	}
	if fresh, found := r.enum2BV[decl]; found {
		value, err := m.MkApp(fresh, nil)
		if err != nil {
			return false, err
		}
		err = enum2BVAssign(m, result, value)
		return err == nil, err
	}
	if err = r.reduceFreshConstant(decl, sort, result, count, width); err != nil {
		return false, err
	}
	r.translated++
	return true, nil
}

func (r *Enum2BVRewriter) reduceFreshConstant(decl, sort ast.ASTHandle, result *ast.ASTRef, count, width uint32) (err error) {
	m := r.manager
	bvSort, err := m.BitVectorSort(width)
	if err != nil {
		return err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return err
	}
	text, err := m.Symbols.Display(name)
	if err != nil {
		return err
	}
	fresh, err := m.MkFreshConst(text, bvSort)
	if err != nil {
		return err
	}
	if err = enum2BVAssign(m, result, fresh); err != nil {
		return err
	}
	freshDecl, err := m.GetDecl(fresh)
	if err != nil {
		return err
	}
	if !enum2BVPowerOfTwo(count) || count == 1 {
		upper, err := m.BitVectorNumeralUnsigned(count-1, width)
		if err != nil {
			return err
		}
		bound, err := m.MkBitVectorULE(fresh, upper)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(bound)
		if err != nil {
			return err
		}
		r.bounds = append(r.bounds, bound)
		r.boundRefs = append(r.boundRefs, ref)
	}
	definition := &enum2BVScalarScope{manager: m}
	defer r.finishScalar(definition, &err)
	constructors, err := r.config.utility.Constructors(sort)
	if err != nil {
		return err
	}
	value, err := m.MkApp(constructors[count-1], nil)
	if err != nil {
		return err
	}
	if err = enum2BVAssign(m, &definition.ref, value); err != nil {
		return err
	}
	for i := count - 1; i > 0; {
		i--
		numeral, err := m.BitVectorNumeralUnsigned(i, width)
		if err != nil {
			return err
		}
		equal, err := m.MkEq(fresh, numeral)
		if err != nil {
			return err
		}
		constructor, err := m.MkApp(constructors[i], nil)
		if err != nil {
			return err
		}
		old, err := m.RefGet(definition.ref)
		if err != nil {
			return err
		}
		value, err = m.MkIte(equal, constructor, old)
		if err != nil {
			return err
		}
		if err = enum2BVAssign(m, &definition.ref, value); err != nil {
			return err
		}
	}
	value, err = m.RefGet(definition.ref)
	if err != nil {
		return err
	}
	// The source publishes all borrowed maps before pinning consts, BVs,
	// then definitions. A later refusal retains its actual owning prefix.
	r.enum2Def[decl] = value
	r.enum2BV[decl] = freshDecl
	r.bv2Enum[freshDecl] = decl
	r.entries = append(r.entries, enum2BVEntry{original: decl, bitVector: freshDecl, definition: value})
	entry := &r.entries[len(r.entries)-1]
	for i, h := range []ast.ASTHandle{decl, freshDecl, value} {
		entry.owned[i], err = m.NewRef(h)
		if err != nil {
			r.closing = true
			return err
		}
	}
	return nil
}

func (c *enum2BVConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (status RewriteStatus, err error) {
	r := c.owner
	m := r.manager
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	scope := &enum2BVAppScope{manager: m}
	defer r.finishApp(scope, &err)
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := m.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family == 0 && kind == ast.ASTOpEq && args.Count() == 2 {
		a, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		yes, err := r.reduceArgument(a, &scope.a0)
		if err != nil {
			return RewriteFailed, err
		}
		if yes {
			b, err := args.At(1)
			if err != nil {
				return RewriteFailed, err
			}
			yes, err = r.reduceArgument(b, &scope.a1)
			if err != nil {
				return RewriteFailed, err
			}
			if yes {
				a, err = m.RefGet(scope.a0)
				if err != nil {
					return RewriteFailed, err
				}
				b, err = m.RefGet(scope.a1)
				if err != nil {
					return RewriteFailed, err
				}
				value, err := m.MkEq(a, b)
				if err != nil {
					return RewriteFailed, err
				}
				return RewriteDone, m.SetRef(out, value)
			}
		}
	} else if family == 0 && kind == ast.ASTOpDistinct {
		yes, err := r.reduceArguments(args, scope)
		if err != nil {
			return RewriteFailed, err
		}
		if yes {
			value, err := m.MkDistinct(scope.values)
			if err != nil {
				return RewriteFailed, err
			}
			return RewriteDone, m.SetRef(out, value)
		}
	} else if _, yes, err := m.DatatypeRecognizerConstructor(decl); err != nil {
		return RewriteFailed, err
	} else if yes && args.Count() == 1 {
		a, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		yes, err = r.reduceArgument(a, &scope.a0)
		if err != nil {
			return RewriteFailed, err
		}
		if yes {
			index, _, err := m.DatatypeRecognizerConstructorIndex(decl)
			if err != nil {
				return RewriteFailed, err
			}
			a, err = m.RefGet(scope.a0)
			if err != nil {
				return RewriteFailed, err
			}
			sort, err := m.GetSort(a)
			if err != nil {
				return RewriteFailed, err
			}
			width, err := m.BitVectorSize(sort)
			if err != nil {
				return RewriteFailed, err
			}
			numeral, err := m.BitVectorNumeralUnsigned(index, width)
			if err != nil {
				return RewriteFailed, err
			}
			if err = enum2BVAssign(m, &scope.a1, numeral); err != nil {
				return RewriteFailed, err
			}
			value, err := m.MkEq(a, numeral)
			if err != nil {
				return RewriteFailed, err
			}
			return RewriteDone, m.SetRef(out, value)
		}
	}
	for i := uint32(0); i < args.Count(); i++ {
		a, err := args.At(i)
		if err != nil {
			return RewriteFailed, err
		}
		sort, err := m.GetSort(a)
		if err != nil {
			return RewriteFailed, err
		}
		finite, err := r.isFinite(sort)
		if err != nil {
			return RewriteFailed, err
		}
		if finite {
			return RewriteFailed, r.throwNonFD(a)
		}
	}
	return RewriteFailed, nil
}

func enum2BVArgumentSlice(arguments ast.AppArguments) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, arguments.Count())
	for i := range result {
		value, err := arguments.At(uint32(i))
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (c *enum2BVConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (changed bool, err error) {
	r := c.owner
	m := r.manager
	if err = r.open(); err != nil {
		return false, err
	}
	c.sorts = c.sorts[:0]
	scope := &enum2BVQuantifierScope{manager: m}
	defer r.finishQuantifier(scope, &err)
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	found := false
	for i := uint32(0); i < count; i++ {
		sort, err := m.GetDeclarationSort(q, i)
		if err != nil {
			return false, err
		}
		finite, err := r.isFinite(sort)
		if err != nil {
			return false, err
		}
		if finite {
			nc, err := m.DatatypeNumConstructors(sort)
			if err != nil {
				return false, err
			}
			bvSort, err := m.BitVectorSort(enum2BVWidth(nc))
			if err != nil {
				return false, err
			}
			c.sorts = append(c.sorts, bvSort)
			nc, err = m.DatatypeNumConstructors(sort)
			if err != nil {
				return false, err
			}
			if !enum2BVPowerOfTwo(nc) || nc == 1 {
				variable, err := m.MkVar(count-i-1, c.sorts[i])
				if err != nil {
					return false, err
				}
				upper, err := m.BitVectorNumeralUnsigned(nc-1, enum2BVWidth(nc))
				if err != nil {
					return false, err
				}
				bound, err := m.MkBitVectorULE(variable, upper)
				if err != nil {
					return false, err
				}
				if err = scope.push(bound); err != nil {
					return false, err
				}
			}
			found = true
		} else {
			c.sorts = append(c.sorts, sort)
		}
	}
	if !found {
		return false, nil
	}
	if err = enum2BVAssign(m, &scope.body, body); err != nil {
		return false, err
	}
	// Source tmp is never assigned and therefore owns no reference record.
	forall, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	if len(scope.bounds) != 0 {
		if forall {
			condition, err := m.MkAndArray(scope.values)
			if err != nil {
				return false, err
			}
			old, err := m.RefGet(scope.body)
			if err != nil {
				return false, err
			}
			value, err := m.MkImplies(condition, old)
			if err != nil {
				return false, err
			}
			if err = enum2BVAssign(m, &scope.body, value); err != nil {
				return false, err
			}
		} else {
			old, err := m.RefGet(scope.body)
			if err != nil {
				return false, err
			}
			if err = scope.push(old); err != nil {
				return false, err
			}
			value, err := m.MkAndArray(scope.values)
			if err != nil {
				return false, err
			}
			if err = enum2BVAssign(m, &scope.body, value); err != nil {
				return false, err
			}
		}
	}
	newPatterns, err := enum2BVArgumentSlice(patterns)
	if err != nil {
		return false, err
	}
	newNoPatterns, err := enum2BVArgumentSlice(noPatterns)
	if err != nil {
		return false, err
	}
	names := make([]usymbol.Symbol, count)
	for i := uint32(0); i < count; i++ {
		names[i], err = m.GetDeclarationName(q, i)
		if err != nil {
			return false, err
		}
	}
	weight, err := m.GetQuantifierWeight(q)
	if err != nil {
		return false, err
	}
	qid, skid, err := m.GetQuantifierIDs(q)
	if err != nil {
		return false, err
	}
	newBody, err := m.RefGet(scope.body)
	if err != nil {
		return false, err
	}
	value, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: c.sorts, Names: names, Body: newBody, Weight: weight, QID: qid, SkolemID: skid, Patterns: newPatterns, NoPatterns: newNoPatterns})
	if err != nil {
		return false, err
	}
	if err = m.SetRef(out, value); err != nil {
		return false, err
	}
	if err = m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Enum2BVRewriter) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	if r.traversal.engine.core.proofGen {
		return r.traversal.RewriteWithProof(root, out, proof)
	}
	return r.traversal.Rewrite(root, out)
}

func (r *Enum2BVRewriter) Push() error {
	if err := r.open(); err != nil {
		return err
	}
	r.scopeLimits = append(r.scopeLimits, len(r.entries))
	return nil
}

func (r *Enum2BVRewriter) Pop(scopes uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := enum2BVSourceAssert(243, "m_bounds.empty()", func() bool { return len(r.bounds) == 0 }); err != nil {
		return err
	}
	if scopes != 0 {
		if err := enum2BVSourceAssert(245, "num_scopes <= m_enum_consts_lim.size()", func() bool { return int(scopes) <= len(r.scopeLimits) }); err != nil {
			return err
		}
		// Native out-of-range scope subtraction/indexing is outside the
		// source precondition; checked Go reports it without unsafe indexing.
		if int(scopes) > len(r.scopeLimits) {
			return ast.ErrASTDomain
		}
		level := len(r.scopeLimits) - int(scopes)
		limit := r.scopeLimits[level]
		for i := len(r.entries) - 1; i >= limit; i-- {
			entry := &r.entries[i]
			delete(r.enum2BV, entry.original)
			delete(r.bv2Enum, entry.bitVector)
			delete(r.enum2Def, entry.original)
		}
		for _, column := range []int{0, 2, 1} {
			for i := limit; i < len(r.entries); i++ {
				if r.entries[i].owned[column] == (ast.ASTRef{}) {
					continue
				}
				if err := r.manager.DelRef(&r.entries[i].owned[column]); err != nil {
					r.closing = true
					return err
				}
			}
		}
		r.entries = r.entries[:limit]
		r.scopeLimits = r.scopeLimits[:level]
	}
	return r.traversal.Reset()
}

// FlushSideConstraints appends caller-owned references before resetting the
// source m_bounds vector. Acquired outputs survive Cleanup and Close.
func (r *Enum2BVRewriter) FlushSideConstraints(out *[]ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTDomain
	}
	for _, bound := range r.bounds {
		ref, err := r.manager.NewRef(bound)
		if err != nil {
			return err
		}
		*out = append(*out, ref)
	}
	for i := range r.boundRefs {
		if r.boundRefs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := r.manager.DelRef(&r.boundRefs[i]); err != nil {
			r.closing = true
			return err
		}
	}
	r.bounds, r.boundRefs = nil, nil
	return nil
}

func (r *Enum2BVRewriter) SetSortPredicate(predicate EnumSortPredicate) error {
	if err := r.open(); err != nil {
		return err
	}
	r.predicate = predicate
	return nil
}
func (r *Enum2BVRewriter) UpdateParams(_ *uparams.ParamsRef) error { return r.open() }

// The public source cleanup destroys and reconstructs imp, including both
// datatype utilities, rather than only resetting the traversal.
func (r *Enum2BVRewriter) Cleanup() error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	r.closing = true
	// Public cleanup's local params_ref outlives destruction of the old imp.
	if r.cleanupParameters == (uparams.ParamsRef{}) {
		var err error
		r.cleanupParameters, err = r.paramsContext.NewValue()
		if err != nil {
			return err
		}
		if err = r.paramsContext.Assign(&r.cleanupParameters, &r.parameters); err != nil {
			return errors.Join(err, r.endParameters(&r.cleanupParameters, &r.cleanupDecrement))
		}
	}
	if err := r.releaseMembers(); err != nil {
		return err
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	bytes := uint64(reflect.TypeOf(Enum2BVRewriter{}).Size())
	if err := r.manager.Budget().Reserve(bytes); err != nil {
		return err
	}
	r.header = bytes
	if err := r.initMembers(&r.cleanupParameters); err != nil {
		return err
	}
	r.closing = true
	if err := r.endParameters(&r.cleanupParameters, &r.cleanupDecrement); err != nil {
		return err
	}
	r.closing = false
	return nil
}

// Derived rw_cfg members die before the traversal base, then imp's vectors
// die in reverse member order. Each project ref_vector retires forward.
func (r *Enum2BVRewriter) releaseMembers() error {
	for len(r.pending) != 0 {
		p := &r.pending[0]
		var err error
		if p.printer != nil {
			err = p.printer.close()
		} else if p.quantifier != nil {
			err = p.quantifier.close()
		} else if p.scalar != nil {
			err = p.scalar.close()
		} else {
			err = p.app.close()
		}
		if err != nil {
			return err
		}
		r.pending = r.pending[1:]
	}

	// rw_cfg::m_sorts is the last declared derived member.
	r.config.sorts = nil
	if r.config.utility != nil {
		if err := r.config.utility.Close(); err != nil {
			return err
		}
		r.config.utility = nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	for _, column := range []int{2, 1, 0} {
		for i := range r.entries {
			if r.entries[i].owned[column] == (ast.ASTRef{}) {
				continue
			}
			if err := r.manager.DelRef(&r.entries[i].owned[column]); err != nil {
				return err
			}
		}
	}
	if r.utility != nil {
		if err := r.utility.Close(); err != nil {
			return err
		}
		r.utility = nil
	}
	for i := range r.boundRefs {
		if r.boundRefs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := r.manager.DelRef(&r.boundRefs[i]); err != nil {
			return err
		}
	}
	r.enum2BV, r.bv2Enum, r.enum2Def = nil, nil, nil
	r.entries, r.scopeLimits, r.bounds, r.boundRefs = nil, nil, nil, nil
	r.predicate, r.translated = nil, 0
	return r.endParameters(&r.parameters, &r.parametersDecrement)
}

func (r *Enum2BVRewriter) NumTranslated() (uint32, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	return r.translated, nil
}
func (r *Enum2BVRewriter) NumSteps() (uint32, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	return r.traversal.NumSteps()
}
func (r *Enum2BVRewriter) EnumToBitVector() map[ast.ASTHandle]ast.ASTHandle {
	if r == nil || r.closed || r.closing {
		return nil
	}
	out := make(map[ast.ASTHandle]ast.ASTHandle, len(r.enum2BV))
	for k, v := range r.enum2BV {
		out[k] = v
	}
	return out
}
func (r *Enum2BVRewriter) BitVectorToEnum() map[ast.ASTHandle]ast.ASTHandle {
	if r == nil || r.closed || r.closing {
		return nil
	}
	out := make(map[ast.ASTHandle]ast.ASTHandle, len(r.bv2Enum))
	for k, v := range r.bv2Enum {
		out[k] = v
	}
	return out
}
func (r *Enum2BVRewriter) EnumDefinitions() map[ast.ASTHandle]ast.ASTHandle {
	if r == nil || r.closed || r.closing {
		return nil
	}
	out := make(map[ast.ASTHandle]ast.ASTHandle, len(r.enum2Def))
	for k, v := range r.enum2Def {
		out[k] = v
	}
	return out
}

func (r *Enum2BVRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.closing = true
	if err := r.releaseMembers(); err != nil {
		return err
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	if err := r.endParameters(&r.cleanupParameters, &r.cleanupDecrement); err != nil {
		return err
	}
	r.closed = true
	return nil
}

var _ Configuration = (*enum2BVConfiguration)(nil)

// A receipt continues only the exact parameter destruction already reached.
func (r *Enum2BVRewriter) endParameters(p *uparams.ParamsRef, receipt *uparams.ParamsDecrementReceipt) error {
	if *p == (uparams.ParamsRef{}) {
		return nil
	}
	var err error
	if receipt.Invocation() != 0 {
		err = r.paramsContext.CompleteDecrement(p, *receipt)
	} else {
		err = r.paramsContext.DelTracked(p, receipt)
	}
	if err == nil {
		*receipt = uparams.ParamsDecrementReceipt{}
	}
	return err
}
