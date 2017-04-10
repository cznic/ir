// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
	"runtime"

	"github.com/cznic/mathutil"
)

func roundup(n, to int64) int64 {
	if r := n % to; r != 0 {
		return n + to - r
	}

	return n
}

// MemoryModelItem describes memory properties of a particular type kind.
type MemoryModelItem struct {
	Size        uint
	Align       uint
	StructAlign uint
}

// MemoryModel defines properties of types. A valid memory model must provide
// model items for all type kinds except Array, Struct and Union. Methods of
// invalid models may panic. Memory model instances are not modified by this
// package and safe for concurrent use by multiple goroutines as long as any of
// them does not modify them either.
type MemoryModel map[TypeKind]MemoryModelItem

// NewMemoryModel returns a new MemoryModel for the current architecture and
// platform or an error, if any.
func NewMemoryModel() (MemoryModel, error) {
	switch arch := runtime.GOARCH; arch {
	case
		"386",
		"arm",
		"arm64be",
		"armbe",
		"mips",
		"mipsle",
		"ppc",
		"ppc64le",
		"s390",
		"s390x",
		"sparc":

		return MemoryModel{
			Int8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Int16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Int32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Int64: MemoryModelItem{Align: 4, Size: 8, StructAlign: 4},

			Uint8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Uint16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Uint32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Uint64: MemoryModelItem{Align: 4, Size: 8, StructAlign: 4},

			Float32:  MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Float64:  MemoryModelItem{Align: 4, Size: 8, StructAlign: 4},
			Float128: MemoryModelItem{Align: 4, Size: 16, StructAlign: 4},

			Complex64:  MemoryModelItem{Align: 4, Size: 8, StructAlign: 4},
			Complex128: MemoryModelItem{Align: 4, Size: 16, StructAlign: 4},
			Complex256: MemoryModelItem{Align: 4, Size: 32, StructAlign: 4},

			Pointer:  MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Function: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
		}, nil

	case
		"amd64p32",
		"mips64p32",
		"mips64p32le":

		return MemoryModel{
			Int8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Int16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Int32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Int64: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},

			Uint8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Uint16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Uint32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Uint64: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},

			Float32:  MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Float64:  MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
			Float128: MemoryModelItem{Align: 8, Size: 16, StructAlign: 8},

			Complex64:  MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
			Complex128: MemoryModelItem{Align: 8, Size: 16, StructAlign: 8},
			Complex256: MemoryModelItem{Align: 8, Size: 32, StructAlign: 8},

			Pointer:  MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Function: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
		}, nil

	case
		"amd64",
		"arm64",
		"mips64",
		"mips64le",
		"ppc64",
		"sparc64":

		return MemoryModel{
			Int8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Int16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Int32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Int64: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},

			Uint8:  MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
			Uint16: MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
			Uint32: MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Uint64: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},

			Float32:  MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
			Float64:  MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
			Float128: MemoryModelItem{Align: 8, Size: 16, StructAlign: 8},

			Complex64:  MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
			Complex128: MemoryModelItem{Align: 8, Size: 16, StructAlign: 8},
			Complex256: MemoryModelItem{Align: 8, Size: 32, StructAlign: 8},

			Pointer:  MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
			Function: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
		}, nil
	default:
		return nil, fmt.Errorf("unknown or unsupported architecture %s", arch)
	}
}

// Alignof computes the memory alignment requirements of t. Zero is returned
// for a struct/union type with no fields.
func (m MemoryModel) Alignof(t Type) int {
	switch x := t.(type) {
	case *ArrayType:
		return mathutil.Max(1, m.Alignof(x.Item))
	case *StructOrUnionType:
		var r int
		for _, v := range x.Fields {
			if a := m.Alignof(v); a > r {
				r = a
			}
		}
		return mathutil.Max(1, r)
	default:
		item, ok := m[t.Kind()]
		if !ok {
			panic(fmt.Errorf("missing model item for %s", t.Kind()))
		}

		return int(item.Align)
	}
}

// Layout computes the memory layout of t.
func (m MemoryModel) Layout(t *StructOrUnionType) []FieldProperties {
	if len(t.Fields) == 0 {
		return nil
	}

	r := make([]FieldProperties, len(t.Fields))
	switch t.Kind() {
	case Struct:
		var off int64
		for i, v := range t.Fields {
			sz := m.Sizeof(v)
			a := m.StructAlignof(v)
			z := off
			if a != 0 {
				off = roundup(off, int64(a))
			}
			if off != z {
				r[i-1].Padding = int(off - z)
			}
			r[i] = FieldProperties{Offset: off, Size: sz}
			off += sz
		}
		z := off
		off = roundup(off, int64(m.Alignof(t)))
		if off != z {
			r[len(r)-1].Padding = int(off - z)
		}
	case Union:
		var sz int64
		for i, v := range t.Fields {
			n := m.Sizeof(v)
			r[i] = FieldProperties{Size: n}
			if n > sz {
				sz = n
			}
		}
		sz = roundup(sz, int64(m.Alignof(t)))
		for i, v := range r {
			r[i].Padding = int(sz - v.Size)
		}
	}
	return r
}

// Sizeof computes the memory size of t.
func (m MemoryModel) Sizeof(t Type) int64 {
	switch x := t.(type) {
	case *ArrayType:
		return m.Sizeof(x.Item) * x.Items
	case *StructOrUnionType:
		if len(x.Fields) == 0 {
			return 0
		}

		switch t.Kind() {
		case Struct:
			var off int64
			for _, v := range x.Fields {
				sz := m.Sizeof(v)
				a := m.StructAlignof(v)
				if a != 0 {
					off = roundup(off, int64(a))
				}
				off += sz
			}
			return roundup(off, int64(m.Alignof(t)))
		case Union:
			var sz int64
			for _, v := range x.Fields {
				if n := m.Sizeof(v); n > sz {
					sz = n
				}
			}
			return roundup(sz, int64(m.Alignof(t)))
		}
	default:
		item, ok := m[t.Kind()]
		if !ok {
			panic(fmt.Errorf("missing model item for %s", t.Kind()))
		}

		return int64(item.Size)
	}
	panic("internal error")
}

// StructAlignof computes the memory alignment requirements of t when its
// instance is a struct field. Zero is returned for a struct/union type with no
// fields.
func (m MemoryModel) StructAlignof(t Type) int {
	switch x := t.(type) {
	case *ArrayType:
		return m.StructAlignof(x.Item)
	case *StructOrUnionType:
		var r int
		for _, v := range x.Fields {
			if a := m.StructAlignof(v); a > r {
				r = a
			}
		}
		return r
	default:
		item, ok := m[t.Kind()]
		if !ok {
			panic(fmt.Errorf("missing model item for %s", t.Kind()))
		}

		return int(item.StructAlign)
	}
}

// FieldProperties describe a struct/union field.
type FieldProperties struct {
	Offset  int64 // Relative to start of the struct/union.
	Size    int64 // Field size for copying.
	Padding int   // Adjustment to enforce proper alignment.
}

// Sizeof returns the sum of f.Size and f.Padding.
func (f *FieldProperties) Sizeof() int64 { return f.Size + int64(f.Padding) }
