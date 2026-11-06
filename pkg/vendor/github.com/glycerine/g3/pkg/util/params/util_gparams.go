package params

// Source: src/util/gparams.cpp:81-123,633-644; src/util/gparams.h.
// Explicit runtime-owned gparams::imp; source catalog dispatch is borrowed.

import (
	"errors"
	"reflect"
	"sync"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrGlobalParamsOwner = errors.New("g3: global parameters owner or lifetime mismatch")
var ErrGlobalParamsRegistrationMissing = errors.New("g3: actual parameter registration dispatcher missing")

type GlobalParamsConfig struct {
	Catalog                                     *GlobalParamsCatalog
	Budget                                      *arena.Budget
	DescriptorPageSize, DescriptorWordsPageSize uint32
	CLongBits                                   uint8
	Register                                    func(GlobalParamsRegistrar) error
	onAllocated                                 func(*GlobalParams)
	memoryTrace                                 *udiag.TraceTags
	memoryProcess                               *umemory.ProcessAccounting
}
type GparamsRefCell struct{ value ParamsRef }

var gparamsRefCellBytes = uint64(reflect.TypeOf(GparamsRefCell{}).Size())

type GparamsRegionBinding struct {
	origin uint64
	region *arena.BytesRegion
}
type GlobalParams struct {
	catalog                                 *GlobalParamsCatalog
	mu                                      sync.Mutex
	self                                    *GlobalParams
	SymbolsField                            *usymbol.Manager
	ContextField                            *ParamsContext
	Budget                                  *arena.Budget
	memoryTrace                             *udiag.TraceTags
	memoryProcess                           *umemory.ProcessAccounting
	Register                                func(GlobalParamsRegistrar) error
	modulesRegistered, registering, closed  bool
	sourceStarted, sourceCompleted          bool
	sourceStage                             uint8
	sourceIndex                             uint32
	sourceCause                             error
	sourceDecrement                         ParamsDecrementReceipt
	sourceReset                             ParamsResetReceipt
	OwnerBytes                              uint64
	constructorReady                        bool
	constructorCompleted, constructorUnwind uint8
	constructorCause                        error
	constructorPendingDescriptor            *ParamDescrs
	RetainedNext                            *GlobalParams
	cLongBits                               uint8
	ModuleDescriptors                       gparamsDescriptorDictionary
	ModuleDescriptions                      gparamsDescriptionDictionary
	ModuleParams                            gparamsRefDictionary
	Descriptors                             []*ParamDescrs
	Cells                                   []*GparamsRefCell
	Bindings                                []GparamsRegionBinding
	GlobalDescriptors                       *ParamDescrs
	globalParams                            ParamsRef
	Workspace                               []byte
}

// Registrar is a cold by-value capability used only inside the registration
// critical section. Its methods use private unlocked owner operations.
type GlobalParamsRegistrar struct{ owner *GlobalParams }

func (r GlobalParamsRegistrar) RegisterModule(name string, d *ParamDescrs) error {
	if r.owner == nil || !r.owner.registering {
		return ErrGlobalParamsOwner
	}
	return r.owner.registerModule(name, d)
}
func (r GlobalParamsRegistrar) RegisterGlobal(d *ParamDescrs) error {
	if r.owner == nil || !r.owner.registering {
		return ErrGlobalParamsOwner
	}
	return r.owner.GlobalDescriptors.Copy(d)
}
func newSourceGlobalParams(s *usymbol.Manager, p *ParamsContext, c GlobalParamsConfig) (*GlobalParams, error) {
	if s == nil || p == nil || p.SymbolsField != s {
		return nil, ErrGlobalParamsOwner
	}
	if e := p.open(); e != nil {
		return nil, e
	}
	if c.Catalog != nil {
		if c.Register != nil {
			return nil, arena.ErrConfig
		}
		c.Catalog.mu.Lock()
		e := c.Catalog.open()
		same := c.Catalog.symbols == s
		c.Catalog.mu.Unlock()
		if e != nil {
			return nil, e
		}
		if !same {
			return nil, ErrGlobalParamsOwner
		}
	}
	longBits := c.CLongBits
	if longBits == 0 {
		longBits = 64
	}
	if longBits != 32 && longBits != 64 {
		return nil, arena.ErrConfig
	}

	n := uint64(reflect.TypeOf(GlobalParams{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	g := &GlobalParams{SymbolsField: s, ContextField: p, Budget: c.Budget, Register: c.Register, OwnerBytes: n, catalog: c.Catalog,
		memoryTrace: c.memoryTrace, memoryProcess: c.memoryProcess}
	g.self = g
	if c.onAllocated != nil {
		c.onAllocated(g) // source alloc(imp) traces before placement construction
	}
	g.cLongBits = longBits
	fail := func(e error) (*GlobalParams, error) {
		g.constructorCause = e
		g.constructorUnwind = g.constructorCompleted
		if cleanup := g.unwindConstructor(); cleanup != nil {
			return g, errors.Join(e, cleanup)
		}
		return g, e
	}
	g.ModuleDescriptors.init()
	g.constructorCompleted = 1
	g.ModuleDescriptions.init()
	g.constructorCompleted = 2
	d, e := newSourceParamDescrs(s, ParamDescrsConfig{Budget: c.Budget, Text: arena.VectorConfig{PageSize: c.DescriptorWordsPageSize, Descriptors: arena.Config{PageSize: c.DescriptorPageSize, Budget: c.Budget}}, memoryOwner: g})
	if e != nil {
		g.constructorPendingDescriptor = d
		return fail(e)
	}
	g.GlobalDescriptors = d
	g.constructorCompleted = 3
	g.ModuleParams.init()
	g.constructorCompleted = 4
	g.globalParams, e = p.NewValue()
	if e != nil {
		return fail(e)
	}
	g.constructorCompleted = 5
	g.constructorReady = true
	return g, nil
}
func (g *GlobalParams) open() error {
	if g == nil || g.self != g {
		return ErrGlobalParamsOwner
	}
	if !g.constructorReady {
		return ErrGlobalParamsUnconstructed
	}
	if g.closed || g.sourceStarted {
		return arena.ErrClosed
	}
	return g.ContextField.open()
}
func (g *GlobalParams) ensureRegistered() error {
	if e := g.open(); e != nil {
		return e
	}
	if g.modulesRegistered {
		return nil
	}
	g.modulesRegistered = true
	if g.catalog != nil {
		g.registering = true
		defer func() { g.registering = false }()
		return g.catalog.Register(GlobalParamsRegistrar{g})
	}
	if g.Register == nil {
		return ErrGlobalParamsRegistrationMissing
	}
	g.registering = true
	defer func() { g.registering = false }()
	return g.Register(GlobalParamsRegistrar{g})
}
func (g *GlobalParams) moduleDescriptor(k usymbol.Symbol) (*ParamDescrs, error) {
	if e := g.ensureRegistered(); e != nil {
		return nil, e
	}
	if _, e := g.SymbolsField.Hash(k); e != nil {
		return nil, e
	}
	n, ok := g.ModuleDescriptors.get(k)
	if !ok {
		return nil, ErrGlobalParamsOwner
	}
	if n == 0 || int(n) > len(g.Descriptors) || g.Descriptors[n-1] == nil {
		return nil, ErrGlobalParamsOwner
	}
	return g.Descriptors[n-1], nil
}
func (g *GlobalParams) registerModule(name string, d *ParamDescrs) error {
	if e := g.open(); e != nil {
		return e
	}
	if d == nil || d.manager != g.SymbolsField {
		return ErrGlobalParamsOwner
	}
	if e := d.open(); e != nil {
		return e
	}
	k, e := g.SymbolsField.Intern(name)
	if e != nil {
		return e
	}
	if _, e := g.SymbolsField.Hash(k); e != nil {
		return e
	}
	if descriptor, ok := g.ModuleDescriptors.get(k); ok {
		old := g.Descriptors[descriptor-1]
		if old == d {
			return ErrGlobalParamsOwner
		}
		if e = old.Copy(d); e != nil {
			return e
		}
		if e = d.Close(); e != nil {
			return e
		}
		g.traceModuleDescriptorDeallocated(d, 143)
		return nil
	}
	if uint64(len(g.Descriptors)) >= uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	slot := uint32(len(g.Descriptors) + 1)
	g.ModuleDescriptors.set(k, slot)
	g.Descriptors = append(g.Descriptors, d)
	return nil
}
func (g *GlobalParams) RegisterModule(name string, d *ParamDescrs) error {
	if g == nil {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.registerModule(name, d)
}
func (g *GlobalParams) RegisterGlobal(d *ParamDescrs) error {
	if g == nil {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return e
	}
	return g.GlobalDescriptors.Copy(d)
}
func (g *GlobalParams) GetParams(k usymbol.Symbol) (*ParamsRef, error) {
	if e := g.open(); e != nil {
		return nil, e
	}
	if _, e := g.SymbolsField.Hash(k); e != nil {
		return nil, e
	}
	if k == (usymbol.Symbol{}) {
		return &g.globalParams, nil
	}
	if n, ok := g.ModuleParams.get(k); ok {
		if e := g.sourceGetParamsAssertion(n != 0); e != nil {
			return nil, e
		}
		if n == 0 || int(n) > len(g.Cells) || g.Cells[n-1] == nil {
			return nil, ErrGlobalParamsOwner
		}
		return &g.Cells[n-1].value, nil
	}
	if e := g.Budget.Reserve(gparamsRefCellBytes); e != nil {
		return nil, e
	}
	cell := &GparamsRefCell{}
	g.traceModuleCellAllocated(cell)
	v, e := g.ContextField.NewValue()
	if e != nil {
		g.Budget.Release(gparamsRefCellBytes)
		return nil, e
	}
	cell.value = v
	// Scalar directory slots are private metadata, not externally issued handles.
	// Reuse released roots so repeated source reset/new-cell cycles plateau.
	slot := len(g.Cells)
	for i, old := range g.Cells {
		if old == nil {
			slot = i
			break
		}
	}
	if uint64(slot) >= uint64(^uint32(0)) {
		_ = g.ContextField.Del(&v)
		g.Budget.Release(gparamsRefCellBytes)
		return nil, arena.ErrBounds
	}
	n := uint32(slot + 1)
	g.ModuleParams.set(k, n)
	if slot == len(g.Cells) {
		g.Cells = append(g.Cells, cell)
	} else {
		g.Cells[slot] = cell
	}
	if e := g.sourceGetParamsAssertion(cell != nil); e != nil {
		return nil, e
	}
	return &cell.value, nil
}

func (g *GlobalParams) sourceGetParamsAssertion(present bool) error {
	d := udiag.BorrowProcessDebugContext()
	enabled, e := udiag.SourceAssertionsEnabled(d)
	if e != nil || !enabled || present {
		return e
	}
	return udiag.DebugAssertionFailure(d, "src/util/gparams.cpp", 195, "p != 0")
}
func (g *GlobalParams) getModule(k usymbol.Symbol) (ParamsRef, error) {
	if e := g.open(); e != nil {
		return ParamsRef{}, e
	}
	if _, e := g.SymbolsField.Hash(k); e != nil {
		return ParamsRef{}, e
	}
	v, e := g.ContextField.NewValue()
	if e != nil {
		return ParamsRef{}, e
	}
	if n, ok := g.ModuleParams.get(k); ok {
		if n == 0 || int(n) > len(g.Cells) || g.Cells[n-1] == nil {
			return ParamsRef{}, ErrGlobalParamsOwner
		}
		if e = g.ContextField.Assign(&v, &g.Cells[n-1].value); e != nil {
			return ParamsRef{}, e
		}
	}
	return v, nil
}
func (g *GlobalParams) GetModule(k usymbol.Symbol) (ParamsRef, error) {
	if g == nil {
		return ParamsRef{}, ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.getModule(k)
}
func (g *GlobalParams) GetModuleName(name string) (ParamsRef, error) {
	if g == nil {
		return ParamsRef{}, ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return ParamsRef{}, e
	}
	k, e := g.SymbolsField.Intern(name)
	if e != nil {
		return ParamsRef{}, e
	}
	return g.getModule(k)
}
func (g *GlobalParams) reset() error {
	if g.globalParams.Owner != 0 {
		if e := g.ContextField.Reset(&g.globalParams); e != nil {
			return e
		}
	}
	for _, i := range g.ModuleParams.values {
		if i == 0 || int(i) > len(g.Cells) || g.Cells[i-1] == nil {
			return ErrGlobalParamsOwner
		}
		cell := g.Cells[i-1]
		if e := g.ContextField.Del(&cell.value); e != nil {
			return e
		}
		g.Cells[i-1] = nil
		g.Budget.Release(gparamsRefCellBytes)
		g.traceModuleCellDeallocated(cell)
	}
	g.ModuleParams.reset()
	return nil
}
func (g *GlobalParams) Reset() error {
	if g == nil {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.open(); e != nil {
		return e
	}
	return g.reset()
}
func (g *GlobalParams) Close() error {
	if g != nil && g.self == g && g.sourceStarted {
		return g.CleanupStopped()
	}
	return g.closeLegacy()
}
func (g *GlobalParams) closeLegacy() error {
	if g == nil || g.self != g {
		return ErrGlobalParamsOwner
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	if e := g.reset(); e != nil {
		return e
	}
	for _, descriptor := range g.ModuleDescriptors.values.All() {
		d := g.Descriptors[descriptor-1]
		if d != nil {
			if e := d.Close(); e != nil {
				return e
			}
			g.traceModuleDescriptorDeallocated(d, 108)
			g.Descriptors[descriptor-1] = nil
		}
	}
	if g.GlobalDescriptors != nil {
		if e := g.GlobalDescriptors.Close(); e != nil {
			return e
		}
	}
	// Global null Ref is a borrowed owner-local scalar after reset; source
	// member destruction decrements it if still initialized, without mutation.
	if g.globalParams.Owner != 0 {
		if e := g.ContextField.Del(&g.globalParams); e != nil {
			return e
		}
	}
	g.ModuleParams.close()
	g.ModuleDescriptions.close()
	g.ModuleDescriptors.close()
	g.Budget.Release(g.OwnerBytes)
	g.Cells = nil
	g.Descriptors = nil
	g.Bindings = nil
	g.Workspace = nil
	g.OwnerBytes = 0
	g.closed = true
	return nil
}
