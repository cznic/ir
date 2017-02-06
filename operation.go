// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/token"
)

var (
	_ Operation = (*AllocResult)(nil)
	_ Operation = (*Argument)(nil)
	_ Operation = (*Arguments)(nil)
	_ Operation = (*BeginScope)(nil)
	_ Operation = (*Call)(nil)
	_ Operation = (*Drop)(nil)
	_ Operation = (*EndScope)(nil)
	_ Operation = (*Extern)(nil)
	_ Operation = (*Int32Const)(nil)
	_ Operation = (*Result)(nil)
	_ Operation = (*Return)(nil)
	_ Operation = (*Store)(nil)
	_ Operation = (*StringConst)(nil)
	_ Operation = (*Variable)(nil)
	_ Operation = (*VariableDeclaration)(nil)
)

// Operation is a unit of execution.
type Operation interface {
	verify([]TypeID) ([]TypeID, error)
}

// AllocResult operation reserves evaluation stack space for a result of type
// TypeID.
type AllocResult struct {
	TypeID
	TypeName NameID
	token.Position
}

func (o *AllocResult) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

// Argument pushes a function argument by index, or its address, to the
// evaluation stack.
type Argument struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Argument) verify(in []TypeID) ([]TypeID, error) { return in, nil }

// Arguments operation annotates that function results, if any are allocated
// and function call arguments evaluation starts.
type Arguments struct {
	token.Position
}

func (o *Arguments) verify(in []TypeID) ([]TypeID, error) { return in, nil }

// BeginScope operation annotates entering a block scope.
type BeginScope struct {
	token.Position
}

func (o *BeginScope) verify(in []TypeID) ([]TypeID, error) { return in, nil }

// Call operation executes a call through the function pointer on top of the
// evaluation stack.
type Call struct {
	TypeID
	token.Position
}

func (o *Call) verify(in []TypeID) ([]TypeID, error) { panic("TODO") }

// Drop operation removes one item from the evaluation stack.
type Drop struct {
	TypeID
	token.Position
}

func (o *Drop) verify(in []TypeID) ([]TypeID, error) { panic("TODO") }

// Extern operation pushes an external definition on the evaluation stack.
type Extern struct {
	Address bool
	NameID
	TypeID
	TypeName NameID
	token.Position
}

func (o *Extern) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

// EndScope operation annotates leaving a block scope.
type EndScope struct {
	token.Position
}

func (o *EndScope) verify(in []TypeID) ([]TypeID, error) { return in, nil }

// Int32Const operation pushes an int32 literal on the evaluation stack.
type Int32Const struct {
	Value int32
	token.Position
}

func (o *Int32Const) verify(in []TypeID) ([]TypeID, error) { return append(in, TypeID(idInt32)), nil }

// Return operation removes all function call arguments from the evaluation
// stack as well as the function pointer used in the call.
type Return struct {
	token.Position
}

func (o *Return) verify(in []TypeID) ([]TypeID, error) { return in, nil }

// Result pushes a function result by index, or its address, to the evaluation
// stack.
type Result struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Result) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

// Store operation stores a TOS value at address in the next stack position.
// The address is removed from the evaluation stack.
type Store struct {
	token.Position
}

func (o *Store) verify(in []TypeID) ([]TypeID, error) { panic("TODO") }

// Int32Const operation pushes a string literal on the evaluation stack.
type StringConst struct {
	Value StringID
	token.Position
}

func (o *StringConst) verify(in []TypeID) ([]TypeID, error) { return append(in, TypeID(idInt8Ptr)), nil }

// Variable pushes a function local variable by index, or its address, to the
// evaluation stack.
type Variable struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Variable) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

// VariableDeclaration operation declares a function local variable. NameID,
// TypeName and Value are all optional.
type VariableDeclaration struct {
	Index int // 0-based index within a function.
	NameID
	TypeID
	TypeName NameID
	Value
	token.Position
}

func (o *VariableDeclaration) verify(in []TypeID) ([]TypeID, error) { return in, nil }
