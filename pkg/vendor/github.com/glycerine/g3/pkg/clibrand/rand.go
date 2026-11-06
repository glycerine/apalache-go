// Package clibrand supplies the shared C rand/srand stream for the selected
// Linux/glibc source profile. It implements the mathematical recurrence
// described by Peter Selinger at https://www.mathstat.dal.ca/~selinger/random/;
// no libc implementation code is incorporated. Other libc profiles remain
// separate portability obligations.
package clibrand

// Source dependency: src/math/polynomial/polynomial.cpp:4053-4068;
// src/test/lp.cpp:1459.

import "sync"

// RandMax is RAND_MAX for the selected source profile.
const RandMax uint32 = 1<<31 - 1

// GLibc31 holds an independent fixed-size instance of the selected recurrence.
// Its zero value starts with seed 1. Source rand consumers use Rand instead,
// because the source stream belongs to the process, not to a solver manager.
type GLibc31 struct {
	words       [31]uint32
	front       uint32
	initialized bool
}

// Seed replaces the entire state. The source treats zero as seed 1.
func (g *GLibc31) Seed(seed uint32) {
	if seed == 0 {
		seed = 1
	}
	g.words[0] = seed
	for i := 1; i < len(g.words); i++ {
		v := (16807 * int64(int32(g.words[i-1]))) % int64(RandMax)
		if v < 0 {
			v += int64(RandMax)
		}
		g.words[i] = uint32(v)
	}
	g.front = 3
	g.initialized = true
	// The ring already holds r0 through r30. Its first three steps generate
	// r34 through r36 using r31=r0, r32=r1, and r33=r2. Discard through r343.
	for i := 0; i < 310; i++ {
		g.next()
	}
}

func (g *GLibc31) next() uint32 {
	rear := g.front + 28
	if rear >= 31 {
		rear -= 31
	}
	v := g.words[g.front] + g.words[rear]
	g.words[g.front] = v
	g.front++
	if g.front == 31 {
		g.front = 0
	}
	return v >> 1
}

// Next advances this instance and returns a nonnegative C int value.
func (g *GLibc31) Next() uint32 {
	if !g.initialized {
		g.Seed(1)
	}
	return g.next()
}

var process struct {
	mu    sync.Mutex
	state GLibc31
}

// Rand advances the process stream. Borrowed Rand function values all share
// this state and observe every Srand call.
func Rand() uint32 {
	process.mu.Lock()
	v := process.state.Next()
	process.mu.Unlock()
	return v
}

// Srand reseeds the process stream.
func Srand(seed uint32) {
	process.mu.Lock()
	process.state.Seed(seed)
	process.mu.Unlock()
}
