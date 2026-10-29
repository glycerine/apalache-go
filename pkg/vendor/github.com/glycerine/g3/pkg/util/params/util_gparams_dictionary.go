package params

// Source: src/util/gparams.cpp; src/util/dictionary.h; src/util/map.h.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	// Module descriptors are rendered by traversal, so their dictionary preserves
	// registration order. The other two dictionaries are lookup-only.
)

import "github.com/glycerine/insdict"

type gparamsDescriptorDictionary struct {
	values *insdict.Dict[usymbol.Symbol, uint32]
}

type gparamsRefDictionary struct {
	values map[usymbol.Symbol]uint32
}

type gparamsDescriptionDictionary struct {
	values map[usymbol.Symbol]utext.RegionCString
}

func (d *gparamsDescriptorDictionary) init() {
	d.values = insdict.NewDictFunc[usymbol.Symbol, uint32](func(k usymbol.Symbol) uint64 {
		words := k.SymbolWords()
		h := (uint64(words[0]) | uint64(words[1])<<32) ^ uint64(k.Value)<<32 | uint64(k.Tag)
		return h ^ uint64(k.Generation)*0x9e3779b97f4a7c15
	})
}
func (d *gparamsDescriptorDictionary) get(k usymbol.Symbol) (uint32, bool) {
	if d.values == nil {
		return 0, false
	}
	return d.values.Get2(k)
}
func (d *gparamsDescriptorDictionary) set(k usymbol.Symbol, descriptor uint32) {
	if d.values == nil {
		d.init()
	}
	d.values.Put(k, descriptor)
}
func (d *gparamsDescriptorDictionary) reset() {
	if d.values != nil {
		d.values.Clear()
	}
}
func (d *gparamsDescriptorDictionary) close() { d.values = nil }
func (d *gparamsDescriptorDictionary) size() uint32 {
	if d.values == nil {
		return 0
	}
	return uint32(d.values.Len())
}

func (d *gparamsRefDictionary) init() { d.values = make(map[usymbol.Symbol]uint32) }
func (d *gparamsRefDictionary) get(k usymbol.Symbol) (uint32, bool) {
	v, ok := d.values[k]
	return v, ok
}
func (d *gparamsRefDictionary) set(k usymbol.Symbol, cell uint32) {
	if d.values == nil {
		d.init()
	}
	d.values[k] = cell
}
func (d *gparamsRefDictionary) reset() { clear(d.values) }
func (d *gparamsRefDictionary) close() { d.values = nil }
func (d *gparamsRefDictionary) size() uint32 {
	return uint32(len(d.values))
}

func (d *gparamsDescriptionDictionary) init() {
	d.values = make(map[usymbol.Symbol]utext.RegionCString)
}
func (d *gparamsDescriptionDictionary) get(k usymbol.Symbol) (utext.RegionCString, bool) {
	v, ok := d.values[k]
	return v, ok
}
func (d *gparamsDescriptionDictionary) set(k usymbol.Symbol, text utext.RegionCString) {
	if d.values == nil {
		d.init()
	}
	d.values[k] = text
}
func (d *gparamsDescriptionDictionary) reset() { clear(d.values) }
func (d *gparamsDescriptionDictionary) close() { d.values = nil }
func (d *gparamsDescriptionDictionary) size() uint32 {
	return uint32(len(d.values))
}
