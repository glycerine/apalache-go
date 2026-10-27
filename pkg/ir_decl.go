package apalache

// Decl is a TLA+ constant, variable, assumption, operator, or theorem
// declaration. As in the Scala IR, IDs and tags are not part of structural
// equality.
type Decl interface {
	ID() UID
	Tag() TypeTag
	Name() string
	WithTag(TypeTag) Decl
	declNode()
}

type declIdentity struct {
	id  UID
	tag TypeTag
}

func newDeclIdentity(tag TypeTag) declIdentity { return declIdentity{id: NewUID(), tag: tag} }
func (identity declIdentity) ID() UID          { return identity.id }
func (identity declIdentity) Tag() TypeTag     { return identity.tag }

type ConstDecl struct {
	declIdentity
	name string
}

func (*ConstDecl) declNode() {}
func NewConstDecl(name string, tag TypeTag) *ConstDecl {
	return &ConstDecl{declIdentity: newDeclIdentity(tag), name: name}
}
func (decl *ConstDecl) Name() string { return decl.name }
func (decl *ConstDecl) WithTag(tag TypeTag) Decl {
	return NewConstDecl(decl.name, tag)
}

type VarDecl struct {
	declIdentity
	name     string
	internal bool
}

func (*VarDecl) declNode() {}
func NewVarDecl(name string, tag TypeTag) *VarDecl {
	return &VarDecl{declIdentity: newDeclIdentity(tag), name: name}
}
func (decl *VarDecl) Name() string     { return decl.name }
func (decl *VarDecl) IsInternal() bool { return decl.internal }
func (decl *VarDecl) MarkInternal()    { decl.internal = true }
func (decl *VarDecl) WithTag(tag TypeTag) Decl {
	result := NewVarDecl(decl.name, tag)
	result.internal = decl.internal
	return result
}

type AssumeDecl struct {
	declIdentity
	definedName *string
	body        Expr
}

func (*AssumeDecl) declNode() {}
func NewAssumeDecl(definedName *string, body Expr, tag TypeTag) *AssumeDecl {
	if body == nil {
		panic("NewAssumeDecl requires a non-nil body")
	}
	var copiedName *string
	if definedName != nil {
		name := *definedName
		copiedName = &name
	}
	return &AssumeDecl{declIdentity: newDeclIdentity(tag), definedName: copiedName, body: body}
}
func NewNamedAssumeDecl(name string, body Expr, tag TypeTag) *AssumeDecl {
	return NewAssumeDecl(&name, body, tag)
}
func NewUnnamedAssumeDecl(body Expr, tag TypeTag) *AssumeDecl {
	return NewAssumeDecl(nil, body, tag)
}
func (decl *AssumeDecl) Name() string {
	if decl.definedName != nil {
		return *decl.definedName
	}
	return "ASSUME" + decl.body.ID().String()
}
func (decl *AssumeDecl) DefinedName() (string, bool) {
	if decl.definedName == nil {
		return "", false
	}
	return *decl.definedName, true
}
func (decl *AssumeDecl) Body() Expr { return decl.body }
func (decl *AssumeDecl) WithTag(tag TypeTag) Decl {
	return NewAssumeDecl(decl.definedName, decl.body, tag)
}

type OperDecl struct {
	declIdentity
	name         string
	formalParams []OperatorParam
	body         Expr
	recursive    bool
}

func (*OperDecl) declNode() {}
func NewOperDecl(name string, formalParams []OperatorParam, body Expr, tag TypeTag) *OperDecl {
	if body == nil {
		panic("NewOperDecl requires a non-nil body")
	}
	return &OperDecl{
		declIdentity: newDeclIdentity(tag),
		name:         name,
		formalParams: append([]OperatorParam(nil), formalParams...),
		body:         body,
	}
}
func (decl *OperDecl) Name() string { return decl.name }
func (decl *OperDecl) FormalParams() []OperatorParam {
	return append([]OperatorParam(nil), decl.formalParams...)
}
func (decl *OperDecl) Body() Expr               { return decl.body }
func (decl *OperDecl) SetBody(body Expr)        { requireExpr(body); decl.body = body }
func (decl *OperDecl) IsRecursive() bool        { return decl.recursive }
func (decl *OperDecl) SetRecursive(value bool)  { decl.recursive = value }
func (decl *OperDecl) WithTag(tag TypeTag) Decl { return decl.copyWithTag(tag) }
func (decl *OperDecl) copyWithTag(tag TypeTag) *OperDecl {
	result := NewOperDecl(decl.name, decl.formalParams, decl.body, tag)
	result.recursive = decl.recursive
	return result
}

type TheoremDecl struct {
	declIdentity
	name string
	body Expr
}

func (*TheoremDecl) declNode() {}
func NewTheoremDecl(name string, body Expr, tag TypeTag) *TheoremDecl {
	requireExpr(body)
	return &TheoremDecl{declIdentity: newDeclIdentity(tag), name: name, body: body}
}
func (decl *TheoremDecl) Name() string { return decl.name }
func (decl *TheoremDecl) Body() Expr   { return decl.body }
func (decl *TheoremDecl) WithTag(tag TypeTag) Decl {
	return NewTheoremDecl(decl.name, decl.body, tag)
}

func requireExpr(expr Expr) {
	if expr == nil {
		panic("declaration requires a non-nil body")
	}
}

func EqualDecl(left, right Decl) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	switch l := left.(type) {
	case *ConstDecl:
		r, ok := right.(*ConstDecl)
		return ok && l.name == r.name
	case *VarDecl:
		r, ok := right.(*VarDecl)
		return ok && l.name == r.name
	case *AssumeDecl:
		r, ok := right.(*AssumeDecl)
		return ok && optionalStringEqual(l.definedName, r.definedName) && EqualExpr(l.body, r.body)
	case *OperDecl:
		r, ok := right.(*OperDecl)
		if !ok || l.name != r.name || len(l.formalParams) != len(r.formalParams) || !EqualExpr(l.body, r.body) {
			return false
		}
		for index := range l.formalParams {
			if l.formalParams[index] != r.formalParams[index] {
				return false
			}
		}
		return true
	case *TheoremDecl:
		r, ok := right.(*TheoremDecl)
		return ok && l.name == r.name && EqualExpr(l.body, r.body)
	default:
		return false
	}
}

func EqualTypedDecl(left, right Decl) bool {
	return EqualDecl(left, right) && left != nil && right != nil && left.Tag().Equal(right.Tag())
}

func optionalStringEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

type ModuleProperty uint8

const (
	PropertyTemporalEncoded ModuleProperty = iota
	PropertyDesugared
	PropertyTypeChecked
	PropertyConfigured
	PropertyInlined
	PropertyPrimed
	PropertyVCGenerated
	PropertyTransitionsFound
	PropertyPreprocessed
	PropertyOptimized
	PropertyAnalyzed
)

type Module struct {
	name         string
	declarations []Decl
	properties   map[ModuleProperty]struct{}
}

func NewModule(name string, declarations []Decl) *Module {
	for _, decl := range declarations {
		if decl == nil {
			panic("NewModule requires non-nil declarations")
		}
	}
	return &Module{name: name, declarations: append([]Decl(nil), declarations...), properties: make(map[ModuleProperty]struct{})}
}
func (module *Module) Name() string { return module.name }
func (module *Module) Declarations() []Decl {
	return append([]Decl(nil), module.declarations...)
}
func (module *Module) ConstDeclarations() []*ConstDecl {
	result := make([]*ConstDecl, 0)
	for _, decl := range module.declarations {
		if typed, ok := decl.(*ConstDecl); ok {
			result = append(result, typed)
		}
	}
	return result
}
func (module *Module) VarDeclarations() []*VarDecl {
	result := make([]*VarDecl, 0)
	for _, decl := range module.declarations {
		if typed, ok := decl.(*VarDecl); ok {
			result = append(result, typed)
		}
	}
	return result
}
func (module *Module) OperDeclarations() []*OperDecl {
	result := make([]*OperDecl, 0)
	for _, decl := range module.declarations {
		if typed, ok := decl.(*OperDecl); ok {
			result = append(result, typed)
		}
	}
	return result
}
func (module *Module) AssumeDeclarations() []*AssumeDecl {
	result := make([]*AssumeDecl, 0)
	for _, decl := range module.declarations {
		if typed, ok := decl.(*AssumeDecl); ok {
			result = append(result, typed)
		}
	}
	return result
}
func (module *Module) AddProperty(property ModuleProperty) {
	module.properties[property] = struct{}{}
}
func (module *Module) HasProperty(property ModuleProperty) bool {
	_, ok := module.properties[property]
	return ok
}
