package sat

// Source: src/sat/sat_scc.cpp; src/sat/sat_scc.h; src/sat/sat_scc_params.pyg.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// Traversal arrays are native Go storage. Frames borrow the actual watch iterator;
// no encoded frame or container backing is allocated through Runtime.
type satSCC struct {
	self                                             *satSCC
	solver                                           *Solver // borrowed source owner for mk_stat in report
	lifecycle                                        *satSolverClauseLifecycle
	index, lowlink, stack                            []uint32
	inStack                                          []bool
	roots                                            []Literal
	frames                                           []satSCCFrame
	toElim                                           []BoolVar
	eliminator                                       satElimEqs
	numElim, nextIndex, reportNumElim                uint32
	header                                           uint64
	bodyCause, cleanupCause                          error
	report                                           stopwatch.Stopwatch
	reportActive                                     bool
	enabled, active, closing, closed, debugInvariant bool
}

func newSATSCC(l *satSolverClauseLifecycle, p *uparams.ParamsRef) (*satSCC, error) {
	if e := l.constructionBorrow(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satSCC{}).Size())
	if e := l.budget.Reserve(n); e != nil {
		return nil, e
	}
	c := &satSCC{lifecycle: l, header: n}
	c.self = c
	c.eliminator.lifecycle = l
	c.eliminator.self = &c.eliminator
	if e := c.Update(p); e != nil {
		l.budget.Release(n)
		return nil, e
	}
	return c, nil
}
func (c *satSCC) constructionBorrow() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	if c.closed || c.closing {
		return arena.ErrClosed
	}
	return c.lifecycle.constructionBorrow()
}
func (c *satSCC) open() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	if c.closed || c.closing {
		return arena.ErrClosed
	}
	return c.lifecycle.open()
}
func (c *satSCC) Update(local *uparams.ParamsRef) (err error) {
	if e := c.constructionBorrow(); e != nil {
		return e
	}
	cfg := c.lifecycle.propagation.config
	g, e := cfg.moduleParams()
	if e != nil {
		return e
	}
	defer func() {
		if e := cfg.context.Del(&g); err == nil {
			err = e
		}
	}()
	key := "scc"
	enabled, e := cfg.context.GetBoolFallbackName(local, &key, &g, true)
	if e == nil {
		c.enabled = enabled
	}
	return e
}
func (c *satSCC) resetStatistics() { c.numElim = 0 }
func (c *satSCC) collectStatistics(s *statistics.Statistics, key statistics.Key) error {
	if e := c.open(); e != nil {
		return e
	}
	s.UpdateUint(key, c.numElim)
	return nil
}
func (c *satSCC) closeVectors() error {
	c.index, c.lowlink, c.stack = nil, nil, nil
	c.inStack, c.roots, c.frames, c.toElim = nil, nil, nil, nil
	c.active = false
	if c.reportActive {
		if e := c.report.Stop(); e != nil {
			return e
		}
		c.reportActive = false
		if c.solver != nil {
			return c.emitReport()
		}
	}
	return nil
}
func (c *satSCC) Close() error {
	if c == nil || c.self != c {
		return ErrSATOwner
	}
	if c.closed {
		return nil
	}
	c.closing = true
	if e := c.eliminator.cleanupInterrupted(); e != nil {
		return e
	}
	if e := c.closeVectors(); e != nil {
		return e
	}
	c.lifecycle.budget.Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
func (c *satSCC) newNode(lidx uint32) error {
	p := c.lifecycle.propagation
	c.index[lidx] = c.nextIndex
	c.lowlink[lidx] = c.nextIndex
	c.nextIndex++
	c.stack = append(c.stack, lidx)
	c.inStack[lidx] = true
	list, e := p.variables.watches.Get(lidx)
	if e != nil {
		return e
	}
	ref, e := list.ref()
	if e != nil {
		return e
	}
	it, e := list.owner.store.Iter(ref)
	if e != nil {
		return e
	}
	c.frames = append(c.frames, satSCCFrame{literal: lidx, first: true, iterator: it})
	return nil
}

type satSCCFrame struct {
	literal  uint32
	first    bool
	iterator satSolverWatchIterator
}

func (c *satSCC) frame(f uint32) (uint32, bool, satSolverWatchIterator, error) {
	if uint64(f) >= uint64(len(c.frames)) {
		return 0, false, satSolverWatchIterator{}, arena.ErrBounds
	}
	x := c.frames[f]
	return x.literal, x.first, x.iterator, nil
}
func (c *satSCC) run() (count uint32, err error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	p := c.lifecycle.propagation
	if p.inconsistent || !c.enabled {
		return 0, nil
	}
	if c.active || c.reportActive {
		return 0, arena.ErrClosed
	}
	if satStorageSourceDebug && c.debugInvariant {
		if e := c.lifecycle.checkInvariant(); e != nil {
			return 0, e
		}
	}
	c.report = stopwatch.Stopwatch{}
	c.reportNumElim = c.numElim
	if e := c.report.Start(); e != nil {
		return 0, e
	}
	c.reportActive = true
	c.bodyCause = nil
	c.cleanupCause = nil
	c.active = true
	defer func() {
		c.bodyCause = err
		if e := c.closeVectors(); e != nil {
			c.cleanupCause = e
			if err == nil {
				err = e
			} else if !errors.Is(err, e) {
				err = errors.Join(err, e)
			}
		}
	}()
	c.traceSolverDisplay("scc", 68)
	c.traceSolverWatches()
	nv, e := p.variables.numVarsChecked()
	if e != nil {
		return 0, e
	}
	nl := nv * 2
	c.nextIndex = 0
	c.index = make([]uint32, nl)
	c.lowlink = make([]uint32, nl)
	for i := range c.index {
		c.index[i], c.lowlink[i] = ^uint32(0), ^uint32(0)
	}
	c.inStack = make([]bool, nl)
	c.roots = make([]Literal, nv)
	for i := range c.roots {
		c.roots[i] = DefaultLiteral()
	}
	for lidx := uint32(0); lidx < nl; lidx++ {
		ix := c.index[lidx]
		if ix != ^uint32(0) {
			continue
		}
		el, e := p.variables.wasEliminated(Literal(lidx).variable())
		if e != nil {
			return 0, e
		}
		if el {
			continue
		}
		if e = p.checkpoint(); e != nil {
			return 0, e
		}
		if e = c.newNode(lidx); e != nil {
			return 0, e
		}
		for {
			fn := uint32(len(c.frames))
			if fn == 0 {
				break
			}
			f := fn - 1
			li, first, it, e := c.frame(f)
			if e != nil {
				return 0, e
			}
			if !first {
				w, e := satPropagationCursorRead(&it, it.index)
				if e != nil {
					return 0, e
				}
				lit, e := w.Literal()
				if e != nil {
					return 0, e
				}
				child := c.index[lit.index()]
				if satStorageSourceDebug && child == ^uint32(0) {
					return 0, ErrSATAssertion
				}
				lo := c.lowlink[lit.index()]
				here := c.lowlink[li]
				if lo < here {
					c.lowlink[li] = lo
				}
				it.index++
			}
			c.frames[f].first = false
			descend := false
			for it.index != it.end {
				w, e := satPropagationCursorRead(&it, it.index)
				if e != nil {
					return 0, e
				}
				if w.Kind() != 0 {
					it.index++
					continue
				}
				lit, e := w.Literal()
				if e != nil {
					return 0, e
				}
				ix := c.index[lit.index()]
				if ix == ^uint32(0) {
					c.frames[f].iterator.index = it.index
					if e = c.newNode(lit.index()); e != nil {
						return 0, e
					}
					descend = true
					break
				}
				inside := c.inStack[lit.index()]
				if inside {
					lo := c.lowlink[li]
					if ix < lo {
						c.lowlink[li] = ix
					}
				}
				it.index++
			}
			if descend {
				continue
			}
			c.frames[f].iterator.index = it.index
			lo := c.lowlink[li]
			ix = c.index[li]
			if lo == ix {
				if e = c.component(li); e != nil {
					return 0, e
				}
				if p.inconsistent {
					return 0, nil
				}
			}
			c.frames[len(c.frames)-1] = satSCCFrame{}
			c.frames = c.frames[:len(c.frames)-1]
		}
	}
	c.traceRoots()
	n := uint32(len(c.toElim))
	c.numElim += n
	if e = c.eliminator.run(c.roots, c.toElim); e != nil {
		return 0, e
	}
	c.traceSolverDisplay("scc_detail", 221)
	if satStorageSourceDebug && c.debugInvariant {
		if e := c.lifecycle.checkInvariant(); e != nil {
			return 0, e
		}
	}
	return n, nil
}
func (c *satSCC) component(li uint32) error {
	p := c.lifecycle.propagation
	l := Literal(li)
	v := l.variable()
	rv := c.roots[v]
	sn := uint32(len(c.stack))
	if sn == 0 {
		return ErrSATAssertion
	}
	c.traceCycle(li)
	if rv != DefaultLiteral() {
		c.traceSCCDetail(156, "consuming stack...\n")
		for {
			n := uint32(len(c.stack))
			x := c.stack[n-1]
			c.stack[len(c.stack)-1] = 0
			c.stack = c.stack[:len(c.stack)-1]
			c.inStack[x] = false
			if satStorageSourceDebug {
				rr := c.roots[uint32(Literal(x).variable())]
				if rr.variable() != rv.variable() {
					return ErrSATAssertion
				}
			}
			if x == li {
				break
			}
		}
		return nil
	}
	c.traceSCCDetail(168, "assigning roots...\n")
	root := DefaultLiteral()
	j := sn - 1
	for {
		x := c.stack[j]
		j--
		if Literal(x) == l.Negated() {
			p.setConflict(satJustification{}, DefaultLiteral())
			return nil
		}
		ext, e := p.variables.IsExternal(Literal(x).variable())
		if e != nil {
			return e
		}
		if ext {
			root = Literal(x)
			break
		}
		if x == li {
			break
		}
	}
	if root == DefaultLiteral() {
		root = l
	}
	c.traceRoot(root)
	for {
		n := uint32(len(c.stack))
		x := c.stack[n-1]
		c.stack[len(c.stack)-1] = 0
		c.stack = c.stack[:len(c.stack)-1]
		c.inStack[x] = false
		lit := Literal(x)
		vv := lit.variable()
		rr := c.roots[vv]
		if rr == DefaultLiteral() {
			rr = root
			if lit.sign() {
				rr = root.Negated()
			}
			c.roots[vv] = rr
			if vv != root.variable() {
				c.toElim = append(c.toElim, vv)
			}
		}
		if x == li {
			break
		}
	}
	return nil
}

func collectSCCParamDescrs(d *uparams.ParamDescrs) error {
	return satConfigInsertDescriptor(d, "scc", uparams.ParamBool, "eliminate Boolean variables by computing strongly connected components", "true")
}
