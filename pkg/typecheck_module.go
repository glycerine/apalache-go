package apalache

import "fmt"

type RecordingTypeCheckerListener struct {
	sources     *SourceStore
	types       map[UID]Type
	diagnostics []Diagnostic
}

func NewRecordingTypeCheckerListener(sources *SourceStore) *RecordingTypeCheckerListener {
	return &RecordingTypeCheckerListener{sources: sources, types: make(map[UID]Type)}
}
func (listener *RecordingTypeCheckerListener) OnTypeFound(ref ExactRef, typ Type) {
	listener.types[ref.ID()] = typ
}
func (listener *RecordingTypeCheckerListener) OnTypeError(ref EtcRef, message string) {
	listener.diagnostics = append(listener.diagnostics, Diagnostic{Code: "E-TYPE", Severity: SeverityError, Message: message, Position: listener.position(ref.ID())})
}
func (listener *RecordingTypeCheckerListener) OnTypeWarning(ref EtcRef, message string) {
	listener.diagnostics = append(listener.diagnostics, Diagnostic{Code: "W-TYPE", Severity: SeverityWarning, Message: message, Position: listener.position(ref.ID())})
}
func (listener *RecordingTypeCheckerListener) position(id UID) Position {
	return positionForID(listener.sources, id)
}
func (listener *RecordingTypeCheckerListener) Types() map[UID]Type {
	result := make(map[UID]Type, len(listener.types))
	for id, typ := range listener.types {
		result[id] = typ
	}
	return result
}
func (listener *RecordingTypeCheckerListener) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), listener.diagnostics...)
}
func (listener *RecordingTypeCheckerListener) Errors() []Diagnostic {
	return listener.diagnosticsWithSeverity(SeverityError)
}
func (listener *RecordingTypeCheckerListener) Warnings() []Diagnostic {
	return listener.diagnosticsWithSeverity(SeverityWarning)
}
func (listener *RecordingTypeCheckerListener) diagnosticsWithSeverity(severity Severity) []Diagnostic {
	result := make([]Diagnostic, 0)
	for _, diagnostic := range listener.diagnostics {
		if diagnostic.Severity == severity {
			result = append(result, diagnostic)
		}
	}
	return result
}

type TypeCheckOptions struct {
	Annotations      map[UID]Type
	Aliases          *TypeAliasSubstitution
	DisablePolytypes bool
	UseRows          bool
	// NoRows records an explicit legacy-mode request. When both UseRows and
	// NoRows are false, annotations from either type-system generation are
	// accepted for compatibility with programmatic callers.
	NoRows        bool
	VariableStart int
	Sources       *SourceStore
}

type TypeCheckResult struct {
	Module      *Module
	Types       map[UID]Type
	Diagnostics []Diagnostic
}

func (result TypeCheckResult) OK() bool {
	return result.Module != nil && !hasErrors(result.Diagnostics)
}

// TypeCheckModule translates the complete module to ETC, infers types, and
// returns a freshly allocated typed module. Input nodes are never mutated.
func TypeCheckModule(module *Module, options TypeCheckOptions) TypeCheckResult {
	if module == nil {
		return TypeCheckResult{Diagnostics: []Diagnostic{{
			Code: "E-TYPE", Severity: SeverityError, Message: "cannot type-check a nil module",
		}}}
	}
	if options.UseRows && options.NoRows {
		return TypeCheckResult{Diagnostics: []Diagnostic{{
			Code: "E-TYPE-INPUT", Severity: SeverityError, Message: "row types and no-rows mode cannot both be enabled",
		}}}
	}
	if id, typ, invalid := incompatibleAnnotationType(options); invalid {
		message := "Found row types, while Type System 1.2 is disabled. Do not use --features=no-rows."
		if options.UseRows {
			message = "Found imprecise record types, while Type System 1.2 is enabled. Either update the annotations, or use --features=no-rows."
		}
		return TypeCheckResult{Diagnostics: []Diagnostic{{
			Code: "E-TYPE-INPUT", Severity: SeverityError,
			Message: message, Position: positionForID(options.Sources, id),
		}}, Types: map[UID]Type{id: typ}}
	}
	sorted, err := SortDeclarations(module)
	if err != nil {
		return TypeCheckResult{Diagnostics: []Diagnostic{{
			Code: "E-TYPE-INPUT", Severity: SeverityError, Message: fmt.Sprintf("cannot order declarations for type checking: %v", err),
		}}}
	}
	module = sorted
	start := options.VariableStart
	if start <= 0 {
		start = maxUsedTypeVariable(module) + 1
	}
	pool := NewTypeVarPool(start)
	translator := NewToEtcExpr(options.Annotations, options.Aliases, pool, options.UseRows)
	var expression EtcExpr = NewEtcConst(BlameRef{SourceID: NewUID()}, BoolT1)
	declarations := module.Declarations()
	for i := len(declarations) - 1; i >= 0; i-- {
		var err error
		expression, err = translator.TranslateDeclaration(declarations[i], expression)
		if err != nil {
			return TypeCheckResult{Diagnostics: []Diagnostic{{
				Code: "E-TYPE-INPUT", Severity: SeverityError, Message: err.Error(),
				Position: positionForID(options.Sources, declarations[i].ID()),
			}}}
		}
	}
	// Declaration nodes deliberately use ExactRef so their inferred types can
	// be recorded. Do not also use the outermost declaration as the reference
	// for the whole scoped program: its result is the terminal Bool expression,
	// which would overwrite the declaration's own type in the listener.
	expression = NewEtcApp(BlameRef{SourceID: NewUID()}, []Type{mustOperator([]Type{BoolT1}, BoolT1)}, expression)
	listener := NewRecordingTypeCheckerListener(options.Sources)
	checker := NewEtcTypeChecker(pool, !options.DisablePolytypes)
	_, ok, err := checker.Compute(listener, EmptyTypeContext(), expression)
	diagnostics := listener.Diagnostics()
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Code: "E-TYPE-INTERNAL", Severity: SeverityError, Message: err.Error()})
		return TypeCheckResult{Types: listener.Types(), Diagnostics: diagnostics}
	}
	if !ok {
		return TypeCheckResult{Types: listener.Types(), Diagnostics: diagnostics}
	}
	types := listener.Types()
	rewriter := NewTypeRewriter(types, options.Sources)
	typedModule, err := rewriter.Module(module)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Code: "E-TYPE-REWRITE", Severity: SeverityError, Message: err.Error()})
		return TypeCheckResult{Types: types, Diagnostics: diagnostics}
	}
	typedModule.AddProperty(PropertyTypeChecked)
	return TypeCheckResult{Module: typedModule, Types: types, Diagnostics: diagnostics}
}

func incompatibleAnnotationType(options TypeCheckOptions) (UID, Type, bool) {
	for id, typ := range options.Annotations {
		if !annotationTypeMatchesRowsMode(typ, options.UseRows, options.NoRows) {
			return id, typ, true
		}
	}
	if options.Aliases != nil {
		for _, typ := range options.Aliases.Context() {
			if !annotationTypeMatchesRowsMode(typ, options.UseRows, options.NoRows) {
				return NullUID(), typ, true
			}
		}
	}
	return NullUID(), nil, false
}

func annotationTypeMatchesRowsMode(typ Type, useRows, noRows bool) bool {
	if typ == nil {
		return false
	}
	switch typed := typ.(type) {
	case primitiveType, ConstType, VarType:
		return true
	case SparseTupleType:
		for _, field := range typed.Fields() {
			if !annotationTypeMatchesRowsMode(field, useRows, noRows) {
				return false
			}
		}
		return true
	case SetType:
		return annotationTypeMatchesRowsMode(typed.Element(), useRows, noRows)
	case SeqType:
		return annotationTypeMatchesRowsMode(typed.Element(), useRows, noRows)
	case FunType:
		return annotationTypeMatchesRowsMode(typed.Argument(), useRows, noRows) && annotationTypeMatchesRowsMode(typed.Result(), useRows, noRows)
	case TupleType:
		for _, element := range typed.Elements() {
			if !annotationTypeMatchesRowsMode(element, useRows, noRows) {
				return false
			}
		}
		return true
	case RecordType:
		if useRows {
			return false
		}
		for _, field := range typed.Fields() {
			if !annotationTypeMatchesRowsMode(field, useRows, noRows) {
				return false
			}
		}
		return true
	case RowType:
		if noRows {
			return false
		}
		for _, field := range typed.Fields() {
			if !annotationTypeMatchesRowsMode(field, useRows, noRows) {
				return false
			}
		}
		if other, open := typed.Other(); open {
			return annotationTypeMatchesRowsMode(other, useRows, noRows)
		}
		return true
	case RecordRowType:
		return !noRows && annotationTypeMatchesRowsMode(typed.Row(), useRows, noRows)
	case VariantType:
		return !noRows && annotationTypeMatchesRowsMode(typed.Row(), useRows, noRows)
	case OperatorType:
		for _, argument := range typed.Arguments() {
			if !annotationTypeMatchesRowsMode(argument, useRows, noRows) {
				return false
			}
		}
		return annotationTypeMatchesRowsMode(typed.Result(), useRows, noRows)
	default:
		return false
	}
}

func maxUsedTypeVariable(module *Module) int {
	maximum := -1
	visitTag := func(tag TypeTag) {
		if typ, typed := tag.Type(); typed {
			for _, variable := range typ.UsedNames() {
				if variable > maximum {
					maximum = variable
				}
			}
		}
	}
	var visitExpression func(Expr)
	var visitDeclaration func(Decl)
	visitExpression = func(expression Expr) {
		switch typed := expression.(type) {
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visitExpression(argument)
			}
		case *LetInExpr:
			visitExpression(typed.Body())
			for _, declaration := range typed.Declarations() {
				visitDeclaration(declaration)
			}
		}
	}
	visitDeclaration = func(declaration Decl) {
		visitTag(declaration.Tag())
		if operator, ok := declaration.(*OperDecl); ok {
			visitExpression(operator.Body())
		}
	}
	if module != nil {
		for _, declaration := range module.Declarations() {
			visitDeclaration(declaration)
		}
	}
	return maximum
}

func positionForID(store *SourceStore, id UID) Position {
	if store == nil {
		return Position{}
	}
	location, ok := store.Find(id)
	if !ok {
		return Position{}
	}
	return Position{Filename: location.Filename, Line: location.Region.Start.Line, Column: location.Region.Start.Column, EndLine: location.Region.End.Line, EndColumn: location.Region.End.Column}
}

type TypeRewriter struct {
	types      map[UID]Type
	tracker    TransformationTracker
	defaultTag func(UID) TypeTag
}

func NewTypeRewriter(types map[UID]Type, sources *SourceStore) *TypeRewriter {
	return newTypeRewriter(types, sources, nil)
}

// NewTypeRewriterWithDefault supplies the fallback used when inference did not
// produce a type for a node, matching Scala's type-coverage callback.
func NewTypeRewriterWithDefault(types map[UID]Type, sources *SourceStore, defaultTag func(UID) TypeTag) *TypeRewriter {
	if defaultTag == nil {
		panic("NewTypeRewriterWithDefault requires a fallback")
	}
	return newTypeRewriter(types, sources, defaultTag)
}

func newTypeRewriter(types map[UID]Type, sources *SourceStore, defaultTag func(UID) TypeTag) *TypeRewriter {
	copy := make(map[UID]Type, len(types))
	for id, typ := range types {
		copy[id] = typ
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	return &TypeRewriter{types: copy, tracker: tracker, defaultTag: defaultTag}
}
func (rewriter *TypeRewriter) tag(id UID, old TypeTag) TypeTag {
	if typ, found := rewriter.types[id]; found {
		return Typed(typ)
	}
	if rewriter.defaultTag != nil {
		return rewriter.defaultTag(id)
	}
	return old
}
func (rewriter *TypeRewriter) Expression(expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot rewrite nil expression")
	}
	var result Expr
	switch typed := expression.(type) {
	case *nullExpr:
		return nil, fmt.Errorf("cannot apply TypeRewriter to NullExpr")
	case *ValueExpr:
		tag := rewriter.tag(typed.ID(), typed.Tag())
		if text, ok := typed.Value().(StringValue); ok {
			tag = Typed(ModelValueOrStringType(text.Value()))
		}
		result = NewValueExpr(typed.Value(), tag)
	case *NameExpr:
		result = NewNameExpr(typed.Name(), rewriter.tag(typed.ID(), typed.Tag()))
	case *OperExpr:
		arguments := typed.Arguments()
		for i, argument := range arguments {
			rewritten, err := rewriter.Expression(argument)
			if err != nil {
				return nil, err
			}
			arguments[i] = rewritten
		}
		if typed.Operator().Name() == "EXCEPT" {
			for i := 1; i < len(arguments); i += 2 {
				tuple, ok := arguments[i].(*OperExpr)
				if !ok || tuple.Operator().Name() != "TUPLE" {
					return nil, fmt.Errorf("expected tuple accessor in EXCEPT")
				}
				indices := tuple.Arguments()
				elements := make([]Type, len(indices))
				for j, index := range indices {
					element, ok := index.Tag().Type()
					if !ok {
						return nil, fmt.Errorf("missing inferred type for EXCEPT index")
					}
					elements[j] = element
				}
				arguments[i] = MustOperExpr(tuple.Operator(), Typed(mustTuple(elements...)), indices...)
			}
		}
		result = MustOperExpr(typed.Operator(), rewriter.tag(typed.ID(), typed.Tag()), arguments...)
	case *LetInExpr:
		body, err := rewriter.Expression(typed.Body())
		if err != nil {
			return nil, err
		}
		declarations := typed.Declarations()
		for i, declaration := range declarations {
			rewritten, err := rewriter.Declaration(declaration)
			if err != nil {
				return nil, err
			}
			declarations[i] = rewritten.(*OperDecl)
		}
		result = NewLetInExpr(body, rewriter.tag(typed.ID(), typed.Tag()), declarations...)
	default:
		return nil, fmt.Errorf("unsupported expression %T", expression)
	}
	rewriter.tracker.TrackExpression(func(Expr) Expr { return result })(expression)
	return result, nil
}
func (rewriter *TypeRewriter) Declaration(declaration Decl) (Decl, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot rewrite nil declaration")
	}
	var result Decl
	switch typed := declaration.(type) {
	case *ConstDecl:
		result = NewConstDecl(typed.Name(), rewriter.tag(typed.ID(), typed.Tag()))
	case *VarDecl:
		variable := NewVarDecl(typed.Name(), rewriter.tag(typed.ID(), typed.Tag()))
		if typed.IsInternal() {
			variable.MarkInternal()
		}
		result = variable
	case *AssumeDecl:
		body, err := rewriter.Expression(typed.Body())
		if err != nil {
			return nil, err
		}
		if name, named := typed.DefinedName(); named {
			result = NewNamedAssumeDecl(name, body, rewriter.tag(typed.ID(), typed.Tag()))
		} else {
			result = NewUnnamedAssumeDecl(body, rewriter.tag(typed.ID(), typed.Tag()))
		}
	case *TheoremDecl:
		body, err := rewriter.Expression(typed.Body())
		if err != nil {
			return nil, err
		}
		result = NewTheoremDecl(typed.Name(), body, rewriter.tag(typed.ID(), typed.Tag()))
	case *OperDecl:
		body, err := rewriter.Expression(typed.Body())
		if err != nil {
			return nil, err
		}
		operator := NewOperDecl(typed.Name(), typed.FormalParams(), body, rewriter.tag(typed.ID(), typed.Tag()))
		operator.SetRecursive(typed.IsRecursive())
		result = operator
	default:
		return nil, fmt.Errorf("unsupported declaration %T", declaration)
	}
	rewriter.tracker.TrackDeclaration(func(Decl) Decl { return result })(declaration)
	return result, nil
}
func (rewriter *TypeRewriter) Module(module *Module) (*Module, error) {
	declarations := module.Declarations()
	for i, declaration := range declarations {
		rewritten, err := rewriter.Declaration(declaration)
		if err != nil {
			return nil, err
		}
		declarations[i] = rewritten
	}
	result := NewModule(module.Name(), declarations)
	for property := PropertyTemporalEncoded; property <= PropertyAnalyzed; property++ {
		if module.HasProperty(property) {
			result.AddProperty(property)
		}
	}
	return result, nil
}
