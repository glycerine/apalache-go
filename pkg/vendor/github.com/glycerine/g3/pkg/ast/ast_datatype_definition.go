package ast

// Source: src/ast/datatype_decl_plugin.cpp:27-147,346-429,675-736,753-872,
// 980-1024; src/ast/datatype_decl_plugin.h:50-104,192-220.

import (
	"errors"
	"strconv"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// DatatypeAccessorDefinition describes one constructor field. Recursive
// fields name a datatype in the same MkDatatypes call by zero-based index;
// other fields use Range.
type DatatypeAccessorDefinition struct {
	Name           usymbol.Symbol
	Range          ASTHandle
	Recursive      bool
	RecursiveIndex uint32
}

// DatatypeConstructorDefinition describes one constructor and its recognizer.
type DatatypeConstructorDefinition struct {
	Name       usymbol.Symbol
	Recognizer usymbol.Symbol
	Accessors  []DatatypeAccessorDefinition
}

// DatatypeDefinition is a concrete datatype declaration. Parameters are sort
// placeholders used by parametric datatype definitions.
type DatatypeDefinition struct {
	Name         usymbol.Symbol
	Parameters   []ASTHandle
	Constructors []DatatypeConstructorDefinition
}

// MkDatatypes installs one mutually recursive datatype definition block and
// returns the block's canonical sorts in input order.
func (m *ASTManager) MkDatatypes(defs []DatatypeDefinition) (result []ASTHandle, err error) {
	if err = m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	p := m.datatypePlugin
	if p == nil || !p.published || len(defs) == 0 {
		return nil, ErrASTDomain
	}

	names := make(map[usymbol.Symbol]struct{}, len(defs))
	sorts := make([]astNodeRef, len(defs))
	built := make([]*astDatatypeDefinition, len(defs))
	acquired := make([]astNodeRef, 0, len(defs))
	rollback := true
	defer func() {
		if !rollback {
			return
		}
		for i := len(acquired) - 1; i >= 0; i-- {
			_, releaseErr := m.decNode(acquired[i])
			err = errors.Join(err, releaseErr)
		}
	}()

	p.classID++
	for i := range defs {
		d := &defs[i]
		if _, e := m.Symbols.Hash(d.Name); e != nil {
			return nil, e
		}
		if _, found := names[d.Name]; found || len(d.Constructors) == 0 {
			return nil, datatypeDeclarationError("invalid datatype")
		}
		names[d.Name] = struct{}{}
		parameters := make([]ASTParameter, 0, len(d.Parameters)+1)
		nameParameter, e := m.newParameterSymbol(d.Name)
		if e != nil {
			return nil, e
		}
		parameters = append(parameters, nameParameter)
		for _, h := range d.Parameters {
			r, e := m.requireKind(h, ASTSort)
			if e != nil {
				for j := len(parameters) - 1; j >= 0; j-- {
					_ = m.dropParameter(parameters[j])
				}
				return nil, e
			}
			parameter, e := m.newParameterAST(m.external(r))
			if e != nil {
				for j := len(parameters) - 1; j >= 0; j-- {
					_ = m.dropParameter(parameters[j])
				}
				return nil, e
			}
			parameters = append(parameters, parameter)
		}
		sort, e := m.datatypeSort(ASTDatatypeSort, parameters)
		for j := len(parameters) - 1; j >= 0; j-- {
			e = errors.Join(e, m.dropParameter(parameters[j]))
		}
		if e != nil {
			return nil, e
		}
		if e = m.incNode(sort); e != nil {
			return nil, e
		}
		acquired = append(acquired, sort)
		sorts[i] = sort
		definition := &astDatatypeDefinition{name: d.Name, classID: p.classID, sort: sort}
		for _, h := range d.Parameters {
			r, _ := m.requireKind(h, ASTSort)
			if e = m.incNode(r); e != nil {
				return nil, e
			}
			acquired = append(acquired, r)
			definition.parameters = append(definition.parameters, r)
		}
		built[i] = definition
	}

	for i := range defs {
		for _, inputConstructor := range defs[i].Constructors {
			if _, e := m.Symbols.Hash(inputConstructor.Name); e != nil {
				return nil, e
			}
			if _, e := m.Symbols.Hash(inputConstructor.Recognizer); e != nil {
				return nil, e
			}
			constructor := astDatatypeConstructorDefinition{name: inputConstructor.Name, recognizer: inputConstructor.Recognizer}
			for _, inputAccessor := range inputConstructor.Accessors {
				if _, e := m.Symbols.Hash(inputAccessor.Name); e != nil {
					return nil, e
				}
				var field astNodeRef
				if inputAccessor.Recursive {
					if inputAccessor.RecursiveIndex >= uint32(len(sorts)) {
						return nil, datatypeDeclarationError("invalid recursive datatype reference")
					}
					field = sorts[inputAccessor.RecursiveIndex]
				} else {
					var e error
					field, e = m.requireKind(inputAccessor.Range, ASTSort)
					if e != nil {
						return nil, e
					}
				}
				if e := m.incNode(field); e != nil {
					return nil, e
				}
				acquired = append(acquired, field)
				constructor.ranges = append(constructor.ranges, field)
				constructor.accessors = append(constructor.accessors, inputAccessor.Name)
			}
			built[i].constructors = append(built[i].constructors, constructor)
		}
	}

	wellFounded := make([]bool, len(built))
	count := 0
	changed := true
	for changed && count != len(built) {
		changed = false
		for i, definition := range built {
			if wellFounded[i] {
				continue
			}
			for _, constructor := range definition.constructors {
				ok := true
				for _, field := range constructor.ranges {
					for j, sort := range sorts {
						if field == sort && !wellFounded[j] {
							ok = false
							break
						}
					}
					if !ok {
						break
					}
				}
				if ok {
					wellFounded[i] = true
					count++
					changed = true
					break
				}
			}
		}
	}
	if count != len(built) {
		return nil, datatypeDeclarationError("datatype is not well-founded")
	}

	previous := make([]*astDatatypeDefinition, len(built))
	for i, definition := range built {
		previous[i] = p.definitionNames[definition.name]
		if previous[i] != nil {
			delete(p.definitions, previous[i].sort)
		}
		p.definitionNames[definition.name] = definition
		p.definitions[definition.sort] = definition
		result = append(result, m.external(sorts[i]))
	}
	for _, sort := range sorts {
		if e := m.datatypeSetCardinality(sort); e != nil {
			for i, definition := range built {
				delete(p.definitions, definition.sort)
				if previous[i] == nil {
					delete(p.definitionNames, definition.name)
				} else {
					p.definitionNames[definition.name] = previous[i]
					p.definitions[previous[i].sort] = previous[i]
				}
			}
			return nil, e
		}
	}
	p.definitionRefs = append(p.definitionRefs, acquired...)
	rollback = false
	return result, nil
}

func (m *ASTManager) datatypeDefinitionForSort(sort astNodeRef) (*astDatatypeDefinition, error) {
	p := m.datatypePlugin
	if p == nil {
		return nil, ErrASTDomain
	}
	if definition := p.definitions[sort]; definition != nil {
		return definition, nil
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil || info.family != p.family || info.kind != ASTDatatypeSort || info.count == 0 {
		if err != nil {
			return nil, err
		}
		return nil, ErrASTDomain
	}
	parameter, err := m.infoParameter(info, 0)
	if err != nil {
		return nil, err
	}
	value, err := m.parameter(parameter)
	if err != nil || value.Kind != ASTParameterSymbol {
		if err != nil {
			return nil, err
		}
		return nil, ErrASTDomain
	}
	definition := p.definitionNames[value.symbol]
	if definition == nil {
		return nil, ErrASTDomain
	}
	return definition, nil
}

func (m *ASTManager) datatypeSortArguments(sort astNodeRef, definition *astDatatypeDefinition) ([]astNodeRef, error) {
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return nil, err
	}
	if info.count != uint32(len(definition.parameters)+1) {
		return nil, ErrASTDomain
	}
	result := make([]astNodeRef, 0, len(definition.parameters))
	for i := uint32(1); i < info.count; i++ {
		parameter, e := m.infoParameter(info, i)
		if e != nil {
			return nil, e
		}
		value, e := m.parameter(parameter)
		if e != nil {
			return nil, e
		}
		if value.Kind != ASTParameterAST || m.validNode(value.ast) != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			return nil, ErrASTDomain
		}
		result = append(result, value.ast)
	}
	return result, nil
}

func (m *ASTManager) datatypeSubstituteSort(sort astNodeRef, source, destination []astNodeRef) (result astNodeRef, err error) {
	for i := range source {
		if sort == source[i] {
			return destination[i], nil
		}
	}
	record := m.sortAt(sort.slot)
	if record.info == (ASTSortInfo{}) {
		return sort, nil
	}
	info, err := m.sortInfo(record.info)
	if err != nil {
		return astNodeRef{}, err
	}
	parameters := make([]ASTParameter, 0, info.count)
	created := make([]ASTParameter, 0, info.count)
	defer func() {
		for i := len(created) - 1; i >= 0; i-- {
			err = errors.Join(err, m.dropParameter(created[i]))
		}
	}()
	changed := false
	for i := uint32(0); i < info.count; i++ {
		parameter, e := m.infoParameter(info, i)
		if e != nil {
			return astNodeRef{}, e
		}
		value, e := m.parameter(parameter)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterAST {
			parameters = append(parameters, parameter)
			continue
		}
		if e = m.validNode(value.ast); e != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			if e != nil {
				return astNodeRef{}, e
			}
			return astNodeRef{}, ErrASTDomain
		}
		replacement, e := m.datatypeSubstituteSort(value.ast, source, destination)
		if e != nil {
			return astNodeRef{}, e
		}
		p, e := m.newParameterAST(m.external(replacement))
		if e != nil {
			return astNodeRef{}, e
		}
		created = append(created, p)
		parameters = append(parameters, p)
		changed = true
	}
	if !changed {
		return sort, nil
	}
	slot, generation, err := m.newInfo(true, info.family, info.kind, parameters, info.size, ASTFuncFlags{}, info.private)
	if err != nil {
		return astNodeRef{}, err
	}
	defer func() { err = errors.Join(err, m.dropInfoStorage(slot, generation)) }()
	return m.mkSort(record.name, ASTSortInfo{m.origin, slot, generation})
}

func (m *ASTManager) datatypeInstantiatedRanges(sort astNodeRef, definition *astDatatypeDefinition, constructor *astDatatypeConstructorDefinition) ([]astNodeRef, error) {
	arguments, err := m.datatypeSortArguments(sort, definition)
	if err != nil {
		return nil, err
	}
	result := make([]astNodeRef, len(constructor.ranges))
	for i, field := range constructor.ranges {
		result[i], err = m.datatypeSubstituteSort(field, definition.parameters, arguments)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *ASTManager) datatypeConstructorDecl(sort astNodeRef, definition *astDatatypeDefinition, constructor *astDatatypeConstructorDefinition) (astNodeRef, error) {
	ranges, err := m.datatypeInstantiatedRanges(sort, definition, constructor)
	if err != nil {
		return astNodeRef{}, err
	}
	domain := make([]ASTHandle, len(ranges))
	for i, field := range ranges {
		domain[i] = m.external(field)
	}
	parameter, err := m.newParameterSymbol(constructor.name)
	if err != nil {
		return astNodeRef{}, err
	}
	defer m.dropParameter(parameter)
	return m.datatypeDecl(ASTDatatypeConstructor, []ASTParameter{parameter}, domain, m.external(sort))
}

func (m *ASTManager) datatypeConstructors(sort astNodeRef) ([]ASTHandle, error) {
	if err := datatypeUtilAssert(753, "is_datatype(ty)", func() bool { return m.datatypeUtilIsDatatype(sort) }); err != nil {
		return nil, err
	}
	definition, err := m.datatypeDefinitionForSort(sort)
	if err != nil {
		return nil, err
	}
	result := make([]ASTHandle, 0, len(definition.constructors))
	for i := range definition.constructors {
		decl, e := m.datatypeConstructorDecl(sort, definition, &definition.constructors[i])
		if e != nil {
			return nil, e
		}
		result = append(result, m.external(decl))
	}
	return result, nil
}

func (m *ASTManager) DatatypeConstructors(sort ASTHandle) ([]ASTHandle, error) {
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return nil, err
	}
	return m.datatypeConstructors(r)
}

func (m *ASTManager) DatatypeNumConstructors(sort ASTHandle) (uint32, error) {
	if err := m.lock(); err != nil {
		return 0, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return 0, err
	}
	d, err := m.datatypeDefinitionForSort(r)
	if err != nil {
		return 0, err
	}
	return uint32(len(d.constructors)), nil
}

func (m *ASTManager) DatatypeConstructorIndex(constructor ASTHandle) (uint32, bool, error) {
	if err := m.lock(); err != nil {
		return 0, false, err
	}
	defer m.mu.Unlock()
	c, err := m.requireKind(constructor, ASTFuncDecl)
	if err != nil {
		return 0, false, err
	}
	family, kind, err := m.declIdentity(c)
	if err != nil || m.datatypePlugin == nil || family != m.datatypePlugin.family || kind != ASTDatatypeConstructor {
		return 0, false, err
	}
	d, err := m.datatypeDefinitionForSort(m.declAt(c.slot).rangeSort)
	if err != nil {
		return 0, false, err
	}
	name, err := m.datatypeDeclSymbolParameter(c, 0)
	if err != nil {
		return 0, false, err
	}
	for i := range d.constructors {
		if d.constructors[i].name == name {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}

func (m *ASTManager) DatatypeIsEnumSort(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	if m.datatypePlugin == nil {
		return false, nil
	}
	info := m.sortAt(r.slot).info
	if info == (ASTSortInfo{}) {
		return false, nil
	}
	identity, err := m.sortInfo(info)
	if err != nil {
		return false, err
	}
	if identity.family != m.datatypePlugin.family || identity.kind != ASTDatatypeSort {
		return false, nil
	}
	d, err := m.datatypeDefinitionForSort(r)
	if err != nil {
		return false, err
	}
	for i := range d.constructors {
		if len(d.constructors[i].ranges) != 0 {
			return false, nil
		}
	}
	return true, nil
}

func (m *ASTManager) DatatypeAreSiblings(left, right ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	l, err := m.requireKind(left, ASTSort)
	if err != nil {
		return false, err
	}
	r, err := m.requireKind(right, ASTSort)
	if err != nil {
		return false, err
	}
	ld, le := m.datatypeDefinitionForSort(l)
	rd, re := m.datatypeDefinitionForSort(r)
	if le != nil || re != nil {
		return l == r, nil
	}
	return ld.classID == rd.classID, nil
}

func (m *ASTManager) DatatypeConstructorAccessors(constructor ASTHandle) ([]ASTHandle, error) {
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	c, err := m.requireKind(constructor, ASTFuncDecl)
	if err != nil {
		return nil, err
	}
	definition, err := m.datatypeDefinitionForSort(m.declAt(c.slot).rangeSort)
	if err != nil {
		return nil, err
	}
	name, err := m.datatypeDeclSymbolParameter(c, 0)
	if err != nil {
		return nil, err
	}
	for ci := range definition.constructors {
		candidate := &definition.constructors[ci]
		if candidate.name != name {
			continue
		}
		ranges, e := m.datatypeInstantiatedRanges(m.declAt(c.slot).rangeSort, definition, candidate)
		if e != nil {
			return nil, e
		}
		result := make([]ASTHandle, 0, len(candidate.accessors))
		for i, accessorName := range candidate.accessors {
			p0, e := m.newParameterSymbol(accessorName)
			if e != nil {
				return nil, e
			}
			p1, e := m.newParameterSymbol(candidate.name)
			if e != nil {
				_ = m.dropParameter(p0)
				return nil, e
			}
			decl, e := m.datatypeDecl(ASTDatatypeAccessor, []ASTParameter{p0, p1}, []ASTHandle{m.external(m.declAt(c.slot).rangeSort)}, m.external(ranges[i]))
			e = errors.Join(e, m.dropParameter(p1), m.dropParameter(p0))
			if e != nil {
				return nil, e
			}
			result = append(result, m.external(decl))
		}
		return result, nil
	}
	return nil, ErrASTDomain
}

func (m *ASTManager) DatatypeConstructorRecognizer(constructor ASTHandle) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	c, err := m.requireKind(constructor, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, err
	}
	definition, err := m.datatypeDefinitionForSort(m.declAt(c.slot).rangeSort)
	if err != nil {
		return ASTHandle{}, err
	}
	name, err := m.datatypeDeclSymbolParameter(c, 0)
	if err != nil {
		return ASTHandle{}, err
	}
	for i := range definition.constructors {
		candidate := &definition.constructors[i]
		if candidate.name != name {
			continue
		}
		p0, e := m.newParameterAST(m.external(c))
		if e != nil {
			return ASTHandle{}, e
		}
		p1, e := m.newParameterSymbol(candidate.recognizer)
		if e != nil {
			_ = m.dropParameter(p0)
			return ASTHandle{}, e
		}
		decl, e := m.datatypeDecl(ASTDatatypeRecognizer, []ASTParameter{p0, p1}, []ASTHandle{m.external(m.declAt(c.slot).rangeSort)}, ASTHandle{})
		e = errors.Join(e, m.dropParameter(p1), m.dropParameter(p0))
		return m.external(decl), e
	}
	return ASTHandle{}, ErrASTDomain
}

func (m *ASTManager) DatatypeRecognizerConstructorIndex(recognizer ASTHandle) (uint32, bool, error) {
	if err := m.lock(); err != nil {
		return 0, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(recognizer, ASTFuncDecl)
	if err != nil {
		return 0, false, err
	}
	family, kind, err := m.declIdentity(r)
	if err != nil || m.datatypePlugin == nil || family != m.datatypePlugin.family || kind != ASTDatatypeRecognizer && kind != ASTDatatypeIs {
		return 0, false, err
	}
	constructor, err := m.datatypeDeclASTParameter(r, 0)
	if err != nil {
		return 0, false, err
	}
	definition, err := m.datatypeDefinitionForSort(m.declAt(constructor.slot).rangeSort)
	if err != nil {
		return 0, false, err
	}
	name, err := m.datatypeDeclSymbolParameter(constructor, 0)
	if err != nil {
		return 0, false, err
	}
	for i := range definition.constructors {
		if definition.constructors[i].name == name {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}

func (m *ASTManager) DatatypeAccessorConstructor(accessor ASTHandle) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	a, err := m.requireKind(accessor, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	family, kind, err := m.declIdentity(a)
	if err != nil || m.datatypePlugin == nil || family != m.datatypePlugin.family || kind != ASTDatatypeAccessor {
		return ASTHandle{}, false, err
	}
	domain, err := m.readInternal(m.declAt(a.slot).domain, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	definition, err := m.datatypeDefinitionForSort(domain)
	if err != nil {
		return ASTHandle{}, false, err
	}
	constructorName, err := m.datatypeDeclSymbolParameter(a, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	for i := range definition.constructors {
		if definition.constructors[i].name == constructorName {
			decl, e := m.datatypeConstructorDecl(domain, definition, &definition.constructors[i])
			return m.external(decl), e == nil, e
		}
	}
	return ASTHandle{}, false, nil
}

func (m *ASTManager) DatatypeNumParameters(sort ASTHandle) (uint32, error) {
	if err := m.lock(); err != nil {
		return 0, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return 0, err
	}
	d, err := m.datatypeDefinitionForSort(r)
	if err != nil {
		return 0, err
	}
	arguments, err := m.datatypeSortArguments(r, d)
	return uint32(len(arguments)), err
}

func (m *ASTManager) DatatypeParameter(sort ASTHandle, index uint32) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, err
	}
	d, err := m.datatypeDefinitionForSort(r)
	if err != nil {
		return ASTHandle{}, err
	}
	arguments, err := m.datatypeSortArguments(r, d)
	if err != nil {
		return ASTHandle{}, err
	}
	if index >= uint32(len(arguments)) {
		return ASTHandle{}, ErrASTDomain
	}
	return m.external(arguments[index]), nil
}

func (m *ASTManager) datatypeSubsorts(sort astNodeRef, result *[]astNodeRef) error {
	*result = append(*result, sort)
	record := m.sortAt(sort.slot)
	if record.info == (ASTSortInfo{}) {
		return nil
	}
	info, err := m.sortInfo(record.info)
	if err != nil {
		return err
	}
	for i := uint32(0); i < info.count; i++ {
		parameter, e := m.infoParameter(info, i)
		if e != nil {
			return e
		}
		value, e := m.parameter(parameter)
		if e != nil {
			return e
		}
		if value.Kind == ASTParameterAST && m.validNode(value.ast) == nil && m.nodeAt(value.ast.slot).kind == ASTSort {
			if e = m.datatypeSubsorts(value.ast, result); e != nil {
				return e
			}
		}
	}
	return nil
}

// Source datatype::util::is_recursive_core: process the last queued sort,
// revisit its fields after queued descendants, and distinguish gray/black.
func (m *ASTManager) datatypeRecursive(sort astNodeRef) (bool, error) {
	marks := make(map[astNodeRef]uint8)
	todo := []astNodeRef{sort}
	var subsorts []astNodeRef
	for len(todo) != 0 {
		sort := todo[len(todo)-1]
		if marks[sort] == 2 {
			todo = todo[:len(todo)-1]
			continue
		}
		marks[sort] = 1
		definition, err := m.datatypeDefinitionForSort(sort)
		if err != nil {
			return false, err
		}
		canProcess := true
		// Source get_def(s) returns the named definition, not instantiated
		// constructor domains. Its accessor ranges still contain formals.
		for ci := range definition.constructors {
			for _, field := range definition.constructors[ci].ranges {
				subsorts = subsorts[:0]
				if err = m.datatypeSubsorts(field, &subsorts); err != nil {
					return false, err
				}
				for _, nested := range subsorts {
					if !m.datatypeUtilIsDatatype(nested) {
						continue
					}
					if mark, found := marks[nested]; found {
						if mark == 1 {
							return true, nil
						}
					} else {
						todo = append(todo, nested)
						canProcess = false
					}
				}
			}
		}
		if canProcess {
			marks[sort] = 2
			todo = todo[:len(todo)-1]
		}
	}
	return false, nil
}

func (m *ASTManager) DatatypeIsRecursive(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	return m.datatypeRecursive(r)
}

func (m *ASTManager) datatypeNonRecursiveConstructor(sort astNodeRef) (astNodeRef, bool, error) {
	p := m.datatypePlugin
	if p == nil {
		return astNodeRef{}, false, ErrASTHandle
	}
	if p.utility == nil {
		utility, err := m.newDatatypeUtil()
		if err != nil {
			return astNodeRef{}, false, err
		}
		p.utility = utility
	}
	h, found, err := p.utility.nonRecursiveConstructor(m.external(sort))
	return astNodeRef{slot: h.slot, generation: h.generation}, found, err
}

func (m *ASTManager) datatypeNonRecursiveConstructorCore(sort astNodeRef, forbidden []astNodeRef, counter *uint32, utility *DatatypeUtil) (astNodeRef, bool, error) {
	// Source get_datatype_constructors instantiates every declaration before
	// the selection counter advances, including declarations never selected.
	constructors, err := utility.constructorList(sort)
	if err != nil {
		return astNodeRef{}, false, err
	}
	n := uint32(len(constructors))
	if err := utility.traceDatatype(937, func(w *datatypeTraceWriter) error {
		w.text("get-non-rec constructor: ")
		if err := w.ast(sort); err != nil {
			return err
		}
		if err := w.end("\n"); err != nil {
			return err
		}
		w.text("forbidden: ")
		for _, s := range forbidden {
			if err := w.ast(s); err != nil {
				return err
			}
			if err := w.end(" "); err != nil {
				return err
			}
		}
		w.text("\nconstructors: ")
		w.text(strconv.FormatUint(uint64(n), 10))
		w.text("\n")
		for _, h := range constructors {
			if err := w.ast(astNodeRef{slot: h.slot, generation: h.generation}); err != nil {
				return err
			}
			if err := w.end("\n"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return astNodeRef{}, false, err
	}
	*counter++
	start := *counter
	for j := uint32(0); j < n; j++ {
		h := constructors[(j+start)%n]
		if err := utility.traceDatatype(948, func(w *datatypeTraceWriter) error {
			w.text("checking ")
			if err := w.ast(sort); err != nil {
				return err
			}
			w.text(": ")
			if err := w.ast(astNodeRef{slot: h.slot, generation: h.generation}); err != nil {
				return err
			}
			return w.end("\n")
		}); err != nil {
			return astNodeRef{}, false, err
		}
		c := m.declAt(h.slot)
		i := uint32(0)
		for ; i < c.arity; i++ {
			field, err := m.readInternal(c.domain, i)
			if err != nil {
				return astNodeRef{}, false, err
			}
			if m.datatypeUtilIsDatatype(field) {
				break
			}
		}
		if i == c.arity {
			if err := utility.traceDatatype(953, func(w *datatypeTraceWriter) error {
				w.text("found non-rec ")
				if err := w.ast(astNodeRef{slot: h.slot, generation: h.generation}); err != nil {
					return err
				}
				return w.end("\n")
			}); err != nil {
				return astNodeRef{}, false, err
			}
			return astNodeRef{slot: h.slot, generation: h.generation}, true, nil
		}
	}
	for j := uint32(0); j < n; j++ {
		h := constructors[(j+start)%n]
		if err := utility.traceDatatype(960, func(w *datatypeTraceWriter) error {
			w.text("non_rec_constructor c: ")
			w.text(strconv.FormatUint(uint64(j), 10))
			w.text(" ")
			if err := w.ast(astNodeRef{slot: h.slot, generation: h.generation}); err != nil {
				return err
			}
			return w.end("\n")
		}); err != nil {
			return astNodeRef{}, false, err
		}
		c := *m.declAt(h.slot)
		i := uint32(0)
		for ; i < c.arity; i++ {
			field, err := m.readInternal(c.domain, i)
			if err != nil {
				return astNodeRef{}, false, err
			}
			if err := utility.traceDatatype(965, func(w *datatypeTraceWriter) error {
				w.text("c: ")
				w.text(strconv.FormatUint(uint64(i), 10))
				w.text(" ")
				if err := w.ast(field); err != nil {
					return err
				}
				return w.end("\n")
			}); err != nil {
				return astNodeRef{}, false, err
			}
			if !m.datatypeUtilIsDatatype(field) {
				if err := utility.traceDatatype(967, func(w *datatypeTraceWriter) error {
					if err := w.ast(field); err != nil {
						return err
					}
					return w.end(" is not a datatype\n")
				}); err != nil {
					return astNodeRef{}, false, err
				}
				continue
			}
			if datatypeForbiddenContains(forbidden, field) {
				if err := utility.traceDatatype(971, func(w *datatypeTraceWriter) error {
					if err := w.ast(field); err != nil {
						return err
					}
					return w.end(" is in forbidden_set\n")
				}); err != nil {
					return astNodeRef{}, false, err
				}
				break
			}
			forbidden = append(forbidden, field)
			nested, found, err := m.datatypeNonRecursiveConstructorCore(field, forbidden, counter, utility)
			if err == nil {
				err = datatypeUtilAssert(976, "forbidden_set.back() == T_i", func() bool { return forbidden[len(forbidden)-1] == field })
			}
			forbidden = forbidden[:len(forbidden)-1]
			if err != nil {
				return astNodeRef{}, false, err
			}
			if !found {
				break
			}
			if err := utility.traceDatatype(980, func(w *datatypeTraceWriter) error {
				w.text("nested_c: ")
				name, err := m.Symbols.Display(m.declAt(nested.slot).name)
				if err != nil {
					return err
				}
				w.text(name)
				return w.end("\n")
			}); err != nil {
				return astNodeRef{}, false, err
			}
		}
		if i == c.arity {
			return astNodeRef{slot: h.slot, generation: h.generation}, true, nil
		}
	}
	return astNodeRef{}, false, nil
}

func (m *ASTManager) DatatypeNonRecursiveConstructor(sort ASTHandle) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, false, err
	}
	constructor, found, err := m.datatypeNonRecursiveConstructor(r)
	return m.external(constructor), found, err
}
