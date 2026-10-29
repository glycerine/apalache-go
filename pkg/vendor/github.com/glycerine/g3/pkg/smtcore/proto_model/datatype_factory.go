package protomodel

// Source: src/smt/proto_model/datatype_factory.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type DatatypeFactory struct {
	structuredFactory
	last    map[ast.ASTHandle]ast.ASTHandle
	utility *ast.DatatypeUtil
	pending []datatypeFactoryPending
	closing bool
}

func NewDatatypeFactory(md *ProtoModel) (*DatatypeFactory, error) {
	if err := md.open(); err != nil {
		return nil, err
	}
	fid, err := md.manager.DatatypeFamilyID()
	if err != nil {
		return nil, err
	}
	utility, err := ast.NewDatatypeUtil(md.manager)
	if err != nil {
		return nil, err
	}
	return &DatatypeFactory{structuredFactory: newStructuredFactory(md.manager, fid, md), last: make(map[ast.ASTHandle]ast.ASTHandle), utility: utility}, nil
}
func datatypeFactoryAssert(line int32, condition string, evaluate func() bool) error {
	return valueFactoryAssert("src/smt/proto_model/datatype_factory.cpp", line, condition, evaluate)
}
func (f *DatatypeFactory) assertNonrecursive(s ast.ASTHandle, line int32) error {
	var queryErr error
	err := datatypeFactoryAssert(line, "!m_util.is_recursive(s)", func() bool {
		var recursive bool
		recursive, queryErr = f.utility.IsRecursive(s)
		return !recursive
	})
	return errors.Join(queryErr, err)
}

// Source get_some_value only looks up the registry. A cold set is published
// by register_value after constructor/arguments/application creation succeeds.
func (f *DatatypeFactory) SomeValue(s ast.ASTHandle) (value ast.ASTHandle, err error) {
	if err = f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if set := f.sets[s]; set != nil && len(set.values) != 0 {
		return set.values[0], nil
	}
	c, _, err := f.utility.NonRecursiveConstructor(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source get_some_value uses ptr_vector<expr>: these arguments borrow
	// the model providers' values, unlike the fresh paths' expr_ref_vector.
	n, err := f.manager.GetArity(c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var args []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		domain, err := f.manager.GetDomain(c, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		arg, err := f.model.SomeValue(domain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, arg)
	}
	value, err = f.manager.MkApp(c, args)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = f.RegisterValue(value); err != nil {
		return value, err
	}
	err = f.trace("get_some_value", 41, nil, func(w *datatypeFactoryTraceWriter) error {
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	})
	return value, err
}

// Approach1 retains each source expr_ref_vector element before the next
// argument is evaluated. Cold SomeValue uses borrowed arguments separately.
func (f *DatatypeFactory) freshConstructorArguments(scope *datatypeFactoryScope, c, s ast.ASTHandle) (used bool, err error) {
	n, err := f.manager.GetArity(c)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		domain, err := f.manager.GetDomain(c, i)
		if err != nil {
			return false, err
		}
		if !used {
			recursive, err := f.utility.IsRecursive(s)
			if err != nil {
				return false, err
			}
			sibling := false
			if recursive {
				sibling, err = f.sibling(s, domain)
				if err != nil {
					return false, err
				}
			}
			if !recursive || !sibling {
				arg, err := f.model.FreshValue(domain)
				if err != nil {
					return false, err
				}
				if !arg.IsNull() {
					if err = scope.push(arg); err != nil {
						return false, err
					}
					used = true
					continue
				}
			}
		}
		arg, err := f.model.SomeValue(domain)
		if err != nil {
			return false, err
		}
		if err = scope.push(arg); err != nil {
			return false, err
		}
	}
	return used, nil
}
func (f *DatatypeFactory) lastValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if value, found := f.last[s]; found {
		err := f.trace("get_last_fresh_value", 51, nil, func(w *datatypeFactoryTraceWriter) error {
			w.text("cached fresh value: ")
			if err := w.ast(value); err != nil {
				return err
			}
			w.text("\n")
			return nil
		})
		return value, err
	}
	set, err := f.valueSet(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var value ast.ASTHandle
	if len(set.values) == 0 {
		value, err = f.SomeValue(s)
	} else {
		value = set.values[0]
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	recursive, err := f.utility.IsRecursive(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if recursive {
		f.last[s] = value
	}
	return value, nil
}

// Use the actual contains_app specialization: it owns its target/root,
// traverses quantifier bodies and deliberately ignores their annotations.
func (f *DatatypeFactory) contains(root, needle ast.ASTHandle) (found bool, err error) {
	contains, err := ast.NewContainsApp(f.manager, needle)
	if err != nil {
		return false, err
	}
	defer f.finishContains(contains, &err)
	return contains.Check(root)
}
func (f *DatatypeFactory) isSubtermOfLastValue(value ast.ASTHandle) (result bool, err error) {
	sort, err := f.manager.GetSort(value)
	if err != nil {
		return false, err
	}
	last, found := f.last[sort]
	if !found {
		return false, nil
	}
	contains, err := ast.NewContainsApp(f.manager, value)
	if err != nil {
		return false, err
	}
	defer f.finishContains(contains, &err)
	result, err = contains.Check(last)
	if err != nil {
		return false, err
	}
	err = f.trace("is_subterm_of_last_value", 71, nil, func(w *datatypeFactoryTraceWriter) error {
		if err := w.ast(value); err != nil {
			return err
		}
		w.text(" in ")
		if err := w.ast(last); err != nil {
			return err
		}
		w.text(" ")
		w.bit(result)
		w.text("\n")
		return nil
	})
	return result, err
}
func (f *DatatypeFactory) isDatatype(s ast.ASTHandle) (bool, error) {
	family, err := f.manager.GetFamilyID(s)
	if err != nil || family != f.family {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(s)
	return kind == ast.ASTDatatypeSort, err
}
func (f *DatatypeFactory) sibling(s, arg ast.ASTHandle) (bool, error) {
	datatype, err := f.isDatatype(arg)
	if err != nil || !datatype {
		return false, err
	}
	return f.manager.DatatypeAreSiblings(s, arg)
}
func (f *DatatypeFactory) almostFresh(s ast.ASTHandle) (ast.ASTHandle, error) {
	set, err := f.valueSet(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(set.values) == 0 {
		value, err := f.SomeValue(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = datatypeFactoryAssert(83, "val", func() bool { return !value.IsNull() }); err != nil {
			return ast.ASTHandle{}, err
		}
		recursive, err := f.utility.IsRecursive(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if recursive {
			f.last[s] = value
		}
		return value, nil
	}
	constructors, err := f.utility.Constructors(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, c := range constructors {
		value, accepted, err := f.almostConstructor(s, c, set)
		if err != nil || accepted {
			return value, err
		}
	}
	return ast.ASTHandle{}, f.assertNonrecursive(s, 133)
}
func (f *DatatypeFactory) almostConstructor(s, c ast.ASTHandle, set *factoryValueSet) (value ast.ASTHandle, accepted bool, err error) {
	scope := &datatypeFactoryScope{manager: f.manager}
	defer f.finishScope(scope, &err)
	n, err := f.manager.GetArity(c)
	if err != nil {
		return value, false, err
	}
	fresh, recursive := false, false
	for i := uint32(0); i < n; i++ {
		domain, err := f.manager.GetDomain(c, i)
		if err != nil {
			return value, false, err
		}
		if !fresh {
			sibling, err := f.sibling(s, domain)
			if err != nil {
				return value, false, err
			}
			if !sibling {
				arg, err := f.model.FreshValue(domain)
				if err != nil {
					return value, false, err
				}
				if !arg.IsNull() {
					if err = scope.push(arg); err != nil {
						return value, false, err
					}
					fresh = true
					continue
				}
			}
		}
		var arg ast.ASTHandle
		if !fresh {
			sibling, e := f.sibling(s, domain)
			if e != nil {
				return value, false, e
			}
			if sibling {
				recursive = true
				arg, err = f.lastValue(domain)
			} else {
				arg, err = f.model.SomeValue(domain)
			}
		} else {
			arg, err = f.model.SomeValue(domain)
		}
		if err != nil {
			return value, false, err
		}
		if err = scope.push(arg); err != nil {
			return value, false, err
		}
	}
	if !recursive && !fresh {
		return value, false, nil
	}
	value, err = f.manager.MkApp(c, scope.values)
	if err != nil {
		return value, false, err
	}
	if err = datatypeFactoryAssert(119, "!found_fresh_arg || !set->contains(new_value)", func() bool { _, seen := set.seen[value]; return !fresh || !seen }); err != nil {
		return value, false, err
	}
	if err = f.RegisterValue(value); err != nil {
		return value, false, err
	}
	rec, err := f.utility.IsRecursive(s)
	if err != nil {
		return value, false, err
	}
	if rec {
		contains, err := f.isSubtermOfLastValue(value)
		if err != nil {
			return value, false, err
		}
		if contains {
			value = f.last[s]
		} else {
			f.last[s] = value
		}
	}
	err = f.trace("get_almost_fresh_value", 129, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("almost fresh: ")
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	})
	return value, true, err
}
func (f *DatatypeFactory) FreshValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := f.trace("get_fresh_value", 139, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("generating fresh value for: ")
		if err := w.name(s); err != nil {
			return err
		}
		w.text("\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	set, err := f.valueSet(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(set.values) == 0 {
		value, err := f.SomeValue(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		recursive, err := f.utility.IsRecursive(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if recursive {
			f.last[s] = value
		}
		err = f.trace("get_fresh_value", 147, nil, func(w *datatypeFactoryTraceWriter) error {
			w.text("0. result: ")
			if err := w.ast(value); err != nil {
				return err
			}
			w.text("\n")
			return nil
		})
		return value, err
	}
	constructors, err := f.utility.Constructors(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, c := range constructors {
		value, accepted, err := f.freshConstructor(s, c, set)
		if err != nil || accepted {
			return value, err
		}
	}
	recursive, err := f.utility.IsRecursive(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !recursive {
		return ast.ASTHandle{}, f.assertNonrecursive(s, 246)
	}
	for iteration := uint32(1); ; iteration++ {
		if err := f.trace("get_fresh_value", 191, nil, func(w *datatypeFactoryTraceWriter) error {
			value, err := f.lastValue(s)
			if err != nil {
				return err
			}
			if err = w.ast(value); err != nil {
				return err
			}
			w.text("\n")
			return nil
		}); err != nil {
			return ast.ASTHandle{}, err
		}
		constructors, err = f.utility.Constructors(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for _, c := range constructors {
			value, accepted, err := f.recursiveConstructor(s, c, set, iteration)
			if err != nil || accepted {
				return value, err
			}
		}
	}
}
func (f *DatatypeFactory) freshConstructor(s, c ast.ASTHandle, set *factoryValueSet) (value ast.ASTHandle, accepted bool, err error) {
	scope := &datatypeFactoryScope{manager: f.manager}
	defer f.finishScope(scope, &err)
	used, err := f.freshConstructorArguments(scope, c, s)
	if err != nil {
		return value, false, err
	}
	value, err = f.manager.MkApp(c, scope.values)
	if err != nil {
		return value, false, err
	}
	if err = scope.ownCandidate(value); err != nil {
		return value, false, err
	}
	if err = f.trace("get_fresh_value", 174, func() bool { _, seen := set.seen[value]; return used && seen }, func(w *datatypeFactoryTraceWriter) error {
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	}); err != nil {
		return value, false, err
	}
	if err = datatypeFactoryAssert(175, "!found_fresh_arg || !set->contains(new_value)", func() bool { _, seen := set.seen[value]; return !used || !seen }); err != nil {
		return value, false, err
	}
	if _, seen := set.seen[value]; seen {
		return value, false, nil
	}
	if err = f.RegisterValue(value); err != nil {
		return value, false, err
	}
	recursive, err := f.utility.IsRecursive(s)
	if err != nil {
		return value, false, err
	}
	if recursive {
		f.last[s] = value
	}
	err = f.trace("get_fresh_value", 180, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("1. result: ")
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	})
	return value, true, err
}
func (f *DatatypeFactory) recursiveConstructor(s, c ast.ASTHandle, set *factoryValueSet, iteration uint32) (value ast.ASTHandle, accepted bool, err error) {
	scope := &datatypeFactoryScope{manager: f.manager}
	defer f.finishScope(scope, &err)
	n, err := f.manager.GetArity(c)
	if err != nil {
		return value, false, err
	}
	if err = f.trace("get_fresh_value", 197, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("checking constructor: ")
		if err := w.name(c); err != nil {
			return err
		}
		w.text("\n")
		return nil
	}); err != nil {
		return value, false, err
	}
	foundSibling := false
	for i := uint32(0); i < n; i++ {
		domain, err := f.manager.GetDomain(c, i)
		if err != nil {
			return value, false, err
		}
		if err = f.trace("get_fresh_value", 200, nil, func(w *datatypeFactoryTraceWriter) error {
			if err := w.ast(s); err != nil {
				return err
			}
			w.text(" ")
			if err := w.ast(domain); err != nil {
				return err
			}
			w.text(" are_siblings ")
			siblings, err := f.manager.DatatypeAreSiblings(s, domain)
			if err != nil {
				return err
			}
			w.bit(siblings)
			w.text("  is_datatype ")
			datatype, err := f.isDatatype(domain)
			if err != nil {
				return err
			}
			w.bit(datatype)
			w.text(" found_sibling ")
			w.bit(foundSibling)
			w.text("\n")
			return nil
		}); err != nil {
			return value, false, err
		}
		sibling := false
		if !foundSibling {
			sibling, err = f.sibling(s, domain)
			if err != nil {
				return value, false, err
			}
		}
		var arg ast.ASTHandle
		if !foundSibling && sibling {
			foundSibling = true
			if iteration <= 1 {
				arg, err = f.almostFresh(domain)
			} else {
				arg, err = f.FreshValue(domain)
			}
			if err != nil {
				return value, false, err
			}
			if arg.IsNull() {
				if err = f.trace("get_fresh_value", 215, nil, func(w *datatypeFactoryTraceWriter) error {
					w.text("no argument found for ")
					if err := w.ast(domain); err != nil {
						return err
					}
					w.text("\n")
					return nil
				}); err != nil {
					return value, false, err
				}
				arg, err = f.model.SomeValue(domain)
				if err != nil {
					return value, false, err
				}
				foundSibling = false
			}
			if err = datatypeFactoryAssert(220, "maybe_new_arg", func() bool { return !arg.IsNull() }); err != nil {
				return value, false, err
			}
		} else {
			arg, err = f.model.SomeValue(domain)
			if err != nil {
				return value, false, err
			}
			if err = datatypeFactoryAssert(225, "some_arg", func() bool { return !arg.IsNull() }); err != nil {
				return value, false, err
			}
		}
		if err = scope.push(arg); err != nil {
			return value, false, err
		}
	}
	if !foundSibling {
		return value, false, nil
	}
	value, err = f.manager.MkApp(c, scope.values)
	if err != nil {
		return value, false, err
	}
	if err = scope.ownCandidate(value); err != nil {
		return value, false, err
	}
	if err = f.trace("get_fresh_value", 232, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("potential new value: ")
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	}); err != nil {
		return value, false, err
	}
	f.last[s] = value
	if _, seen := set.seen[value]; seen {
		return value, false, nil
	}
	if err = f.RegisterValue(value); err != nil {
		return value, false, err
	}
	err = f.trace("get_fresh_value", 236, nil, func(w *datatypeFactoryTraceWriter) error {
		w.text("2. result: ")
		if err := w.ast(value); err != nil {
			return err
		}
		w.text("\n")
		return nil
	})
	return value, true, err
}
func (f *DatatypeFactory) SomeValues(s ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	return f.structuredFactory.SomeValues(s, f, first, second)
}
func (f *DatatypeFactory) open() error {
	if f == nil || f.closing || len(f.pending) != 0 {
		return ast.ErrASTHandle
	}
	return f.structuredFactory.open()
}
func (f *DatatypeFactory) RegisterValue(value ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.structuredFactory.RegisterValue(value)
}
func (f *DatatypeFactory) Close() error {
	if f == nil {
		return nil
	}
	f.closing = true
	// Failed lexical cleanups were observed innermost first. Complete them
	// in that order before destroying the containing factory members.
	for len(f.pending) != 0 {
		p := &f.pending[0]
		var err error
		if p.trace != nil {
			err = p.trace.close()
		} else if p.contains != nil {
			err = p.contains.Close()
		} else {
			err = p.scope.close()
		}
		if err != nil {
			return err
		}
		f.pending = f.pending[1:]
	}
	f.pending = nil
	f.last = nil
	if f.utility != nil {
		if err := f.utility.Close(); err != nil {
			return err
		}
	}
	return f.structuredFactory.Close()
}
