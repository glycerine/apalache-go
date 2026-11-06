package spacer

// Source: src/muz/spacer/spacer_util.cpp normalize.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type normalizeCleanupFailure struct {
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	local       uparams.ParamsRef
	localLive   bool
	rewriter    *astrewriter.TheoryRewriter
	equivalence *ast.ExprEquivClass
	values      []ast.ASTRef
	output      ast.ASTRef
	cause       error
	cleanupErr  error
}

func (f *normalizeCleanupFailure) Error() string { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *normalizeCleanupFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *normalizeCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *normalizeCleanupFailure) OriginalCause() error { return f.cause }
func (f *normalizeCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (f *normalizeCleanupFailure) close(includeOutput bool) (result error) {
	if f.equivalence != nil {
		if err := f.equivalence.Close(); err != nil {
			return err
		}
		f.equivalence = nil
	}
	if err := releaseNormalizeRefs(f.manager, f.values); err != nil {
		return err
	}
	f.values = nil
	if includeOutput && f.output != (ast.ASTRef{}) {
		if err := f.manager.DelRef(&f.output); err != nil {
			return err
		}
	}
	if f.rewriter != nil {
		if err := f.rewriter.Close(); err != nil {
			return err
		}
		f.rewriter = nil
	}
	if f.localLive {
		if err := f.context.Del(&f.local); err != nil {
			return err
		}
		f.localLive = false
	}
	return result
}

func (f *normalizeCleanupFailure) Cleanup() error {
	f.cleanupErr = f.close(true)
	return f.cleanupErr
}

// Source ast_lt_proc compares get_id(), unlike the structural ast_lt order.
// Read IDs once while the concrete ASTRef owners keep every node live.
func normalizeLess(manager *ast.ASTManager, values []ast.ASTRef) error {
	type entry struct {
		ref ast.ASTRef
		id  uint32
	}
	entries := make([]entry, len(values))
	for i, ref := range values {
		handle, err := manager.RefGet(ref)
		if err != nil {
			return err
		}
		id, err := manager.GetASTID(handle)
		if err != nil {
			return err
		}
		entries[i] = entry{ref: ref, id: id}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	for i, item := range entries {
		values[i] = item.ref
	}
	return nil
}

func normalizeIsAnd(manager *ast.ASTManager, value ast.ASTHandle) (bool, error) {
	kind, err := manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := manager.GetFamilyID(value)
	if err != nil || family != manager.BasicFamilyID() {
		return false, err
	}
	op, err := manager.GetDeclKind(value)
	return op == ast.ASTOpAnd, err
}

func releaseNormalizeRefs(manager *ast.ASTManager, values []ast.ASTRef) error {
	var result error
	for i := range values {
		if values[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&values[i]))
		}
	}
	return result
}

func normalizeAdhocReplacement(manager *ast.ASTManager, family ast.FamilyID, value ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	if yes, child, notErr := manager.IsNot(value); notErr != nil {
		return ast.ASTHandle{}, false, notErr
	} else if yes {
		if nested, grandchild, nestedErr := manager.IsNot(child); nestedErr != nil {
			return ast.ASTHandle{}, false, nestedErr
		} else if nested {
			return grandchild, true, nil
		}
	}
	fid, err := manager.GetFamilyID(value)
	if err != nil || fid != family {
		return ast.ASTHandle{}, false, err
	}
	op, err := manager.GetDeclKind(value)
	if err != nil || (op != 2 && op != 3) {
		return ast.ASTHandle{}, false, err
	}
	left, err := manager.GetArg(value, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	right, err := manager.GetArg(value, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sort, err := manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sortFamily, err := manager.GetFamilyID(sort)
	if err != nil || sortFamily != family {
		return ast.ASTHandle{}, false, err
	}
	sortKind, err := manager.GetDeclKind(sort)
	if err != nil || sortKind != 1 {
		return ast.ASTHandle{}, false, err
	}
	text, _, numeral, err := manager.ArithmeticNumeralText(right)
	if err != nil || !numeral || (op == 2 && text != "-1") || (op == 3 && text != "1") {
		return ast.ASTHandle{}, false, err
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	comparison, err := manager.MkFamilyApp(family, 5-op, nil, []ast.ASTHandle{left, zero})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := manager.MkNot(comparison)
	return result, err == nil, err
}

// normalizeAdhoc is the concrete bottom-up adhoc_rewriter_cfg traversal.
// Rebuild parents from rewritten children before applying the local rule.
func normalizeAdhoc(manager *ast.ASTManager, root ast.ASTRef) (resultErr error) {
	handle, err := manager.RefGet(root)
	if err != nil {
		return err
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	type frame struct {
		node     ast.ASTHandle
		children []ast.ASTHandle
		patterns int
		ready    bool
	}
	todo := []frame{{node: handle}}
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	owned := make([]ast.ASTRef, 0)
	defer func() {
		cleanup := &normalizeCleanupFailure{manager: manager, values: owned, cause: resultErr}
		if err := cleanup.close(true); err != nil {
			cleanup.cause = errors.Join(resultErr, err)
			cleanup.cleanupErr = err
			resultErr = cleanup
		}
	}()
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, done := cache[current.node]; done {
			continue
		}
		kind, err := manager.GetKind(current.node)
		if err != nil {
			return err
		}
		if !current.ready {
			switch kind {
			case ast.ASTApp:
				count, err := manager.GetNumArgs(current.node)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetArg(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
			case ast.ASTQuantifier:
				body, err := manager.GetQuantifierBody(current.node)
				if err != nil {
					return err
				}
				current.children = append(current.children, body)
				count, err := manager.GetNumPatterns(current.node)
				if err != nil {
					return err
				}
				current.patterns = int(count)
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetPattern(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
				count, err = manager.GetNumNoPatterns(current.node)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetNoPattern(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
			}
			current.ready = true
			todo = append(todo, current)
			for i := len(current.children); i > 0; i-- {
				todo = append(todo, frame{node: current.children[i-1]})
			}
			continue
		}
		changed := false
		children := make([]ast.ASTHandle, len(current.children))
		for i, child := range current.children {
			children[i] = cache[child]
			changed = changed || children[i] != child
		}
		rebuilt := current.node
		if changed {
			switch kind {
			case ast.ASTApp:
				decl, err := manager.GetDecl(current.node)
				if err != nil {
					return err
				}
				rebuilt, err = manager.MkApp(decl, children)
				if err != nil {
					return err
				}
			case ast.ASTQuantifier:
				rebuilt, err = manager.UpdateQuantifierAnnotations(current.node, children[1:1+current.patterns], children[1+current.patterns:], children[0])
				if err != nil {
					return err
				}
			}
		}
		ref, err := manager.NewRef(rebuilt)
		if err != nil {
			return err
		}
		owned = append(owned, ref)
		replacement, replace, err := normalizeAdhocReplacement(manager, family, rebuilt)
		if err != nil {
			return err
		}
		if replace {
			if err = manager.SetRef(ref, replacement); err != nil {
				return err
			}
			rebuilt = replacement
		}
		cache[current.node] = rebuilt
	}
	return manager.SetRef(root, cache[handle])
}

// normalizeFormula returns an owning reference. It follows the retained
// Spacer normalization pipeline; factorEqualities is only enabled by callers
// that use the corresponding source overload.
func normalizeFormula(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, formula ast.ASTHandle, useSimplifyBounds, factorEqualities bool) (result ast.ASTRef, resultErr error) {
	if manager == nil || context == nil || globals == nil {
		return ast.ASTRef{}, ErrManagerOwner
	}
	local, err := context.NewValue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup := &normalizeCleanupFailure{manager: manager, context: context, local: local, localLive: true}
	defer func() {
		if resultErr == nil && result != (ast.ASTRef{}) {
			cleanup.output = result
			if err := cleanup.close(false); err != nil {
				cleanup.cause, cleanup.cleanupErr = err, err
				result, resultErr = ast.ASTRef{}, cleanup
				return
			}
			cleanup.output = ast.ASTRef{}
			return
		}
		result = ast.ASTRef{}
		if err := cleanup.close(true); err != nil {
			cleanup.cause, cleanup.cleanupErr = errors.Join(resultErr, err), err
			resultErr = cleanup
		}
	}()
	for _, name := range []string{"sort_sums", "gcd_rounding", "arith_lhs", "som", "flat"} {
		key := name
		if err = context.SetBoolName(&local, &key, true); err != nil {
			return ast.ASTRef{}, err
		}
	}
	rw, err := astrewriter.NewTheoryRewriter(manager, context, globals, &local)
	cleanup.rewriter = rw
	if err != nil {
		return ast.ASTRef{}, err
	}
	out, err := manager.NewRef(formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup.output = out
	if err = rw.Rewrite(formula, out); err != nil {
		return ast.ASTRef{}, err
	}
	if err = normalizeAdhoc(manager, out); err != nil {
		return ast.ASTRef{}, err
	}
	rewritten, err := manager.RefGet(out)
	if err != nil {
		return ast.ASTRef{}, err
	}
	conjunction, err := normalizeIsAnd(manager, rewritten)
	if err != nil || !conjunction {
		return out, err
	}
	handles, err := manager.FlattenAnd([]ast.ASTHandle{rewritten})
	if err != nil || len(handles) <= 1 {
		return out, err
	}
	values, err := refsFromHandles(manager, handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup.values = values
	if err = normalizeLess(manager, values); err != nil {
		return ast.ASTRef{}, err
	}
	if useSimplifyBounds {
		values, err = simplifyBounds(manager, context, globals, values)
		cleanup.values = values
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	if factorEqualities {
		handles = handles[:0]
		for i := range values {
			handle, getErr := manager.RefGet(values[i])
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			handles = append(handles, handle)
		}
		equivalence, newErr := ast.NewExprEquivClass(manager)
		if newErr != nil {
			return ast.ASTRef{}, newErr
		}
		cleanup.equivalence = equivalence
		handles, err = manager.FactorEqualities(handles, equivalence)
		if err != nil {
			return ast.ASTRef{}, err
		}
		factored, factorErr := manager.EquivToExpr(equivalence)
		if factorErr != nil {
			return ast.ASTRef{}, factorErr
		}
		remaining, refErr := refsFromHandles(manager, handles)
		if refErr != nil {
			cleanup.values = append(append([]ast.ASTRef(nil), values...), factored...)
			return ast.ASTRef{}, refErr
		}
		oldCount := len(values)
		allOwned := make([]ast.ASTRef, 0, oldCount+len(remaining)+len(factored))
		allOwned = append(allOwned, values...)
		allOwned = append(allOwned, remaining...)
		allOwned = append(allOwned, factored...)
		cleanup.values = allOwned
		if err = releaseNormalizeRefs(manager, allOwned[:oldCount]); err != nil {
			return ast.ASTRef{}, err
		}
		values = allOwned[oldCount:]
		cleanup.values = values
	}
	handles = handles[:0]
	for i := range values {
		handle, getErr := manager.RefGet(values[i])
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		handles = append(handles, handle)
	}
	normalized, err := manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = manager.SetRef(out, normalized); err != nil {
		return ast.ASTRef{}, err
	}
	return out, nil
}
