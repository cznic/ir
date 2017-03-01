// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"

	"github.com/cznic/internal/buffer"
)

var (
	_ Value = (*AddressValue)(nil)
	_ Value = (*CompositeValue)(nil)
	_ Value = (*DesignatedValue)(nil)
	_ Value = (*Float64Value)(nil)
	_ Value = (*Int32Value)(nil)
	_ Value = (*Int64Value)(nil)
	_ Value = (*StringValue)(nil)
)

type valuer struct{}

func (valuer) value() {}

// Value represents a constant expression used for initializing static data or
// function variables.
type Value interface {
	value()
}

// AddressValue is a declaration initializer constant of type address. Its
// final value is determined by the linker/loader.
type AddressValue struct {
	valuer
	Index int // A negative value or object index as resolved by the linker.
	Linkage
	NameID
}

func (v *AddressValue) String() string {
	switch v.Linkage {
	case InternalLinkage:
		return fmt.Sprintf("%v, &%v", v.Index, v.NameID)
	case ExternalLinkage:
		return fmt.Sprintf("extern %v, &%v", v.Index, v.NameID)
	default:
		panic("internal error")
	}
}

// CompositeValue represents a constant array/struct initializer.
type CompositeValue struct {
	valuer
	Values []Value
}

func (v *CompositeValue) String() string {
	var b buffer.Bytes
	b.WriteByte('{')
	for i, v := range v.Values {
		if i != 0 {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprint(&b, v)
	}
	b.WriteByte('}')
	return string(b.Bytes())
}

// DesignatedValue represents the value of a particular array element or a
// particular struct field.
type DesignatedValue struct {
	Index int // Array index or field index.
	Value
}

func (v *DesignatedValue) String() string { return fmt.Sprintf("%v: %v", v.Index, v.Value) }

// Float64Value is a declaration initializer constant of type float64.
type Float64Value struct {
	valuer
	Value float64
}

func (v *Float64Value) String() string { return fmt.Sprint(v.Value) }

// Int32Value is a declaration initializer constant of type int32.
type Int32Value struct {
	valuer
	Value int32
}

func (v *Int32Value) String() string { return fmt.Sprint(v.Value) }

// Int64Value is a declaration initializer constant of type int64.
type Int64Value struct {
	valuer
	Value int64
}

func (v *Int64Value) String() string { return fmt.Sprint(v.Value) }

// StringValue is a declaration initializer constant of type string.
type StringValue struct {
	valuer
	StringID
}

func (v *StringValue) String() string { return fmt.Sprintf("%q", v.StringID) }
