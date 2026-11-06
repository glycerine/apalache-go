package api

// Source: src/api/api_params.cpp:29-208; src/api/api_util.h:88-106.
//
// ParameterSet and ParameterDescriptions are the idiomatic Go ownership bridge
// for the source opaque reference-counted handles. They borrow the live
// SMTContext parameter service but own their ParamsRef/ParamDescrs payloads;
// Close is the source final dec_ref.

import (
	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ParameterKind uint32

const (
	ParameterUInt ParameterKind = iota
	ParameterBool
	ParameterDouble
	ParameterString
	ParameterSymbol
	ParameterInvalid
	ParameterOther
)

type ParameterSet struct {
	owner  *smtContextState
	params *uparams.ParamsContext
	value  uparams.ParamsRef
	closed bool
}

func (c *SMTContext) NewParameterSet() (*ParameterSet, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	value, err := globals.Context.NewValue()
	if err != nil {
		return nil, err
	}
	return &ParameterSet{owner: c.smtContextState, params: globals.Context, value: value}, nil
}

func (p *ParameterSet) open() error {
	if p == nil || p.owner == nil || p.params == nil || p.closed {
		return ErrSMTContextOwner
	}
	if p.owner.closed || p.owner.command == nil {
		return ErrSMTContextOwner
	}
	return p.params.Check(&p.value)
}

func apiParameterName(name string) string { return uparams.NormParamName(&name) }

func (p *ParameterSet) SetBool(name string, value bool) error {
	if err := p.open(); err != nil {
		return err
	}
	name = apiParameterName(name)
	return p.params.SetBoolName(&p.value, &name, value)
}

func (p *ParameterSet) SetUInt(name string, value uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	name = apiParameterName(name)
	return p.params.SetUIntName(&p.value, &name, value)
}

func (p *ParameterSet) SetDouble(name string, value float64) error {
	if err := p.open(); err != nil {
		return err
	}
	name = apiParameterName(name)
	return p.params.SetDoubleName(&p.value, &name, value)
}

func (p *ParameterSet) SetSymbol(name, value string) error {
	if err := p.open(); err != nil {
		return err
	}
	name = apiParameterName(name)
	symbol, err := p.params.Symbols().Intern(value)
	if err != nil {
		return err
	}
	return p.params.SetSymbolName(&p.value, &name, symbol)
}

func (p *ParameterSet) String() (string, error) {
	if err := p.open(); err != nil {
		return "", err
	}
	return p.params.Display(&p.value)
}

func (p *ParameterSet) Validate(descriptions *ParameterDescriptions) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := descriptions.open(); err != nil {
		return err
	}
	if descriptions.owner != p.owner {
		return ErrSMTContextOwner
	}
	return p.params.Validate(&p.value, descriptions.value)
}

func (p *ParameterSet) Close() error {
	if p == nil || p.closed {
		return nil
	}
	if p.params == nil {
		return ErrSMTContextOwner
	}
	err := p.params.Del(&p.value)
	if err == nil {
		p.closed = true
		p.owner, p.params = nil, nil
	}
	return err
}

type ParameterDescriptions struct {
	owner   *smtContextState
	value   *uparams.ParamDescrs
	symbols *usymbol.Manager
	closed  bool
}

// ParameterDescriptions copies a registered module catalog into an independent
// API object, matching Z3_param_descrs_ref value ownership.
func (c *SMTContext) ParameterDescriptions(module string) (*ParameterDescriptions, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	source, err := globals.SourceModuleDescriptorsName(module)
	if err != nil {
		return nil, err
	}
	value, err := uparams.NewParamDescrs(globals.Context.Symbols(), uparams.ParamDescrsConfig{Budget: globals.Context.Budget()})
	if err != nil {
		return nil, err
	}
	if err = value.Copy(source); err != nil {
		_ = value.Close()
		return nil, err
	}
	return &ParameterDescriptions{owner: c.smtContextState, value: value, symbols: globals.Context.Symbols()}, nil
}

func (d *ParameterDescriptions) open() error {
	if d == nil || d.owner == nil || d.value == nil || d.closed || d.owner.closed {
		return ErrSMTContextOwner
	}
	_, err := d.value.Size()
	return err
}

func apiParameterKind(kind uparams.ParamKind) ParameterKind {
	switch kind {
	case uparams.ParamUInt:
		return ParameterUInt
	case uparams.ParamBool:
		return ParameterBool
	case uparams.ParamDouble:
		return ParameterDouble
	case uparams.ParamString:
		return ParameterString
	case uparams.ParamSymbol:
		return ParameterSymbol
	case uparams.ParamInvalid:
		return ParameterInvalid
	default:
		return ParameterOther
	}
}

func (d *ParameterDescriptions) Kind(name string) (ParameterKind, error) {
	if err := d.open(); err != nil {
		return ParameterInvalid, err
	}
	name = apiParameterName(name)
	kind, err := d.value.KindName(name)
	return apiParameterKind(kind), err
}

func (d *ParameterDescriptions) Size() (uint32, error) {
	if err := d.open(); err != nil {
		return 0, err
	}
	return d.value.Size()
}

func (d *ParameterDescriptions) Name(index uint32) (string, error) {
	if err := d.open(); err != nil {
		return "", err
	}
	symbol, err := d.value.ParamName(index)
	if err != nil {
		return "", err
	}
	return d.symbols.String(symbol)
}

func (d *ParameterDescriptions) Documentation(name string) (string, error) {
	if err := d.open(); err != nil {
		return "", err
	}
	name = apiParameterName(name)
	text, found, err := d.value.DescriptionName(name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", arena.ErrBounds
	}
	return text, nil
}

func (d *ParameterDescriptions) String() (string, error) {
	if err := d.open(); err != nil {
		return "", err
	}
	n, err := d.value.Size()
	if err != nil {
		return "", err
	}
	result := "("
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			result += ", "
		}
		name, x := d.Name(i)
		if x != nil {
			return "", x
		}
		result += name
	}
	return result + ")", nil
}

func (d *ParameterDescriptions) Close() error {
	if d == nil || d.closed {
		return nil
	}
	if d.value == nil {
		return ErrSMTContextOwner
	}
	err := d.value.Close()
	if err == nil {
		d.closed = true
		d.owner, d.value, d.symbols = nil, nil, nil
	}
	return err
}
