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

	"github.com/cznic/internal/buffer"
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
		labels:    map[int]int{},
		typeCache: TypeCache{},
	}
	var v Operation
	for vv.ip, v = range f.Body {
		switch x := v.(type) {
		case *BeginScope:
			vv.blockLevel++
		case *EndScope:
			vv.blockLevel--
		case *Label:
			n := int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			if _, ok := vv.labels[n]; ok {
				return fmt.Errorf("label redefined\n%s:%#x: %v", f.NameID, vv.ip, v)
			}

			vv.labels[n] = vv.ip
		}
	}

	if vv.blockLevel != 0 {
		return fmt.Errorf("unbalanced BeginScope/EndScope")
	}

	for ip, v := range f.Body {
		var nm NameID
		var num int
		switch x := v.(type) {
		case *Jmp:
			nm, num = x.NameID, x.Number
		case *Jnz:
			nm, num = x.NameID, x.Number
		case *Jz:
			nm, num = x.NameID, x.Number
		default:
			continue
		}

		n := int(nm)
		if n == 0 {
			n = num
		}
		if _, ok := vv.labels[n]; !ok {
			return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, v)
		}
	}

	return nil //TODO-
	p := buffer.CGet(len(f.Body))
	visited := *p

	defer buffer.Put(p)

	phi := map[int][]TypeID{}
	var g func(int, []TypeID) error
	g = func(ip int, stack []TypeID) error {
		for ; ; ip++ {
			if visited[ip] != 0 {
				switch ex, ok := phi[ip]; {
				case ok:
					if g, e := len(stack), len(ex); g != e {
						return fmt.Errorf("eval stack depth differs\n%s:%#x: %v", f.NameID, ip, v)
					}
				default:
					phi[ip] = append([]TypeID(nil), stack...)
				}
				return nil
			}

			visited[ip] = 1

			vv.ip = ip
			if err := f.Body[ip].verify(vv); err != nil {
				return fmt.Errorf("%s\n%s:%#x: %v", err, f.NameID, ip, v)
			}

			switch x := f.Body[ip].(type) {
			case *Jmp:
				n := int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				ip = vv.labels[n]
				continue
			case *Jnz:
				n := int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				if err := g(vv.labels[n], append([]TypeID(nil), stack...)); err != nil {
					return err
				}
			case *Jz:
				n := int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				if err := g(vv.labels[n], append([]TypeID(nil), stack...)); err != nil {
					return err
				}
			case *Return, *Panic:
				return nil
			}
		}
	}
	return g(0, nil)
}

type verifier struct {
	blockLevel int
	function   *FunctionDefinition
	ip         int
	labels     map[int]int
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
