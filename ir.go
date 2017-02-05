// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ir implements intermediate representation of compiled programs.
// (Work In Progress)
//
// See: https://en.wikipedia.org/wiki/Intermediate_representation
package ir

import (
	"go/token"
)

var (
	_ Object = (*Declaration)(nil)
	_ Object = (*FunctionDefinition)(nil)
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
	object()
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

func (ObjectBase) object() {}

// Declaration represents a variable declaration/definition or a function
// declaration.
type Declaration struct {
	ObjectBase
	Value
}

// NewDeclaration returns a newly created Declaration.
func NewDeclaration(p token.Position, name, typeName NameID, typ TypeID, l Linkage, initializer Value) *Declaration {
	return &Declaration{
		ObjectBase: newObjectBase(p, name, typeName, typ, l),
		Value:      initializer,
	}
}

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
