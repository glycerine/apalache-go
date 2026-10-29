package ast

// Source: src/ast/datatype_decl_plugin.cpp:170-191; translation methods at
// lines 54-112.

import (
	"sort"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astDatatypeTranslationAccessor struct {
	name      usymbol.Symbol
	rangeSort ASTHandle
	sibling   usymbol.Symbol
}

type astDatatypeTranslationConstructor struct {
	name, recognizer usymbol.Symbol
	accessors        []astDatatypeTranslationAccessor
}

type astDatatypeTranslationDefinition struct {
	name         usymbol.Symbol
	parameters   []ASTHandle
	constructors []astDatatypeTranslationConstructor
}

type astDatatypeTranslationGroup struct {
	classID     uint32
	definitions []astDatatypeTranslationDefinition
}

func (t *ASTTranslation) snapshotDatatypeDefinitions() ([]astDatatypeTranslationGroup, error) {
	m := t.from
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	p := m.datatypePlugin
	if p == nil || len(p.definitionNames) == 0 {
		return nil, nil
	}
	byClass := make(map[uint32][]*astDatatypeDefinition)
	for _, definition := range p.definitionNames {
		byClass[definition.classID] = append(byClass[definition.classID], definition)
	}
	classes := make([]uint32, 0, len(byClass))
	for classID := range byClass {
		classes = append(classes, classID)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	groups := make([]astDatatypeTranslationGroup, 0, len(classes))
	for _, classID := range classes {
		definitions := byClass[classID]
		sort.Slice(definitions, func(i, j int) bool {
			left, _ := m.Symbols.Display(definitions[i].name)
			right, _ := m.Symbols.Display(definitions[j].name)
			return left < right
		})
		group := astDatatypeTranslationGroup{classID: classID}
		for _, definition := range definitions {
			copyDefinition := astDatatypeTranslationDefinition{name: definition.name}
			for _, parameter := range definition.parameters {
				copyDefinition.parameters = append(copyDefinition.parameters, m.external(parameter))
			}
			for _, constructor := range definition.constructors {
				copyConstructor := astDatatypeTranslationConstructor{name: constructor.name, recognizer: constructor.recognizer}
				for i, field := range constructor.ranges {
					copyAccessor := astDatatypeTranslationAccessor{name: constructor.accessors[i], rangeSort: m.external(field)}
					if sibling := p.definitions[field]; sibling != nil && sibling.classID == classID {
						copyAccessor.sibling = sibling.name
					}
					copyConstructor.accessors = append(copyConstructor.accessors, copyAccessor)
				}
				copyDefinition.constructors = append(copyDefinition.constructors, copyConstructor)
			}
			group.definitions = append(group.definitions, copyDefinition)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (t *ASTTranslation) copyDatatypeDefinitions() error {
	groups, err := t.snapshotDatatypeDefinitions()
	if err != nil {
		return err
	}
	for _, group := range groups {
		indices := make(map[usymbol.Symbol]uint32, len(group.definitions))
		for i := range group.definitions {
			indices[group.definitions[i].name] = uint32(i)
		}
		definitions := make([]DatatypeDefinition, len(group.definitions))
		for i, source := range group.definitions {
			name, e := t.translateSymbol(source.name)
			if e != nil {
				return e
			}
			definition := DatatypeDefinition{Name: name}
			for _, parameter := range source.parameters {
				translated, e := t.Translate(parameter)
				if e != nil {
					return e
				}
				definition.Parameters = append(definition.Parameters, translated)
			}
			for _, sourceConstructor := range source.constructors {
				constructorName, e := t.translateSymbol(sourceConstructor.name)
				if e != nil {
					return e
				}
				recognizer, e := t.translateSymbol(sourceConstructor.recognizer)
				if e != nil {
					return e
				}
				constructor := DatatypeConstructorDefinition{Name: constructorName, Recognizer: recognizer}
				for _, sourceAccessor := range sourceConstructor.accessors {
					accessorName, e := t.translateSymbol(sourceAccessor.name)
					if e != nil {
						return e
					}
					accessor := DatatypeAccessorDefinition{Name: accessorName}
					if !sourceAccessor.sibling.IsNull() {
						accessor.Recursive = true
						accessor.RecursiveIndex = indices[sourceAccessor.sibling]
					} else {
						accessor.Range, e = t.Translate(sourceAccessor.rangeSort)
						if e != nil {
							return e
						}
					}
					constructor.Accessors = append(constructor.Accessors, accessor)
				}
				definition.Constructors = append(definition.Constructors, constructor)
			}
			definitions[i] = definition
		}
		if _, err = t.to.MkDatatypes(definitions); err != nil {
			return err
		}
	}
	return nil
}
