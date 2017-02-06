// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// Linkage represents a linkage type.
type Linkage int

// Linkage values.
const (
	_ Linkage = iota

	ExternalLinkage
	InternalLinkage
)

// TypeKind represents a particular type kind.
type TypeKind int

// TypeKind values.
const (
	_ TypeKind = iota

	Int8
	Int16
	Int32
	Int64

	Uint8
	Uint16
	Uint32
	Uint64

	Float32
	Float64
	Float128

	Complex64
	Complex128
	Complex256

	Array
	Union
	Struct
	Pointer
	Function
)

// Kind implements Type.
func (k TypeKind) Kind() TypeKind { return k }

type tok int

const (
	_ tok = iota + 0x7f

	tokI8
	tokI16
	tokI32
	tokI64

	tokU8
	tokU16
	tokU32
	tokU64

	tokF32
	tokF64
	tokF128

	tokC64
	tokC128
	tokC256

	tokEllipsis
	tokFunc
	tokNumber
	tokStruct
	tokUnion

	tokEOF
	tokIllegal
)
