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
	if listener.sources == nil {
		return Position{}
	}
	location, found := listener.sources.Find(id)
	if !found {
		return Position{}
	}
	return Position{Filename: location.Filename, Line: location.Region.Start.Line, Column: location.Region.Start.Column, EndLine: location.Region.End.Line, EndColumn: location.Region.End.Column}
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

type TypeCheckOptions struct {
	Annotations      map[UID]Type
	Aliases          *TypeAliasSubstitution
	DisablePolytypes bool
	UseRows          bool
	VariableStart    int
	Sources          *SourceStore
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
	sorted, err := SortDeclarations(module)
	if err != nil {
		return TypeCheckResult{Diagnostics: []Diagnostic{{
			Code: "E-TYPE-INPUT", Severity: SeverityError, Message: fmt.Sprintf("cannot order declarations for type checking: %v", err),
		}}}
	}
	module = sorted
	start := options.VariableStart
	if start <= 0 {
		start = 1000
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
	types   map[UID]Type
	tracker TransformationTracker
}

func NewTypeRewriter(types map[UID]Type, sources *SourceStore) *TypeRewriter {
	copy := make(map[UID]Type, len(types))
	for id, typ := range types {
		copy[id] = typ
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	return &TypeRewriter{types: copy, tracker: tracker}
}
func (rewriter *TypeRewriter) tag(id UID, old TypeTag) TypeTag {
	if typ, found := rewriter.types[id]; found {
		return Typed(typ)
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
