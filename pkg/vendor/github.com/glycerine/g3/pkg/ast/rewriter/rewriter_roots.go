package rewriter

// Native AST reference ownership for retained rewriter result/pattern stacks.

import "github.com/glycerine/g3/pkg/ast"

func appendRewriterRoot(m *ast.ASTManager, roots *[]ast.ASTHandle, h ast.ASTHandle) error {
	if e := m.IncRef(h); e != nil {
		return e
	}
	*roots = append(*roots, h)
	return nil
}
func shrinkRewriterRoots(m *ast.ASTManager, roots *[]ast.ASTHandle, n uint32) error {
	if uint64(n) > uint64(len(*roots)) {
		return ast.ErrASTDomain
	}
	values := *roots
	for i := int(n); i < len(values); i++ {
		consumed, e := m.DecRefConsumed(values[i])
		next := i
		if consumed {
			values[i] = ast.ASTHandle{}
			next++
		}
		if e != nil || !consumed {
			// Transfer remaining ownership after the preserved prefix. Consumed roots
			// never reappear in the retry, even if their destructor returned an error.
			*roots = append(values[:n], values[next:]...)
			clear(values[len(*roots):])
			if e != nil {
				return e
			}
			return ast.ErrASTPending
		}
	}
	*roots = values[:n]
	return nil
}
func replaceRewriterRoot(m *ast.ASTManager, roots *[]ast.ASTHandle, i uint32, h ast.ASTHandle) error {
	if uint64(i) >= uint64(len(*roots)) {
		return ast.ErrASTDomain
	}
	if e := m.IncRef(h); e != nil {
		return e
	}
	consumed, e := m.DecRefConsumed((*roots)[i])
	if consumed {
		(*roots)[i] = h
	} else {
		// Keep the newly acquired reference named if the old DEC did not consume.
		*roots = append(*roots, h)
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ast.ErrASTPending
	}
	return nil
}
func (c *rewriterCore) resultAt(i uint32) (ast.ASTHandle, error) {
	if uint64(i) >= uint64(len(c.results)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.results[i], nil
}
func (c *rewriterCore) resultBack() (ast.ASTHandle, error) {
	if len(c.results) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.results[len(c.results)-1], nil
}
func (c *rewriterCore) popResult() error {
	if len(c.results) == 0 {
		return ast.ErrASTDomain
	}
	return shrinkRewriterRoots(c.manager, &c.results, uint32(len(c.results)-1))
}
