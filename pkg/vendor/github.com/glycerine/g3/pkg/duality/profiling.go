// Package duality implements the legacy proof-based fixedpoint engine.
package duality

// Source: src/duality/duality_profiling.cpp;
// src/duality/duality_profiling.h.
// Original author: Ken McMillan.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/glycerine/g3/pkg/interp"
)

type profileNode struct {
	name     string
	elapsed  time.Duration
	started  time.Time
	parent   *profileNode
	children map[string]*profileNode
}

// Profiler owns the nested Duality timers and the interpolation profiler that
// the source appends to every Duality profile report.
//
// The C++ implementation stores this state in process globals. An explicit
// owner preserves the single-run behavior without adding shared mutable state.
type Profiler struct {
	top, current  *profileNode
	now           func() time.Time
	origin        time.Time
	interpolation *interp.Profiler
}

// NewProfiler constructs an empty Duality profile. A nil interpolation
// profiler creates the source-equivalent empty interpolation profile.
func NewProfiler(interpolation *interp.Profiler) *Profiler {
	if interpolation == nil {
		interpolation = interp.NewProfiler()
	}
	now := time.Now
	top := &profileNode{name: "TOTAL", children: make(map[string]*profileNode)}
	return &Profiler{
		top:           top,
		current:       top,
		now:           now,
		origin:        now(),
		interpolation: interpolation,
	}
}

// ProcessProfilers returns the process-lifetime timer owners used by the
// fixedpoint Duality backend. The source retains both timer trees globally;
// callers that need an independent profiler can still use NewProfiler.
// Like the source timers, these owners assume single-goroutine use.
func ProcessProfilers() (*Profiler, *interp.Profiler) {
	return processProfiler, processInterpolationProfiler
}

var processInterpolationProfiler = interp.ProcessProfiler()
var processProfiler = NewProfiler(processInterpolationProfiler)

// TimerStart enters a named child timer. Reusing a child retains its accumulated
// time and replaces only its start time, as in the source map entry.
func (p *Profiler) TimerStart(name string) error {
	if p == nil || p.current == nil || p.now == nil || name == "" {
		return fmt.Errorf("invalid duality profiler")
	}
	child := p.current.children[name]
	if child == nil {
		child = &profileNode{name: name, parent: p.current, children: make(map[string]*profileNode)}
		p.current.children[name] = child
	}
	child.started = p.now()
	p.current = child
	return nil
}

// TimerStop leaves the named timer. The source deliberately recovers from a
// lost inner stop: it walks outward to name, charges that timer, and discards
// the intervening timer samples. A missing name simply restores the top level.
func (p *Profiler) TimerStop(name string) error {
	if p == nil || p.current == nil || p.now == nil || name == "" {
		return fmt.Errorf("invalid duality profiler")
	}
	for p.current.name != name && p.current.parent != nil {
		p.current = p.current.parent
	}
	if p.current.parent != nil {
		p.current.elapsed += p.now().Sub(p.current.started)
		p.current = p.current.parent
	}
	return nil
}

func formatSeconds(d time.Duration) string { return fmt.Sprintf("%g", d.Seconds()) }

func (p *Profiler) printNode(w io.Writer, node *profileNode, indent int, totals map[string]time.Duration) error {
	prefix := strings.Repeat("  ", indent) + node.name
	dots := 70 - 2*indent - len(node.name)
	if dots < 0 {
		dots = 0
	}
	if _, err := fmt.Fprintf(w, "%s%s%s\n", prefix, strings.Repeat(".", dots), formatSeconds(node.elapsed)); err != nil {
		return err
	}
	if indent != 0 {
		totals[node.name] += node.elapsed
	}
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := p.printNode(w, node.children[name], indent+1, totals); err != nil {
			return err
		}
	}
	return nil
}

// Print writes the Duality timer tree, totals grouped by timer name, and the
// interpolation profile in the same three stages as the source.
func (p *Profiler) Print(w io.Writer) error {
	if p == nil || p.top == nil || p.interpolation == nil || w == nil {
		return fmt.Errorf("invalid duality profiler")
	}
	p.top.elapsed = 0
	for _, child := range p.top.children {
		p.top.elapsed += child.elapsed
	}
	totals := make(map[string]time.Duration)
	if err := p.printNode(w, p.top, 0, totals); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "TOTALS:\n"); err != nil {
		return err
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "%s %s\n", name, formatSeconds(totals[name])); err != nil {
			return err
		}
	}
	return p.interpolation.Print(w)
}

// ShowTime writes elapsed monotonic time since profiler construction. This is
// the portable replacement for the source process-clock diagnostic.
func (p *Profiler) ShowTime(w io.Writer) error {
	if p == nil || p.now == nil || p.origin.IsZero() || w == nil {
		return fmt.Errorf("invalid duality profiler")
	}
	_, err := fmt.Fprintf(w, "%g\n", p.now().Sub(p.origin).Seconds())
	return err
}
