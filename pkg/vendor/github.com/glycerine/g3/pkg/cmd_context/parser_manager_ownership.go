package cmd_context

// BorrowsManager reports whether reset preserves the context's AST manager.
func (c *Context) BorrowsManager() bool { return c != nil && c.borrowedManager }
