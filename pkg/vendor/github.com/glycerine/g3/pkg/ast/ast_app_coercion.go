package ast

// Source: src/ast/ast.cpp:2038-2112.

type astAppCoercionFrame struct {
	args        []ASTHandle
	drained     int
	active      bool
	sourceCause error
}

func (m *ASTManager) cleanupAppCoercion(own bool) error {
	f := &m.appCoercion
	if !f.active {
		return nil
	}
	if !own && f.sourceCause != nil {
		return f.sourceCause
	}
	for f.drained < len(f.args) {
		r, e := m.internal(f.args[f.drained])
		if e != nil {
			return e
		}
		consumed, e := m.decNode(r)
		if consumed {
			f.drained++
		}
		if e != nil {
			if f.sourceCause == nil {
				f.sourceCause = e
			}
			return e
		}
	}
	*f = astAppCoercionFrame{}
	return nil
}

func (m *ASTManager) appCoercionNeeded(decl astNodeRef, args AppArguments, associative bool) (bool, error) {
	if !m.coercions || m.arithmeticPlugin == nil {
		return false, nil
	}
	d := m.declAt(decl.slot)
	if !associative && d.arity != args.count {
		return false, nil
	}
	for i := uint32(0); i < args.count; i++ {
		index := i
		if associative {
			index = 0
		}
		expected, e := m.readInternal(d.domain, index)
		if e != nil {
			return false, e
		}
		fid, _, e := m.declIdentity(expected)
		if e != nil {
			return false, e
		}
		if fid != m.arithmeticPlugin.family {
			if associative {
				return false, nil
			}
			continue
		}
		h, e := args.at(i)
		if e != nil {
			return false, e
		}
		r, e := m.requireExpr(h)
		if e != nil {
			return false, e
		}
		actual, e := m.sortOf(r)
		if e != nil {
			return false, e
		}
		if expected != actual {
			return true, nil
		}
	}
	return false, nil
}

func (m *ASTManager) coerceAppArguments(decl astNodeRef, args AppArguments, associative bool) (AppArguments, error) {
	f := &m.appCoercion
	if f.active {
		return AppArguments{}, ErrASTPending
	}
	f.active = true
	d := m.declAt(decl.slot)
	for i := uint32(0); i < args.count; i++ {
		index := i
		if associative {
			index = 0
		}
		expected, e := m.readInternal(d.domain, index)
		if e != nil {
			return AppArguments{}, e
		}
		h, e := args.at(i)
		if e != nil {
			return AppArguments{}, e
		}
		r, e := m.requireExpr(h)
		if e != nil {
			return AppArguments{}, e
		}
		actual, e := m.sortOf(r)
		if e != nil {
			return AppArguments{}, e
		}
		if expected != actual {
			df, kind, e := m.declIdentity(expected)
			if e != nil {
				return AppArguments{}, e
			}
			sf, _, e := m.declIdentity(actual)
			if e != nil {
				return AppArguments{}, e
			}
			if df == m.arithmeticPlugin.family && sf == df {
				op := int32(15)
				if kind == 0 {
					op = 14
				}
				r, e = m.mkFamilyApp(df, op, nil, []ASTHandle{h})
				if e != nil {
					return AppArguments{}, e
				}
				h = m.external(r)
			}
		}
		if e = m.incNode(r); e != nil {
			return AppArguments{}, e
		}
		f.args = append(f.args, h)
	}
	return SliceArguments(f.args), nil
}
