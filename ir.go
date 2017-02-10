// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ir implements intermediate representation of compiled programs.
// (Work In Progress)
//
// See: https://en.wikipedia.org/wiki/Intermediate_representation
package ir

import (
	"fmt"
	"go/token"
)

var (
	_ Object = (*DataDefinition)(nil)
	_ Object = (*FunctionDefinition)(nil)

	// Testing amends things for tests.
	Testing bool
)

// NameID is a numeric identifier of an identifier as registered in a global
// dictionary[0].
//
//  [0]: https://godoc.org/github.com/cznic/xc#pkg-variables
type NameID int

// String implements fmt.Stringer.
func (t NameID) String() string { return string(dict.S(int(t))) }

// GobDecode implements GobDecoder.
func (t *NameID) GobDecode(b []byte) error {
	*t = NameID(dict.ID(b))
	return nil
}

// GobEncode implements GobEncoder.
func (t NameID) GobEncode() ([]byte, error) {
	return append([]byte(nil), dict.S(int(t))...), nil
}

// StringID is a numeric identifier of a string literal as registered in a
// global dictionary[0].
//
//  [0]: https://godoc.org/github.com/cznic/xc#pkg-variables
type StringID int

// String implements fmt.Stringer.
func (t StringID) String() string { return string(dict.S(int(t))) }

// GobDecode implements GobDecoder.
func (t *StringID) GobDecode(b []byte) error {
	*t = StringID(dict.ID(b))
	return nil
}

// GobEncode implements GobEncoder.
func (t StringID) GobEncode() ([]byte, error) {
	return append([]byte(nil), dict.S(int(t))...), nil
}

// Object represents a declarations or definitions of static data and functions.
type Object interface {
	Verify() error
}

// ObjectBase collects fields common to all objects.
type ObjectBase struct {
	token.Position
	NameID
	TypeID
	Linkage
	TypeName NameID
}

func newObjectBase(p token.Position, nm, tnm NameID, typ TypeID, l Linkage) ObjectBase {
	return ObjectBase{
		Position: p,
		NameID:   nm,
		TypeID:   typ,
		Linkage:  l,
		TypeName: tnm,
	}
}

// DataDefinition represents a variable definition and an optional initializer
// value.
type DataDefinition struct {
	ObjectBase
	Value
}

// NewDataDefinition returns a newly created DataDefinition.
func NewDataDefinition(p token.Position, name, typeName NameID, typ TypeID, l Linkage, initializer Value) *DataDefinition {
	return &DataDefinition{
		ObjectBase: newObjectBase(p, name, typeName, typ, l),
		Value:      initializer,
	}
}

// Verify implements Object.
func (d *DataDefinition) Verify() error { return nil }

// FunctionDefinition represents a function definition.
type FunctionDefinition struct {
	Arguments []NameID // May be nil.
	Body      []Operation
	ObjectBase
	Results []NameID // May be nil.
}

// NewFunctionDefinition returns a newly created FunctionDefinition.
func NewFunctionDefinition(p token.Position, name, typeName NameID, typ TypeID, l Linkage, argumnents, results []NameID) *FunctionDefinition {
	return &FunctionDefinition{
		Arguments:  argumnents,
		ObjectBase: newObjectBase(p, name, typeName, typ, l),
		Results:    results,
	}
}

// Verify implements Object.
func (f *FunctionDefinition) Verify() (err error) {
	switch len(f.Body) {
	case 0:
		return fmt.Errorf("function body cannot be empty")
	case 1:
		switch f.Body[0].(type) {
		case *Return, *Panic:
			return nil
		}

		return fmt.Errorf("invalid operation")
	}

	vv := &verifier{
		function:  f,
		labels:    map[int]struct{}{},
		typeCache: TypeCache{},
	}
	var v Operation
	ignore := false
	for vv.ip, v = range f.Body {
		vv.stack = append([]TypeID(nil), vv.stack...)
		if !ignore {
			if err = v.verify(vv); err != nil {
				return fmt.Errorf("%v\n%s:%#x: %v", err, f.NameID, vv.ip, v)
			}
		}
		switch v.(type) {
		case *Panic:
			ignore = true
		case *Label:
			if ignore {
				if err = v.verify(vv); err != nil {
					return fmt.Errorf("%v\n%s:%#x: %v", err, f.NameID, vv.ip, v)
				}
			}
			ignore = false
		case *BeginScope:
			if ignore {
				vv.blockLevel++
			}
		case *EndScope:
			if ignore {
				vv.blockLevel--
			}
		}
	}
	if vv.blockLevel != 0 {
		return fmt.Errorf("unbalanced BeginScope/EndScope")
	}

	for ip, v := range f.Body {
		switch x := v.(type) {
		case *Jmp:
			n := int(x.NameID)
			if n == 0 {
				n = x.Number
			}

			if _, ok := vv.labels[n]; !ok {
				return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, v)
			}
		case *Jnz:
			n := int(x.NameID)
			if n == 0 {
				n = x.Number
			}

			if _, ok := vv.labels[n]; !ok {
				return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, v)
			}
		case *Jz:
			n := int(x.NameID)
			if n == 0 {
				n = x.Number
			}

			if _, ok := vv.labels[n]; !ok {
				return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, v)
			}
		}
	}
	return nil
}

type verifier struct {
	blockLevel int
	function   *FunctionDefinition
	ip         int
	labels     map[int]struct{}
	stack      []TypeID
	typeCache  TypeCache
	variables  []TypeID
}

func (v *verifier) binop() error {
	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	a, b := v.stack[n-2], v.stack[n-1]
	if a != b {
		return fmt.Errorf("mismatched operand types: %s and %s", a, b)
	}

	v.stack = append(v.stack[:len(v.stack)-2], a)
	return nil
}

func (v *verifier) relop() error {
	if err := v.binop(); err != nil {
		return err
	}

	v.stack[len(v.stack)-1] = TypeID(idInt32)
	return nil
}

func (v *verifier) branch() error {
	n := len(v.stack)
	if n < 1 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], TypeID(idInt32); g != e {
		return fmt.Errorf("unexpected branch stack item of type %s (expected %s)", g, e)
	}

	v.stack = v.stack[:n-1]
	return nil
}
