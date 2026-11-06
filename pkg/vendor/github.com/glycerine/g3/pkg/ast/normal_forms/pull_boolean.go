package normal_forms

// Source: src/ast/normal_forms/pull_quant.cpp:34-211, no-proof branch.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pullQuantCore struct {
	manager   *ast.ASTManager
	traceTags *diagnostic.TraceTags
	shift     *rewriter.VarShifter
}

func PullQuantifierNoProof(manager *ast.ASTManager, root ast.ASTHandle, tags *diagnostic.TraceTags) (out ast.ASTRef, err error) {
	if manager == nil {
		return out, ast.ErrASTHandle
	}
	shift, err := rewriter.NewVarShifter(manager)
	if err != nil {
		return out, err
	}
	defer func() {
		if cleanup := shift.Close(); cleanup != nil {
			err = errors.Join(err, cleanup)
			if out != (ast.ASTRef{}) {
				err = errors.Join(err, releasePullRefs(manager, []ast.ASTRef{out}))
				out = ast.ASTRef{}
			}
		}
	}()
	return (&pullQuantCore{manager: manager, traceTags: tags, shift: shift}).pullExpr(root)
}

func (p *pullQuantCore) pullExpr(h ast.ASTHandle) (out ast.ASTRef, err error) {
	defer func() {
		if err != nil && out != (ast.ASTRef{}) {
			err = errors.Join(err, p.releaseRefs([]ast.ASTRef{out}))
			out = ast.ASTRef{}
		}
	}()
	m := p.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return out, err
	}
	switch kind {
	case ast.ASTVar:
		return m.NewRef(h)
	case ast.ASTQuantifier:
		body, e := m.GetQuantifierBody(h)
		if e != nil {
			return out, e
		}
		bodyRef, e := p.pullExpr(body)
		if e != nil {
			return out, e
		}
		defer func() {
			if cleanup := p.releaseRefs([]ast.ASTRef{bodyRef}); cleanup != nil {
				err = errors.Join(err, cleanup)
			}
		}()
		newBody, e := m.RefGet(bodyRef)
		if e != nil {
			return out, e
		}
		result := h
		bodyKind, e := m.GetKind(newBody)
		if e != nil {
			return out, e
		}
		if bodyKind == ast.ASTQuantifier {
			outerForall, e := m.IsForall(h)
			if e != nil {
				return out, e
			}
			innerForall, e := m.IsForall(newBody)
			if e != nil {
				return out, e
			}
			if !outerForall || !innerForall {
				return out, ast.ErrASTPending
			}
			result, e = p.pullDirectForall(h, newBody)
			if e != nil {
				return out, e
			}
		} else if newBody != body {
			result, e = m.UpdateQuantifierBody(h, newBody)
			if e != nil {
				return out, e
			}
		}
		return m.NewRef(result)
	case ast.ASTApp:
		n, e := m.GetNumArgs(h)
		if e != nil {
			return out, e
		}
		args := make([]ast.ASTHandle, n)
		refs := make([]ast.ASTRef, 0, n)
		defer func() {
			if cleanup := p.releaseRefs(refs); cleanup != nil {
				err = errors.Join(err, cleanup)
			}
		}()
		changed := false
		for i := uint32(0); i < n; i++ {
			child, e := m.GetArg(h, i)
			if e != nil {
				return out, e
			}
			ref, e := p.pullExpr(child)
			if e != nil {
				return out, e
			}
			refs = append(refs, ref)
			args[i], e = m.RefGet(ref)
			if e != nil {
				return out, e
			}
			changed = changed || args[i] != child
		}
		decl, e := m.GetDecl(h)
		if e != nil {
			return out, e
		}
		family, e := m.GetFamilyID(decl)
		if e != nil {
			return out, e
		}
		if family == m.BasicFamilyID() {
			operator, e := m.GetDeclKind(decl)
			if e != nil {
				return out, e
			}
			if operator == ast.ASTOpNot || operator == ast.ASTOpAnd || operator == ast.ASTOpOr {
				pulled, didPull, e := p.pullBooleanApp(decl, operator, args)
				if e != nil {
					return out, e
				}
				if didPull {
					return m.NewRef(pulled)
				}
			}
		}
		if !changed {
			return m.NewRef(h)
		}
		rebuilt, e := m.MkApp(decl, args)
		if e != nil {
			return out, e
		}
		return m.NewRef(rebuilt)
	default:
		return out, ast.ErrASTDomain
	}
}

func (p *pullQuantCore) pullBooleanApp(decl ast.ASTHandle, operator int32, args []ast.ASTHandle) (out ast.ASTHandle, changed bool, err error) {
	m := p.manager
	if operator == ast.ASTOpNot {
		if len(args) != 1 {
			return out, false, ast.ErrASTDomain
		}
		kind, e := m.GetKind(args[0])
		if e != nil || kind != ast.ASTQuantifier {
			return out, false, e
		}
		body, e := m.GetQuantifierBody(args[0])
		if e != nil {
			return out, false, e
		}
		negated, e := m.MkApp(decl, []ast.ASTHandle{body})
		if e != nil {
			return out, false, e
		}
		forall, e := m.IsForall(args[0])
		if e != nil {
			return out, false, e
		}
		out, err = m.UpdateQuantifierKind(args[0], !forall, negated)
		return out, err == nil, err
	}
	if operator != ast.ASTOpAnd && operator != ast.ASTOpOr {
		return out, false, nil
	}
	var sorts []ast.ASTHandle
	var names []symbol.Symbol
	var qid symbol.Symbol
	weight := int32(1<<31 - 1)
	var forall bool
	for _, child := range args {
		kind, e := m.GetKind(child)
		if e != nil {
			return out, false, e
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		childForall, e := m.IsForall(child)
		if e != nil {
			return out, false, e
		}
		if len(sorts) == 0 {
			forall = childForall
			qid, _, e = m.GetQuantifierIDs(child)
			if e != nil {
				return out, false, e
			}
		} else if childForall != forall {
			return out, false, ast.ErrASTPending
		}
		w, e := m.GetQuantifierWeight(child)
		if e != nil {
			return out, false, e
		}
		if w < weight {
			weight = w
		}
		n, e := m.GetNumDeclarations(child)
		if e != nil {
			return out, false, e
		}
		for j := n; j > 0; {
			j--
			sort, e := m.GetDeclarationSort(child, j)
			if e != nil {
				return out, false, e
			}
			name, e := m.GetDeclarationName(child, j)
			if e != nil {
				return out, false, e
			}
			duplicate := false
			for _, prior := range names {
				if prior == name {
					duplicate = true
					break
				}
			}
			if duplicate {
				var prefix *string
				if !name.IsNumerical() {
					text, present, e := m.Symbols.BareString(name)
					if e != nil {
						return out, false, e
					}
					if present {
						prefix = &text
					}
				}
				name, e = m.MkFreshVarName(prefix)
				if e != nil {
					return out, false, e
				}
			}
			sorts = append(sorts, sort)
			names = append(names, name)
		}
	}
	if len(sorts) == 0 {
		return out, false, nil
	}
	count := uint32(len(sorts))
	trace := p.traceTags
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	if trace.ShouldTrace("pull_quant") {
		if e := trace.Emit("pull_quant", "pull_quant1_core", "src/ast/normal_forms/pull_quant.cpp", 102,
			"Result num decls:"+strconv.FormatUint(uint64(count), 10)+"\n"); e != nil {
			return out, false, e
		}
	}
	var offset uint32
	adjusted := make([]ast.ASTHandle, len(args))
	refs := make([]ast.ASTRef, 0, len(args))
	defer func() {
		if cleanup := p.releaseRefs(refs); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	for i, child := range args {
		kind, e := m.GetKind(child)
		if e != nil {
			return out, false, e
		}
		bound, shift1, shift2 := uint32(0), count, uint32(0)
		shifted := child
		if kind == ast.ASTQuantifier {
			bound, e = m.GetNumDeclarations(child)
			if e != nil {
				return out, false, e
			}
			shift1, shift2 = count-bound, offset
			shifted, e = m.GetQuantifierBody(child)
			if e != nil {
				return out, false, e
			}
			offset += bound
		}
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return out, false, e
		}
		refs = append(refs, ref)
		if e = p.shift.Shift(shifted, bound, shift1, shift2, ref); e != nil {
			return out, false, e
		}
		adjusted[i], e = m.RefGet(ref)
		if e != nil {
			return out, false, e
		}
		if e = p.tracePulledChild(kind == ast.ASTQuantifier, child, shifted, adjusted[i], count, bound, shift1, shift2); e != nil {
			return out, false, e
		}
	}
	for i, j := 0, len(sorts)-1; i < j; i, j = i+1, j-1 {
		sorts[i], sorts[j] = sorts[j], sorts[i]
		names[i], names[j] = names[j], names[i]
	}
	body, e := m.MkApp(decl, adjusted)
	if e != nil {
		return out, false, e
	}
	out, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names,
		Body: body, Weight: weight, QID: qid})
	return out, err == nil, err
}

func releasePullRefs(m *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := range refs {
		result = errors.Join(result, m.DelRef(&refs[i]))
	}
	return result
}
func (p *pullQuantCore) releaseRefs(refs []ast.ASTRef) error { return releasePullRefs(p.manager, refs) }
