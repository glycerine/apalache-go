package ast

// Source: src/ast/decl_collector.cpp:23-124; concrete AST sort ordering
// through src/util/top_sort.h:31-99.

// astDeclCollector retains borrowed AST identities until its manager closes.
// Datatype constructors and accessors contribute their reached sort edges.
type astDeclCollector struct {
	manager                  *ASTManager
	separatePredicates       bool
	visited                  map[ASTHandle]struct{}
	sorts, decls, predicates []ASTHandle
}

func newASTDeclCollector(m *ASTManager, predicates bool) *astDeclCollector {
	return &astDeclCollector{manager: m, separatePredicates: predicates, visited: make(map[ASTHandle]struct{})}
}

func (c *astDeclCollector) sortParameters(sort ASTHandle) ([]ASTHandle, error) {
	m := c.manager
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return nil, e
	}
	info := m.sortAt(r.slot).info
	if info == (ASTSortInfo{}) {
		return nil, nil
	}
	v, e := m.sortInfo(info)
	if e != nil {
		return nil, e
	}
	var result []ASTHandle
	for i := v.count; i > 0; i-- {
		p, e := m.infoParameter(v, i-1)
		if e != nil {
			return nil, e
		}
		pv, e := m.parameter(p)
		if e != nil {
			return nil, e
		}
		if pv.Kind == ASTParameterAST && pv.ast != (astNodeRef{}) {
			result = append(result, m.external(pv.ast))
		}
	}
	return result, nil
}

func (c *astDeclCollector) visitSort(sort ASTHandle, todo *[]ASTHandle) error {
	m := c.manager
	fid, e := m.GetFamilyID(sort)
	if e != nil {
		return e
	}
	if m.datatypePlugin != nil && fid == m.datatypePlugin.family {
		c.sorts = append(c.sorts, sort)
		constructors, err := m.DatatypeConstructors(sort)
		if err != nil {
			return err
		}
		for _, constructor := range constructors {
			*todo = append(*todo, constructor)
			accessors, err := m.DatatypeConstructorAccessors(constructor)
			if err != nil {
				return err
			}
			*todo = append(*todo, accessors...)
		}
	}
	if fid == NullFamilyID || fid == 4 { // source eager user-sort family ID
		c.sorts = append(c.sorts, sort)
	}
	ps, e := c.sortParameters(sort)
	if e != nil {
		return e
	}
	*todo = append(*todo, ps...)
	return nil
}

func (c *astDeclCollector) visitDecl(decl ASTHandle, todo *[]ASTHandle) error {
	m := c.manager
	count, e := m.GetArity(decl)
	if e != nil {
		return e
	}
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDomain(decl, i)
		if e != nil {
			return e
		}
		*todo = append(*todo, sort)
	}
	rangeSort, e := m.GetRange(decl)
	if e != nil {
		return e
	}
	*todo = append(*todo, rangeSort)
	fid, e := m.GetFamilyID(decl)
	if e != nil {
		return e
	}
	if fid == NullFamilyID {
		if c.separatePredicates && rangeSort == m.external(m.boolSort) {
			c.predicates = append(c.predicates, decl)
		} else {
			c.decls = append(c.decls, decl)
		}
	}
	return nil
}

// visit preserves the source LIFO worklist: an app's declaration is visited
// before its arguments, and a declaration's range before its domains.
func (c *astDeclCollector) visit(root ASTHandle) error {
	if c == nil || c.manager == nil {
		return ErrASTHandle
	}
	m := c.manager
	todo := []ASTHandle{root}
	for len(todo) != 0 {
		last := len(todo) - 1
		h := todo[last]
		todo = todo[:last]
		kind, e := m.GetKind(h)
		if e != nil {
			return e
		}
		if _, present := c.visited[h]; present {
			continue
		}
		c.visited[h] = struct{}{}
		switch kind {
		case ASTApp:
			count, e := m.GetNumArgs(h)
			if e != nil {
				return e
			}
			for i := uint32(0); i < count; i++ {
				arg, e := m.GetArg(h, i)
				if e != nil {
					return e
				}
				todo = append(todo, arg)
			}
			decl, e := m.GetDecl(h)
			if e != nil {
				return e
			}
			todo = append(todo, decl)
		case ASTQuantifier:
			count, e := m.GetNumDeclarations(h)
			if e != nil {
				return e
			}
			for i := uint32(0); i < count; i++ {
				sort, e := m.GetDeclarationSort(h, i)
				if e != nil {
					return e
				}
				todo = append(todo, sort)
			}
			body, e := m.GetQuantifierBody(h)
			if e != nil {
				return e
			}
			todo = append(todo, body)
			count, e = m.GetNumPatterns(h)
			if e != nil {
				return e
			}
			for i := uint32(0); i < count; i++ {
				pattern, e := m.GetPattern(h, i)
				if e != nil {
					return e
				}
				todo = append(todo, pattern)
			}
		case ASTSort:
			if e := c.visitSort(h, &todo); e != nil {
				return e
			}
		case ASTFuncDecl:
			if e := c.visitDecl(h, &todo); e != nil {
				return e
			}
		case ASTVar:
		default:
			return ErrASTDomain
		}
	}
	return nil
}

func (c *astDeclCollector) orderDeps() error {
	if c == nil || c.manager == nil {
		return ErrASTHandle
	}
	var order sortDependencyOrder
	defer order.traceDependencySetsDeallocated()
	for _, sort := range c.sorts {
		if e := order.insertASTSort(c.manager, sort); e != nil {
			return e
		}
	}
	c.sorts = order.order()
	return nil
}
