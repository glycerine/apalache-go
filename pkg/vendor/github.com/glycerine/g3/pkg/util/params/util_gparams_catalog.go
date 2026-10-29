package params

import (
	"errors"
	"reflect"
	"sync"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrGlobalParamsCatalogPending = errors.New("g3: parameter catalog retains unpublished descriptor owner")

type GlobalParamsCatalogConfig struct {
	Budget       *arena.Budget
	Descriptions arena.BytesRegionConfig
	Text         arena.VectorConfig
}

// Catalog owns actual storage for the source's borrowed static descriptions.
// Registries borrow this region and must be closed before the catalog. Every
// created descriptor owner is transferred or retained in pending on failure.
type GlobalParamsCatalog struct {
	mu           sync.Mutex
	self         *GlobalParamsCatalog
	symbols      *usymbol.Manager
	budget       *arena.Budget
	Region       *arena.BytesRegion
	descriptions [16]arena.BytesRegionRef
	text         arena.VectorConfig
	Pending      *ParamDescrs
	OwnerBytes   uint64
	closed       bool
}

func NewGlobalParamsCatalog(s *usymbol.Manager, c GlobalParamsCatalogConfig) (*GlobalParamsCatalog, error) {
	if s == nil {
		return nil, ErrGlobalParamsOwner
	}
	if _, e := s.Hash(usymbol.Symbol{}); e != nil {
		return nil, e
	}
	if c.Descriptions.Descriptors.Budget != nil && c.Descriptions.Descriptors.Budget != c.Budget || c.Text.Descriptors.Budget != nil && c.Text.Descriptors.Budget != c.Budget {
		return nil, arena.ErrConfig
	}
	c.Descriptions.Descriptors.Budget = c.Budget
	c.Text.Descriptors.Budget = c.Budget
	bytes := uint64(reflect.TypeOf(GlobalParamsCatalog{}).Size())
	if e := c.Budget.Reserve(bytes); e != nil {
		return nil, e
	}
	owner := &GlobalParamsCatalog{symbols: s, budget: c.Budget, text: c.Text, OwnerBytes: bytes}
	owner.self = owner
	fail := func(e error) (*GlobalParamsCatalog, error) { _ = owner.Close(); return nil, e }
	var e error
	owner.Region, e = arena.NewBytesRegion(c.Descriptions)
	if e != nil {
		return fail(e)
	}
	for i, row := range catalogDescriptionLiterals {
		ref, e := owner.Region.Allocate(uint32(len(row.text) + 1))
		if e != nil {
			return fail(e)
		}
		for j := range row.text {
			if e = owner.Region.Write(ref, uint32(j), row.text[j]); e != nil {
				return fail(e)
			}
		}
		if e = owner.Region.Write(ref, uint32(len(row.text)), 0); e != nil {
			return fail(e)
		}
		owner.descriptions[i] = ref
	}
	return owner, nil
}
func (c *GlobalParamsCatalog) open() error {
	if c == nil || c.self != c {
		return ErrGlobalParamsOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}
func (c *GlobalParamsCatalog) newDescriptors(owner *GlobalParams, onAllocated func(*ParamDescrs)) (*ParamDescrs, error) {
	return NewParamDescrs(c.symbols, ParamDescrsConfig{Budget: c.budget, Text: c.text, onAllocated: onAllocated, memoryOwner: owner})
}
func (c *GlobalParamsCatalog) global(r GlobalParamsRegistrar, collector func(*ParamDescrs) error) error {
	d, e := c.newDescriptors(r.owner, nil)
	if e != nil {
		return e
	}
	defer d.Close()
	if e = collector(d); e != nil {
		return e
	}
	return r.RegisterGlobal(d)
}
func (c *GlobalParamsCatalog) module(r GlobalParamsRegistrar, sourceLine int, name string, collector func(*ParamDescrs) error) error {
	d, e := c.newDescriptors(r.owner, func(d *ParamDescrs) { r.owner.traceModuleDescriptorAllocated(d, sourceLine) })
	if e != nil {
		return e
	}
	c.Pending = d
	if e = collector(d); e != nil {
		return e
	}
	if e = r.RegisterModule(name, d); e != nil {
		return e
	}
	c.Pending = nil
	return nil
}
func (c *GlobalParamsCatalog) Register(r GlobalParamsRegistrar) error {
	if c == nil || c.self != c {
		return ErrGlobalParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.open(); e != nil {
		return e
	}
	if r.owner == nil || !r.owner.registering || r.owner.SymbolsField != c.symbols {
		return ErrGlobalParamsOwner
	}
	if c.Pending != nil {
		return ErrGlobalParamsCatalogPending
	}
	return c.dispatch(r)
}
func (c *GlobalParamsCatalog) CleanupPending() error {
	if c == nil || c.self != c {
		return ErrGlobalParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.open(); e != nil {
		return e
	}
	if c.Pending != nil {
		if e := c.Pending.Close(); e != nil {
			return e
		}
		c.Pending = nil
	}
	return nil
}
func (c *GlobalParamsCatalog) Close() error {
	if c == nil || c.self != c {
		return ErrGlobalParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if c.Pending != nil {
		if e := c.Pending.Close(); e != nil {
			return e
		}
		c.Pending = nil
	}
	if c.Region != nil {
		c.Region.Close()
		c.Region = nil
	}
	c.budget.Release(c.OwnerBytes)
	c.OwnerBytes = 0
	c.closed = true
	return nil
}
