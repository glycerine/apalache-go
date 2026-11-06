package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:355-465.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// rewrite follows the source's iterative rewrite stack. The Boolean reducer
// is the same configured core owned by the enclosing UFBV rewriter. The result
// has an independent AST reference; the caller releases it through the manager.
func (r *rewriteTraversal) rewrite(index *rewriteRuleIndex, boolean *rewriter.BoolRewriter, root ast.ASTHandle) (_ ast.ASTRef, result error) {
	if e := r.check(); e != nil {
		return ast.ASTRef{}, e
	}
	if e := index.check(); e != nil {
		return ast.ASTRef{}, e
	}
	m := r.manager
	if len(index.byFunc) == 0 {
		return m.NewRef(root)
	}
	r.trace = index.traceTags()
	if e := r.traceRewrite(index, 359, "rewrite: ", root); e != nil {
		return ast.ASTRef{}, e
	}
	if len(r.todo) != 0 {
		if enabled, e := forwardAssertionEnabled(); e != nil {
			return ast.ASTRef{}, e
		} else if enabled {
			if e := forwardAssertionFailure(362, "m_rewrite_todo.empty()"); e != nil {
				return ast.ASTRef{}, e
			}
		}
		return ast.ASTRef{}, ast.ErrASTPending
	}
	if e := r.cache.reset(); e != nil {
		return ast.ASTRef{}, e
	}
	// App temporaries live for this call. Source m_new_exprs keeps quantifier
	// intermediates until the traversal owner closes.
	var scratch []ast.ASTRef
	defer func() {
		for i := len(scratch) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&scratch[i]))
		}
	}()
	keep := func(h ast.ASTHandle) error {
		ref, e := m.NewRef(h)
		if e == nil {
			scratch = append(scratch, ref)
		}
		return e
	}
	if e := r.pushTodo(root); e != nil {
		return ast.ASTRef{}, e
	}
	for len(r.todo) != 0 {
		if e := r.traceStack(); e != nil {
			return ast.ASTRef{}, e
		}
		e := r.todo[len(r.todo)-1]
		actual := e
		if prior, found, err := r.cache.get(e); err != nil {
			return ast.ASTRef{}, err
		} else if found {
			if prior.done {
				if err := r.popTodo(); err != nil {
					return ast.ASTRef{}, err
				}
				continue
			}
			actual = prior.expr
		}
		kind, err := m.GetKind(actual)
		if err != nil {
			return ast.ASTRef{}, err
		}
		switch kind {
		case ast.ASTVar:
			if err := r.cache.insert(e, actual, true); err != nil {
				return ast.ASTRef{}, err
			}
			if err := r.popTodo(); err != nil {
				return ast.ASTRef{}, err
			}
		case ast.ASTApp:
			ready, err := r.visitChildren(actual)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if !ready {
				continue
			}
			f, err := m.GetDecl(actual)
			if err != nil {
				return ast.ASTRef{}, err
			}
			count, err := m.GetNumArgs(actual)
			if err != nil {
				return ast.ASTRef{}, err
			}
			args := make([]ast.ASTHandle, count)
			untouched := true
			for i := range args {
				old, err := m.GetArg(actual, uint32(i))
				if err != nil {
					return ast.ASTRef{}, err
				}
				child, err := r.completedChild(old)
				if err != nil {
					return ast.ASTRef{}, err
				}
				args[i] = child
				untouched = untouched && args[i] == old
			}
			next, matched, err := index.tryRewrite(f, args)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if matched {
				scratch = append(scratch, next)
				h, err := m.RefGet(next)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if err := r.cache.insert(e, h, false); err != nil {
					return ast.ASTRef{}, err
				}
				continue
			}
			if untouched {
				if err := r.cache.insert(e, actual, true); err != nil {
					return ast.ASTRef{}, err
				}
			} else {
				family, err := m.GetFamilyID(f)
				if err != nil {
					return ast.ASTRef{}, err
				}
				var rebuilt ast.ASTHandle
				if family == m.BasicFamilyID() {
					if boolean == nil {
						return ast.ASTRef{}, ast.ErrASTDomain
					}
					out, err := m.NewRef(ast.ASTHandle{})
					if err != nil {
						return ast.ASTRef{}, err
					}
					scratch = append(scratch, out)
					status, err := boolean.MkAppCore(f, ast.SliceArguments(args), out)
					if err != nil {
						return ast.ASTRef{}, err
					}
					if status == rewriter.RewriteFailed {
						rebuilt, err = m.MkApp(f, args)
						if err != nil {
							return ast.ASTRef{}, err
						}
						if err := m.SetRef(out, rebuilt); err != nil {
							return ast.ASTRef{}, err
						}
					} else {
						rebuilt, err = m.RefGet(out)
						if err != nil {
							return ast.ASTRef{}, err
						}
					}
				} else {
					rebuilt, err = m.MkApp(f, args)
					if err != nil {
						return ast.ASTRef{}, err
					}
					if err := keep(rebuilt); err != nil {
						return ast.ASTRef{}, err
					}
				}
				if err := r.traceRebuilt(index, e, args, rebuilt); err != nil {
					return ast.ASTRef{}, err
				}
				if err := r.cache.insert(e, rebuilt, true); err != nil {
					return ast.ASTRef{}, err
				}
			}
			if err := r.popTodo(); err != nil {
				return ast.ASTRef{}, err
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(actual)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if child, found, err := r.cache.get(body); err != nil {
				return ast.ASTRef{}, err
			} else if found {
				if !child.done {
					if enabled, e := forwardAssertionEnabled(); e != nil {
						return ast.ASTRef{}, e
					} else if enabled {
						if e := forwardAssertionFailure(437, "ebp.second"); e != nil {
							return ast.ASTRef{}, e
						}
					}
					return ast.ASTRef{}, ast.ErrASTPending
				}
				q, err := m.UpdateQuantifierBody(actual, child.expr)
				if err != nil {
					return ast.ASTRef{}, err
				}
				qRef, err := m.NewRef(q)
				if err != nil {
					return ast.ASTRef{}, err
				}
				r.newExprs = append(r.newExprs, qRef)
				newQ, err := rewriter.ElimUnusedVarsUFBV(m, q)
				if err != nil {
					return ast.ASTRef{}, err
				}
				r.newExprs = append(r.newExprs, newQ)
				h, err := m.RefGet(newQ)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if err := r.cache.insert(e, h, true); err != nil {
					return ast.ASTRef{}, err
				}
				if err := r.popTodo(); err != nil {
					return ast.ASTRef{}, err
				}
			} else if err := r.pushTodo(body); err != nil {
				return ast.ASTRef{}, err
			}
		default:
			return ast.ASTRef{}, ast.ErrASTDomain
		}
	}
	final, found, err := r.cache.get(root)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !found {
		if enabled, e := forwardAssertionEnabled(); e != nil {
			return ast.ASTRef{}, e
		} else if enabled {
			if e := forwardAssertionFailure(457, "m_rewrite_cache.contains(n)"); e != nil {
				return ast.ASTRef{}, e
			}
		}
		return ast.ASTRef{}, ast.ErrASTPending
	}
	if !final.done {
		if enabled, e := forwardAssertionEnabled(); e != nil {
			return ast.ASTRef{}, e
		} else if enabled {
			if e := forwardAssertionFailure(459, "ebp.second"); e != nil {
				return ast.ASTRef{}, e
			}
		}
		return ast.ASTRef{}, ast.ErrASTPending
	}
	if e := r.traceRewrite(index, 462, "rewrite result: ", final.expr); e != nil {
		return ast.ASTRef{}, e
	}
	return m.NewRef(final.expr)
}

// completedChild is the source's cache lookup immediately after the child
// traversal reports ready. A stopped assertion leaves the cache untouched.
func (r *rewriteTraversal) completedChild(old ast.ASTHandle) (ast.ASTHandle, error) {
	child, found, err := r.cache.get(old)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found || !child.done {
		if enabled, e := forwardAssertionEnabled(); e != nil {
			return ast.ASTHandle{}, e
		} else if enabled {
			if e := forwardAssertionFailure(402, "m_rewrite_cache.contains(o_child) && m_rewrite_cache.get(o_child).second"); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return ast.ASTHandle{}, ast.ErrASTPending
	}
	return child.expr, nil
}
