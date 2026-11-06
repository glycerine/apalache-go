package params

import (
	"bytes"
	"strings"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Mutable normalization scratch is ordinary Go storage retained for reuse.

func (g *GlobalParams) internWorkspace(b []byte) (usymbol.Symbol, error) {
	return g.SymbolsField.Intern(string(b))
}
func (g *GlobalParams) normalizeWorkspace() (usymbol.Symbol, usymbol.Symbol, error) {
	n := uint32(len(g.Workspace)) // source unsigned narrowing, before both loops
	for i := uint32(0); i < n; i++ {
		ch := g.Workspace[i]
		if ch >= 'A' && ch <= 'Z' {
			g.Workspace[i] = ch - 'A' + 'a'
		} else if ch == '-' {
			g.Workspace[i] = '_'
		}
	}
	for i := uint32(0); i < n; i++ {
		if g.Workspace[i] == '.' {
			// The source publishes suffix PARAM before prefix MODULE.
			param, e := g.internWorkspace(g.Workspace[i+1:])
			if e != nil {
				return usymbol.Symbol{}, usymbol.Symbol{}, e
			}
			module, e := g.internWorkspace(g.Workspace[:i])
			return module, param, e
		}
	}
	param, e := g.internWorkspace(g.Workspace)
	return usymbol.Symbol{}, param, e
}
func (g *GlobalParams) normalizeName(name string) (usymbol.Symbol, usymbol.Symbol, error) {
	name = name[:utext.CStringLength(name)]
	if len(name) > 0 && name[0] == ':' {
		name = name[1:]
	}
	g.Workspace = append(g.Workspace[:0], name...)
	return g.normalizeWorkspace()
}
func (g *GlobalParams) normalizeSymbol(name usymbol.Symbol) (usymbol.Symbol, usymbol.Symbol, error) {
	m := g.SymbolsField
	if e := m.Validate(name); e != nil {
		return usymbol.Symbol{}, usymbol.Symbol{}, e
	}
	if name.IsNumerical() {
		if GlobalParamsSourceDebug {
			return usymbol.Symbol{}, usymbol.Symbol{}, ErrGlobalParamsOwner
		}
		return g.normalizeName("")
	}
	if name.Value == 0 {
		return usymbol.Symbol{}, usymbol.Symbol{}, ErrGlobalParamsOwner
	}
	g.Workspace = g.Workspace[:0]
	text, _, e := m.BareString(name)
	if e != nil {
		return usymbol.Symbol{}, usymbol.Symbol{}, e
	}
	first := true
	for i := 0; i < len(text); i++ {
		b := text[i]
		if first && b == ':' {
			first = false
			continue
		}
		first = false
		g.Workspace = append(g.Workspace, b)
	}
	return g.normalizeWorkspace()
}

type gparamsErrorPart struct {
	text     string
	symbol   usymbol.Symbol
	symbolic bool
}

func gpText(s string) gparamsErrorPart           { return gparamsErrorPart{text: s} }
func gpSymbol(s usymbol.Symbol) gparamsErrorPart { return gparamsErrorPart{symbol: s, symbolic: true} }

func (g *GlobalParams) exception(parts ...gparamsErrorPart) error {
	var out strings.Builder
	for _, p := range parts {
		if p.symbolic {
			text, e := g.SymbolsField.Display(p.symbol)
			if e != nil {
				return e
			}
			out.WriteString(text)
		} else {
			out.WriteString(p.text)
		}
	}
	return z3exception.NewDefaultException(out.String())
}
func (g *GlobalParams) oldName(k usymbol.Symbol) (bool, error) {
	for _, name := range gparamsOldNames {
		same, e := g.SymbolsField.EqualString(k, &name)
		if e != nil {
			return false, e
		}
		if same {
			return true, nil
		}
	}
	return false, nil
}
func (g *GlobalParams) renamedName(k usymbol.Symbol) (string, bool, error) {
	for _, pair := range gparamsRenames {
		same, e := g.SymbolsField.EqualString(k, &pair.old)
		if e != nil {
			return "", false, e
		}
		if same {
			return pair.new, true, nil
		}
	}
	return "", false, nil
}
func (g *GlobalParams) unknownParameter(k usymbol.Symbol, d *ParamDescrs, module usymbol.Symbol) error {
	if module == (usymbol.Symbol{}) {
		newName, found, e := g.renamedName(k)
		if e != nil {
			return e
		}
		if found {
			return g.exception(gpText("the parameter '"), gpSymbol(k), gpText("', invoke 'z3 -p' to obtain the new parameter list, and 'z3 -pp:"), gpText(newName), gpText("' for the full description of the parameter"))
		}
		old, e := g.oldName(k)
		if e != nil {
			return e
		}
		if old {
			return g.exception(gpText("unknown parameter '"), gpSymbol(k), gpText("', this is an old parameter name, invoke 'z3 -p' to obtain the new parameter list"))
		}
	}
	keyText, e := g.SymbolsField.Display(k)
	if e != nil {
		return e
	}
	var out bytes.Buffer
	out.WriteString("unknown parameter '")
	out.WriteString(keyText)
	if module == (usymbol.Symbol{}) {
		out.WriteString("'\n")
	} else {
		moduleText, e := g.SymbolsField.Display(module)
		if e != nil {
			return e
		}
		out.WriteString("' at module '")
		out.WriteString(moduleText)
		out.WriteString("'\n")
	}
	out.WriteString("Legal parameters are:\n")
	if e := d.DisplayInto(&out, 2, false, false); e != nil {
		return e
	}
	return z3exception.NewDefaultException(out.String())
}
func (g *GlobalParams) findModuleDescriptor(k usymbol.Symbol) (*ParamDescrs, bool, error) {
	if e := g.ensureRegistered(); e != nil {
		return nil, false, e
	}
	if _, e := g.SymbolsField.Hash(k); e != nil {
		return nil, false, e
	}
	n, ok := g.ModuleDescriptors.get(k)
	if !ok {
		return nil, false, nil
	}
	if n == 0 || int(n) > len(g.Descriptors) || g.Descriptors[n-1] == nil {
		return nil, false, ErrGlobalParamsOwner
	}
	return g.Descriptors[n-1], true, nil
}
func (g *GlobalParams) validateType(k usymbol.Symbol, value string, d *ParamDescrs) error {
	kind, e := d.Kind(k)
	if e != nil {
		return e
	}
	switch kind {
	case ParamUInt:
		for i := 0; i < len(value); i++ {
			if value[i] < '0' || value[i] > '9' {
				return g.exception(gpText("Expected values for parameter "), gpSymbol(k), gpText(" is an unsigned integer. It was given argument '"), gpText(value), gpText("'"))
			}
		}
	case ParamDouble:
		for i := 0; i < len(value); i++ {
			b := value[i]
			if (b < '0' || b > '9') && b != '.' && b != '-' && b != '/' {
				return g.exception(gpText("Expected values for parameter "), gpSymbol(k), gpText(" is a double. It was given argument '"), gpText(value), gpText("'"))
			}
		}
	case ParamBool:
		if value != "true" && value != "false" {
			return g.exception(gpText("Expected values for parameter "), gpSymbol(k), gpText(" are 'true' or 'false'. It was given argument '"), gpText(value), gpText("'"))
		}
	}
	return nil
}
func (g *GlobalParams) setValue(k usymbol.Symbol, value string, d *ParamDescrs, module usymbol.Symbol) error {
	kind, e := d.Kind(k)
	if e != nil {
		return e
	}
	// Source publication intentionally precedes the invalid-kind diagnostic.
	ps, e := g.GetParams(module)
	if e != nil {
		return e
	}
	switch kind {
	case ParamInvalid:
		return g.unknownParameter(k, d, module)
	case ParamUInt:
		return g.ContextField.SetUInt(ps, k, gparamsParseUint(value, g.cLongBits))
	case ParamDouble:
		return g.ContextField.SetDouble(ps, k, gparamsParseDouble(value))
	case ParamBool:
		if value == "true" {
			return g.ContextField.SetBool(ps, k, true)
		}
		if value == "false" {
			return g.ContextField.SetBool(ps, k, false)
		}
		if module == (usymbol.Symbol{}) {
			return g.exception(gpText("invalid value '"), gpText(value), gpText("' for Boolean parameter '"), gpSymbol(k), gpText("' at module '"), gpSymbol(module), gpText("'"))
		}
		return g.exception(gpText("invalid value '"), gpText(value), gpText("' for Boolean parameter '"), gpSymbol(k), gpText("'"))
	case ParamSymbol:
		s, e := g.SymbolsField.Intern(value)
		if e != nil {
			return e
		}
		return g.ContextField.SetSymbol(ps, k, s)
	case ParamString:
		s, e := g.SymbolsField.Intern(value)
		if e != nil {
			return e
		}
		return g.ContextField.SetStringSymbol(ps, k, s)
	default:
		if module == (usymbol.Symbol{}) {
			return g.exception(gpText("unsupported parameter type '"), gpSymbol(k), gpText("' at module '"), gpSymbol(module), gpText("'"))
		}
		return g.exception(gpText("unsupported parameter type '"), gpSymbol(k), gpText("'"))
	}
}
func (g *GlobalParams) setNormalized(module, k usymbol.Symbol, value string) error {
	var d *ParamDescrs
	if module == (usymbol.Symbol{}) {
		if e := g.ensureRegistered(); e != nil {
			return e
		}
		d = g.GlobalDescriptors
	} else {
		found, ok, e := g.findModuleDescriptor(module)
		if e != nil {
			return e
		}
		if !ok {
			return g.exception(gpText("invalid parameter, unknown module '"), gpSymbol(module), gpText("'"))
		}
		d = found
	}
	value = value[:utext.CStringLength(value)]
	if e := g.validateType(k, value, d); e != nil {
		return e
	}
	return g.setValue(k, value, d, module)
}
func (g *GlobalParams) SetName(name, value string) error {
	if g == nil {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return e
	}
	module, k, e := g.normalizeName(name)
	if e != nil {
		return e
	}
	return g.setNormalized(module, k, value)
}
func (g *GlobalParams) SetSymbol(name usymbol.Symbol, value string) error {
	if g == nil {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return e
	}
	module, k, e := g.normalizeSymbol(name)
	if e != nil {
		return e
	}
	return g.setNormalized(module, k, value)
}
func (g *GlobalParams) getDefault(d *ParamDescrs, k, module usymbol.Symbol) (string, error) {
	yes, e := d.Contains(k)
	if e != nil {
		return "", e
	}
	if !yes {
		return "", g.unknownParameter(k, d, module)
	}
	s, present, e := d.Default(k)
	if e != nil {
		return "", e
	}
	if !present {
		return "default", nil
	}
	return s, nil
}
func (g *GlobalParams) getValue(module, k usymbol.Symbol) (string, error) {
	if module == (usymbol.Symbol{}) {
		yes, e := g.ContextField.Contains(&g.globalParams, k)
		if e != nil {
			return "", e
		}
		if yes {
			return g.ContextField.DisplayKey(&g.globalParams, k)
		}
		if e = g.ensureRegistered(); e != nil {
			return "", e
		}
		return g.getDefault(g.GlobalDescriptors, k, module)
	}
	if _, e := g.SymbolsField.Hash(module); e != nil {
		return "", e
	}
	if n, ok := g.ModuleParams.get(module); ok {
		if n == 0 || int(n) > len(g.Cells) || g.Cells[n-1] == nil {
			return "", ErrGlobalParamsOwner
		}
		ps := &g.Cells[n-1].value
		yes, e := g.ContextField.Contains(ps, k)
		if e != nil {
			return "", e
		}
		if yes {
			return g.ContextField.DisplayKey(ps, k)
		}
	}
	d, yes, e := g.findModuleDescriptor(module)
	if e != nil {
		return "", e
	}
	if !yes {
		return "", g.exception(gpText("unknown module '"), gpSymbol(module), gpText("'"))
	}
	return g.getDefault(d, k, module)
}
func (g *GlobalParams) GetValueName(name string) (string, error) {
	if g == nil {
		return "", ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return "", e
	}
	module, k, e := g.normalizeName(name)
	if e != nil {
		return "", e
	}
	return g.getValue(module, k)
}
func (g *GlobalParams) GetValueSymbol(name usymbol.Symbol) (string, error) {
	if g == nil {
		return "", ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return "", e
	}
	module, k, e := g.normalizeSymbol(name)
	if e != nil {
		return "", e
	}
	return g.getValue(module, k)
}

// GetRef is the actual borrowed const-like alias, not a snapshot. Callers must
// neither Del nor mutate this Ref, and the registry must outlive its use.
func (g *GlobalParams) GetRef() (*ParamsRef, error) {
	if g == nil {
		return nil, ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return nil, e
	}
	return &g.globalParams, nil
}
