package duality

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

type normalizedClauseHead struct {
	Parameters []*Expr
	Body       *Expr
}

func (h *normalizedClauseHead) close() error {
	if h == nil {
		return nil
	}
	var result error
	if h.Body != nil {
		if err := h.Body.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			h.Body = nil
		}
	}
	for i, e := range h.Parameters {
		if e == nil {
			continue
		}
		if err := e.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			h.Parameters[i] = nil
		}
	}
	return result
}

// normalizeClauseHead implements both live FromClauses head passes.
// A nil body requests node parameters only. With a body, each replacement also
// appends an equality constraint, in source left-associated order. Parameters
// remain unskolemized until the caller performs delayed SubstBound.
func normalizeClauseHead(head, body *Expr) (out *normalizedClauseHead, result error) {
	if err := head.open(); err != nil {
		return nil, err
	}
	c := head.value.object.context
	m := c.manager
	if body != nil {
		if err := body.open(); err != nil {
			return nil, err
		}
		if body.value.object.context != c {
			return nil, ErrContextMismatch
		}
	}
	out = &normalizedClauseHead{}
	defer func() {
		if result != nil {
			result = errors.Join(result, out.close())
			out = nil
		}
	}()
	if body != nil {
		var err error
		out.Body, err = body.Clone()
		if err != nil {
			return out, err
		}
	}
	root, err := head.Raw()
	if err != nil {
		return out, err
	}
	n, err := m.GetNumArgs(root)
	if err != nil {
		return out, err
	}
	seen := make(map[ast.ASTHandle]bool)
	for i := uint32(0); i < n; i++ {
		original, err := m.GetArg(root, i)
		if err != nil {
			return out, err
		}
		arg := original
		kind, err := m.GetKind(arg)
		if err != nil {
			return out, err
		}
		variable := kind == ast.ASTVar
		if kind == ast.ASTApp {
			family, err := m.GetFamilyID(arg)
			if err != nil {
				return out, err
			}
			count, err := m.GetNumArgs(arg)
			if err != nil {
				return out, err
			}
			variable = family == ast.NullFamilyID && count == 0
		}
		replaced := !variable || seen[arg]
		if replaced {
			sort, err := m.GetSort(arg)
			if err != nil {
				return out, err
			}
			name, err := m.Symbols.Intern("@a_" + strconv.FormatUint(uint64(i), 10))
			if err != nil {
				return out, err
			}
			arg, err = m.MkConst(name, sort)
			if err != nil {
				return out, err
			}
		}
		parameter, err := NewExpr(c, arg)
		if err != nil {
			return out, err
		}
		out.Parameters = append(out.Parameters, parameter)
		seen[arg] = true
		if replaced && out.Body != nil {
			equality, err := m.MkEq(original, arg)
			if err != nil {
				return out, err
			}
			eq, err := NewExpr(c, equality)
			if err != nil {
				return out, err
			}
			previous, err := out.Body.Raw()
			if err != nil {
				return out, errors.Join(err, eq.Close())
			}
			conjunction, err := m.MkAnd(previous, equality)
			if err != nil {
				return out, errors.Join(err, eq.Close())
			}
			next, createErr := NewExpr(c, conjunction)
			closeErr := eq.Close()
			if createErr != nil {
				return out, errors.Join(createErr, closeErr)
			}
			old := out.Body
			out.Body = next
			if err := errors.Join(closeErr, old.Close()); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
