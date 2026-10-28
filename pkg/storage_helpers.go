package apalache

import (
	"log"
	"sort"
)

// BodyMap is the immutable sorted-map abstraction used for operator bodies.
// Updates return a fresh map, and Keys/Declarations expose lexical key order.
type BodyMap struct{ entries map[string]*OperDecl }

func NewBodyMap() BodyMap { return BodyMap{entries: make(map[string]*OperDecl)} }

func (bodyMap BodyMap) Contains(name string) bool {
	_, ok := bodyMap.entries[name]
	return ok
}
func (bodyMap BodyMap) Get(name string) (*OperDecl, bool) {
	declaration, ok := bodyMap.entries[name]
	return declaration, ok
}
func (bodyMap BodyMap) Keys() []string {
	keys := make([]string, 0, len(bodyMap.entries))
	for key := range bodyMap.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func (bodyMap BodyMap) Declarations() []*OperDecl {
	keys := bodyMap.Keys()
	result := make([]*OperDecl, len(keys))
	for index, key := range keys {
		result[index] = bodyMap.entries[key]
	}
	return result
}
func (bodyMap BodyMap) Len() int { return len(bodyMap.entries) }

func BodyMapFromDeclaration(declaration Decl, initial BodyMap) BodyMap {
	if initial.entries == nil {
		initial = NewBodyMap()
	}
	operator, ok := declaration.(*OperDecl)
	if !ok || operator == nil || initial.Contains(operator.Name()) {
		return initial
	}
	entries := make(map[string]*OperDecl, len(initial.entries)+1)
	for name, existing := range initial.entries {
		entries[name] = existing
	}
	entries[operator.Name()] = operator
	return BodyMap{entries: entries}
}

func BodyMapFromDeclarations(declarations []Decl, initial BodyMap) BodyMap {
	result := initial
	for _, declaration := range declarations {
		result = BodyMapFromDeclaration(declaration, result)
	}
	return result
}

// SourceLocator resolves transformed node IDs through ChangeListener back to
// locations in the original source map.
type SourceLocator struct {
	sourceMap SourceMap
	changes   *ChangeListener
}

func NewSourceLocator(sourceMap SourceMap, changes *ChangeListener) *SourceLocator {
	copyMap := make(SourceMap, len(sourceMap))
	for id, location := range sourceMap {
		copyMap[id] = location
	}
	if changes == nil {
		changes = NewChangeListener()
	}
	return &SourceLocator{sourceMap: copyMap, changes: changes}
}

func (locator *SourceLocator) SourceOfID(id UID) (SourceLocation, bool) {
	if locator == nil {
		return SourceLocation{}, false
	}
	location, ok := locator.sourceMap[locator.changes.TraceBack(id)]
	return location, ok
}
func (locator *SourceLocator) SourceOfExpression(expression Expr) (SourceLocation, bool) {
	if expression == nil {
		return SourceLocation{}, false
	}
	return locator.SourceOfID(expression.ID())
}

// MissingSourceIDs returns topmost missing expressions exactly as Scala's
// checkConsistency logs them: descendants of a missing node are not visited.
func (locator *SourceLocator) MissingSourceIDs(expression Expr) []UID {
	if expression == nil {
		return nil
	}
	if _, found := locator.SourceOfExpression(expression); !found {
		return []UID{expression.ID()}
	}
	result := make([]UID, 0)
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			result = append(result, locator.MissingSourceIDs(argument)...)
		}
	case *LetInExpr:
		result = append(result, locator.MissingSourceIDs(typed.Body())...)
		for _, declaration := range typed.Declarations() {
			result = append(result, locator.MissingSourceIDs(declaration.Body())...)
		}
	}
	return result
}

func (locator *SourceLocator) CheckConsistencyExpression(expression Expr) {
	for _, id := range locator.MissingSourceIDs(expression) {
		log.Printf("No source location for expr@%s", id)
	}
}
func (locator *SourceLocator) CheckConsistencyDeclaration(declaration *OperDecl) {
	if declaration != nil {
		locator.CheckConsistencyExpression(declaration.Body())
	}
}
