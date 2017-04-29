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
	// Verify checks if the object is well-formed. Verify may mutate the
	// object. For example, Verify may remove provably unreachable code of
	// a FunctionDefinition.Body.
	Verify() error
	Base() *ObjectBase
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

// Base implements Object.
func (o *ObjectBase) Base() *ObjectBase { return o }

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

	unconvert(&f.Body)
	ver := &verifier{
		function:  f,
		labels:    map[int]int{},
		typeCache: TypeCache{},
	}
	var op Operation
	for ver.ip, op = range f.Body {
		switch x := op.(type) {
		case *BeginScope:
			ver.blockLevel++
		case *EndScope:
			if ver.blockLevel == 0 {
				return fmt.Errorf("unbalanced end scope\n%s:%#x: %v", f.NameID, ver.ip, op)
			}

			ver.blockLevel--
			if ver.blockLevel == 0 {
				if _, ok := f.Body[ver.ip-1].(*Return); !ok {
					return fmt.Errorf("missing return before end of function\n%s:%#x: %v", f.NameID, ver.ip, op)
				}
			}

		case *Label:
			n := -int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			if _, ok := ver.labels[n]; ok {
				return fmt.Errorf("label redefined\n%s:%#x: %v", f.NameID, ver.ip, op)
			}

			ver.labels[n] = ver.ip
		case *VariableDeclaration:
			if g, e := x.Index, len(ver.variables); g != e {
				return fmt.Errorf("invalid variable declaration operation index, got %v, expected %v", g, e)
			}

			ver.variables = append(ver.variables, x.TypeID)
		}
	}

	if ver.blockLevel != 0 {
		return fmt.Errorf("unbalanced BeginScope/EndScope")
	}

	computedGotos := false
	for ip, op := range f.Body {
		var nm NameID
		var num int
		switch x := op.(type) {
		case *Jmp:
			nm, num = x.NameID, x.Number
		case *Jnz:
			nm, num = x.NameID, x.Number
		case *Jz:
			nm, num = x.NameID, x.Number
		case *JmpP:
			computedGotos = true
			continue
		case *Switch:
			for _, v := range x.Labels {
				nm, num = v.NameID, v.Number
				n := -int(nm)
				if n == 0 {
					n = num
				}
				if _, ok := ver.labels[n]; !ok {
					return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, op)
				}
			}
			continue
		default:
			continue
		}

		n := -int(nm)
		if n == 0 {
			n = num
		}
		if _, ok := ver.labels[n]; !ok {
			return fmt.Errorf("undefined branch target\n%s:%#x: %v", f.NameID, ip, op)
		}
	}

	p := buffer.CGet(len(f.Body))
	ipFlags := *p

	defer buffer.Put(p)

	phi := map[int][]TypeID{}
	var g func(int, []TypeID) error
	g = func(ip int, stack []TypeID) error {
		for {
			//fmt.Printf("# %#05x %v ; %v\n", ip, stack, f.Body[ip].Pos())
			op := f.Body[ip]
			if ipFlags[ip] != 0 {
				switch ex, ok := phi[ip]; {
				case ok:
					if g, e := len(stack), len(ex); g != e {
						return fmt.Errorf("evaluation stacks depth differs %v %v\n%s:%#x: %v", stack, ex, f.NameID, ip, op)
					}

					for i, v := range stack {
						if g, e := v, ex[i]; g != e && !ver.assignable(g, e) {
							return fmt.Errorf("evaluation stacks differ %v %v\n%s:%#x: %v", stack, ex, f.NameID, ip, v)
						}
					}

					return nil
				default:
					panic("internal error")
				}
			}

			ipFlags[ip] = 1

			ver.ip = ip
			ver.stack = stack
			if err := f.Body[ip].verify(ver); err != nil {
				return fmt.Errorf("%s\n%s:%#x: %v", err, f.NameID, ip, op)
			}

			stack = ver.stack
		outer:
			switch x := f.Body[ip].(type) {
			case *Jmp:
				n := -int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				ip = ver.labels[n]
				continue
			case *Switch:
				for _, v := range x.Labels {
					n := -int(v.NameID)
					if n == 0 {
						n = v.Number
					}
					if err := g(ver.labels[n], append([]TypeID(nil), stack...)); err != nil {
						return err
					}
				}
				n := -int(x.Default.NameID)
				if n == 0 {
					n = x.Default.Number
				}
				ip = ver.labels[n]
				continue
			case *Jnz:
				n := -int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				if y, ok := f.Body[ip-1].(*Const32); ok {
					switch {
					case y.Value != 0: // Always taken.
						ipFlags[ip-1] = 0
						f.Body[ip] = &Jmp{NameID: x.NameID, Number: x.Number, Position: x.Position}
						ip = ver.labels[n]
						continue
					default: // Never taken.
						ipFlags[ip-1] = 0
						ipFlags[ip] = 0
						break outer
					}
				}

				if err := g(ver.labels[n], append([]TypeID(nil), stack...)); err != nil {
					return err
				}
			case *Jz:
				n := -int(x.NameID)
				if n == 0 {
					n = x.Number
				}
				if y, ok := f.Body[ip-1].(*Const32); ok {
					switch {
					case y.Value == 0: // Always taken.
						ipFlags[ip-1] = 0
						f.Body[ip] = &Jmp{NameID: x.NameID, Number: x.Number, Position: x.Position}
						ip = ver.labels[n]
						continue
					default: // Never taken.
						ipFlags[ip-1] = 0
						ipFlags[ip] = 0
						break outer
					}
				}

				if err := g(ver.labels[n], append([]TypeID(nil), stack...)); err != nil {
					return err
				}
			case *Label:
				phi[ip] = append([]TypeID(nil), stack...)
			case *Return, *Panic:
				return nil
			}
			ip++
		}
	}
	if err := g(0, nil); err != nil {
		return err
	}

	if computedGotos {
		for k, v := range ver.labels {
			if k >= 0 {
				continue
			}

			if err := g(v, phi[v]); err != nil {
				return err
			}
		}
	}

	w := 0
	for ip, op := range f.Body {
		switch op.(type) {
		case *BeginScope, *EndScope, *VariableDeclaration, *Return:
			// nop
		default:
			if ipFlags[ip] == 0 {
				continue
			}
		}
		f.Body[w] = op
		w++
	}
	f.Body = f.Body[:w]
	return nil
}

type verifier struct {
	blockLevel      int
	blockValueLevel int
	function        *FunctionDefinition
	ip              int
	labels          map[int]int // nm (<0) or num (>=0): ip
	stack           []TypeID
	typeCache       TypeCache
	variables       []TypeID
}

func (v *verifier) validPtrBinop(a, b TypeID) bool {
	if v.assignable(a, b) {
		return true
	}

	t := v.typeCache.MustType(a)
	u := v.typeCache.MustType(b)
	if t.Kind() != Pointer && u.Kind() == Pointer {
		t, u = u, t
	}
	if t.Kind() != Pointer {
		return false
	}

	switch t.Kind() {
	case Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return true
	}

	return false
}

func (v *verifier) binop(t TypeID) error {
	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	a, b := v.stack[n-2], v.stack[n-1]
	if a != b && !v.validPtrBinop(a, b) {
		return fmt.Errorf("mismatched operand types: %s and %s", a, b)
	}

	if g, e := a, t; t != 0 && g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched operands types vs result type: %s and %s", g, e)
	}

	v.stack = append(v.stack[:n-2], a)
	return nil
}

func (v *verifier) unop(int bool) error {
	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	a := v.stack[n-1]
	switch v.typeCache.MustType(a).Kind() {
	case
		Int8,
		Int16,
		Int32,
		Int64,

		Uint8,
		Uint16,
		Uint32,
		Uint64:

		// ok
	case
		Float32,
		Float64,
		Float128:

		if int {
			return fmt.Errorf("invalid operand type: %s ", a)
		}
	default:
		return fmt.Errorf("invalid operand type: %s ", a)
	}

	return nil
}

func (v *verifier) relop(t TypeID) error {
	if err := v.binop(0); err != nil {
		return err
	}

	v.stack[len(v.stack)-1] = idInt32
	return nil
}

func (v *verifier) branch() error {
	n := len(v.stack)
	if n < 1 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], idInt32; g != e {
		return fmt.Errorf("unexpected branch stack item of type %s (expected %s)", g, e)
	}

	v.stack = v.stack[:n-1]
	return nil
}

func (v *verifier) assignable(a, b TypeID) bool {
	if a == b {
		return true
	}

	t := v.typeCache.MustType(a)
	u := v.typeCache.MustType(b)
	if t.Kind() == Function {
		t = t.Pointer()
	}
	if u.Kind() == Function {
		u = u.Pointer()
	}

	for t.Kind() == Pointer && u.Kind() == Pointer {
		if v.isVoidPtr(a) || v.isVoidPtr(b) {
			return true
		}

		t = t.(*PointerType).Element
		u = u.(*PointerType).Element
		if t.Kind() == Function && u.Kind() == Function {
			a := t.(*FunctionType)
			b := u.(*FunctionType)
			at := a.Results
			bt := b.Results
			if len(at) != len(bt) {
				return false
			}

			for i, r := range at {
				if !v.assignable(r.ID(), bt[i].ID()) {
					return false
				}
			}

			return true
		}
	}

	return false
}

func (v *verifier) isPtr(t TypeID) bool {
	u := v.typeCache.MustType(t)
	return u.Kind() == Pointer
}

func (v *verifier) isVoidPtr(t TypeID) bool {
	u := v.typeCache.MustType(t)
	for u.Kind() == Pointer {
		if u.ID() == idVoidPtr {
			return true
		}

		u = u.(*PointerType).Element
	}
	return false
}
