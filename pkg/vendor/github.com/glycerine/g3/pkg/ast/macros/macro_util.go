package macros

// Source: src/ast/macros/macro_util.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type Candidate struct {
	Declaration, Definition, Condition ast.ASTHandle
	Inequality, SatisfyAtom, Hint      bool
}

// Candidates owns one AST reference for every non-null handle in each row.
type Candidates struct {
	self    *Candidates
	manager *ast.ASTManager
	rows    []Candidate
	header  uint64
	closed  bool
}

func NewCandidates(m *ast.ASTManager) (*Candidates, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(Candidates{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &Candidates{manager: m, header: n}
	c.self = c
	return c, nil
}

func (c *Candidates) open() error {
	if c == nil || c.self != c || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *Candidates) Len() int {
	if c == nil || c.closed || c.self != c {
		return 0
	}
	return len(c.rows)
}
func (c *Candidates) At(i int) (Candidate, error) {
	if err := c.open(); err != nil {
		return Candidate{}, err
	}
	if i < 0 || i >= len(c.rows) {
		return Candidate{}, ast.ErrASTDomain
	}
	return c.rows[i], nil
}
func (c *Candidates) Empty() bool { return c.Len() == 0 }

func (c *Candidates) Insert(row Candidate) error {
	if err := c.open(); err != nil {
		return err
	}
	values := [...]ast.ASTHandle{row.Declaration, row.Definition, row.Condition}
	owned := 0
	for _, h := range values {
		if h.IsNull() {
			owned++
			continue
		}
		if err := c.manager.IncRef(h); err != nil {
			for j := 0; j < owned; j++ {
				if !values[j].IsNull() {
					_ = c.manager.DecRef(values[j])
				}
			}
			return err
		}
		owned++
	}
	c.rows = append(c.rows, row)
	return nil
}

func (c *Candidates) Reset() (result error) {
	if err := c.open(); err != nil {
		return err
	}
	for i := len(c.rows) - 1; i >= 0; i-- {
		row := &c.rows[i]
		for _, h := range [...]ast.ASTHandle{row.Condition, row.Definition, row.Declaration} {
			if !h.IsNull() {
				result = errors.Join(result, c.manager.DecRef(h))
			}
		}
		*row = Candidate{}
	}
	c.rows = c.rows[:0]
	return result
}

func (c *Candidates) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.Reset(); err != nil {
		return err
	}
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.rows = nil
	c.closed = true
	return nil
}

type Util struct {
	manager   *ast.ASTManager
	forbidden map[ast.ASTHandle]struct{}
	clause    ast.ASTHandle
}

func NewUtil(m *ast.ASTManager) (*Util, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofMode(); err != nil {
		return nil, err
	}
	return &Util{manager: m}, nil
}

// SetForbidden borrows declaration identities; the caller keeps them live.
func (u *Util) SetForbidden(declarations []ast.ASTHandle) {
	if len(declarations) == 0 {
		u.forbidden = nil
		return
	}
	u.forbidden = make(map[ast.ASTHandle]struct{}, len(declarations))
	for _, d := range declarations {
		u.forbidden[d] = struct{}{}
	}
}

func (u *Util) forbiddenDecl(d ast.ASTHandle) bool { _, yes := u.forbidden[d]; return yes }

func (u *Util) IsMacroHead(term ast.ASTHandle, declarations uint32) (bool, error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	d, err := u.manager.GetDecl(term)
	if err != nil {
		return false, err
	}
	assoc, err := u.manager.RewriterAssociative(d)
	if err != nil {
		return false, err
	}
	fid, err := u.manager.GetFamilyID(d)
	if err != nil {
		return false, err
	}
	n, err := u.manager.GetNumArgs(term)
	if err != nil {
		return false, err
	}
	if assoc || fid != ast.NullFamilyID || n != declarations {
		return false, nil
	}
	seen := make([]bool, declarations)
	for i := uint32(0); i < declarations; i++ {
		a, e := u.manager.GetArg(term, i)
		if e != nil {
			return false, e
		}
		k, e := u.manager.GetKind(a)
		if e != nil {
			return false, e
		}
		if k != ast.ASTVar {
			return false, nil
		}
		index, e := u.manager.GetVarIndex(a)
		if e != nil {
			return false, e
		}
		if index >= declarations || seen[index] {
			return false, nil
		}
		seen[index] = true
	}
	return true, nil
}

func (u *Util) simpleMacro(term ast.ASTHandle, declarations uint32, right bool) (head, definition ast.ASTHandle, matched bool, result error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return head, definition, false, err
	}
	fid, err := u.manager.GetFamilyID(term)
	if err != nil {
		return head, definition, false, err
	}
	k, err := u.manager.GetDeclKind(term)
	if err != nil {
		return head, definition, false, err
	}
	if fid != u.manager.BasicFamilyID() || k != ast.ASTOpEq && k != ast.ASTOpIff {
		return head, definition, false, nil
	}
	a, err := u.manager.GetArg(term, 0)
	if err != nil {
		return head, definition, false, err
	}
	b, err := u.manager.GetArg(term, 1)
	if err != nil {
		return head, definition, false, err
	}
	if right {
		a, b = b, a
	}
	yes, err := u.IsMacroHead(a, declarations)
	if err != nil || !yes {
		return head, definition, false, err
	}
	d, err := u.manager.GetDecl(a)
	if err != nil {
		return head, definition, false, err
	}
	if u.forbiddenDecl(d) {
		return head, definition, false, nil
	}
	occurs, err := u.manager.OccursDecl(d, b)
	if err != nil || occurs {
		return head, definition, false, err
	}
	return a, b, true, nil
}

func (u *Util) IsLeftSimpleMacro(term ast.ASTHandle, declarations uint32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return u.simpleMacro(term, declarations, false)
}
func (u *Util) IsRightSimpleMacro(term ast.ASTHandle, declarations uint32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return u.simpleMacro(term, declarations, true)
}
func (u *Util) IsSimpleMacro(term ast.ASTHandle, declarations uint32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	h, d, yes, err := u.IsLeftSimpleMacro(term, declarations)
	if err != nil || yes {
		return h, d, yes, err
	}
	return u.IsRightSimpleMacro(term, declarations)
}

func (u *Util) IsQuasiMacroHead(term ast.ASTHandle, declarations uint32) (bool, error) {
	kind, err := u.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	d, err := u.manager.GetDecl(term)
	if err != nil {
		return false, err
	}
	fid, err := u.manager.GetFamilyID(d)
	if err != nil {
		return false, err
	}
	n, err := u.manager.GetNumArgs(term)
	if err != nil {
		return false, err
	}
	if fid != ast.NullFamilyID || n < declarations {
		return false, nil
	}
	found := make([]bool, declarations)
	count := uint32(0)
	for i := uint32(0); i < n; i++ {
		a, e := u.manager.GetArg(term, i)
		if e != nil {
			return false, e
		}
		k, e := u.manager.GetKind(a)
		if e != nil {
			return false, e
		}
		if k == ast.ASTVar {
			index, e := u.manager.GetVarIndex(a)
			if e != nil {
				return false, e
			}
			if index >= declarations {
				return false, nil
			}
			if !found[index] {
				found[index] = true
				count++
			}
		} else {
			yes, e := u.manager.OccursDecl(d, a)
			if e != nil || yes {
				return false, e
			}
		}
	}
	return count == declarations, nil
}

// NormalizeExpr writes the source variable-renaming normalization into out.
func (u *Util) NormalizeExpr(head ast.ASTHandle, declarations uint32, term ast.ASTHandle, out ast.ASTRef) (result error) {
	n, err := u.manager.GetNumArgs(head)
	if err != nil {
		return err
	}
	mapping := make([]ast.ASTHandle, declarations)
	changed := false
	for i := uint32(0); i < n; i++ {
		v, e := u.manager.GetArg(head, i)
		if e != nil {
			return e
		}
		index, e := u.manager.GetVarIndex(v)
		if e != nil || index >= declarations {
			return ast.ErrASTDomain
		}
		if index != i {
			changed = true
			sort, e := u.manager.GetSort(v)
			if e != nil {
				return e
			}
			v, e = u.manager.MkVar(i, sort)
			if e != nil {
				return e
			}
		}
		mapping[declarations-index-1] = v
	}
	if !changed {
		return u.manager.SetRef(out, term)
	}
	s, err := rewriter.NewVarSubstitution(u.manager, true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.Close()) }()
	return s.Substitute(term, mapping, out)
}
