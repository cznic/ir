// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
	"go/token"
)

const opw = 16

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

func (o *AllocResult) String() string {
	return fmt.Sprintf("\t%-*s\t%v\t; %s %s", opw, "allocResult", o.TypeID, o.TypeName, o.Position)
}

// Argument pushes a function argument by index, or its address, to the
// evaluation stack.
type Argument struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Argument) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

func (o *Argument) String() string {
	return fmt.Sprintf("\t%-*s\t%s%v, %v\t; %s", opw, "argument", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Arguments operation annotates that function results, if any are allocated
// and function call arguments evaluation starts.
type Arguments struct {
	token.Position
}

func (o *Arguments) verify(in []TypeID) ([]TypeID, error) { return in, nil }

func (o *Arguments) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "arguments", o.Position)
}

// BeginScope operation annotates entering a block scope.
type BeginScope struct {
	token.Position
}

func (o *BeginScope) verify(in []TypeID) ([]TypeID, error) {
	if len(in) != 0 {
		return nil, fmt.Errorf("non empty evaluation stack at scope begin")
	}

	return nil, nil
}

func (o *BeginScope) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "beginScope", o.Position)
}

// Call operation executes a call through the function pointer on top of the
// evaluation stack.
type Call struct {
	Arguments int // Actual number of arguments passed to function.
	TypeID
	token.Position
}

func (o *Call) verify(in []TypeID) ([]TypeID, error) {
	if len(in) < o.Arguments+1 {
		return nil, fmt.Errorf("evaluation stack underflow")
	}

	return in[:len(in)-o.Arguments-1], nil
}

func (o *Call) String() string {
	return fmt.Sprintf("\t%-*s\t%v, %s\t; %s", opw, "call", o.Arguments, o.TypeID, o.Position)
}

// Drop operation removes one item from the evaluation stack.
type Drop struct {
	TypeID
	token.Position
}

func (o *Drop) verify(in []TypeID) ([]TypeID, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("evaluation stack underflow")
	}

	return in[:len(in)-1], nil
}

func (o *Drop) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "drop", o.TypeID, o.Position)
}

// EndScope operation annotates leaving a block scope.
type EndScope struct {
	token.Position
}

func (o *EndScope) verify(in []TypeID) ([]TypeID, error) {
	if len(in) != 0 {
		return nil, fmt.Errorf("non empty evaluation stack at scope end")
	}

	return nil, nil
}

func (o *EndScope) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "endScope", o.Position)
}

// Extern operation pushes an external definition on the evaluation stack.
type Extern struct {
	Address bool
	NameID
	TypeID
	TypeName NameID
	token.Position
}

func (o *Extern) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

func (o *Extern) String() string {
	return fmt.Sprintf("extern\t%-*s\t%s\t; %s %s", opw, addr(o.Address)+o.NameID.String(), o.TypeID, o.TypeName, o.Position)
}

// Int32Const operation pushes an int32 literal on the evaluation stack.
type Int32Const struct {
	Value int32
	token.Position
}

func (o *Int32Const) verify(in []TypeID) ([]TypeID, error) { return append(in, TypeID(idInt32)), nil }

func (o *Int32Const) String() string {
	return fmt.Sprintf("\t%-*s\t%v, int32\t; %s", opw, "const", o.Value, o.Position)
}

// Return operation removes all function call arguments from the evaluation
// stack as well as the function pointer used in the call.
type Return struct {
	token.Position
}

func (o *Return) verify(in []TypeID) ([]TypeID, error) {
	if len(in) != 0 {
		return nil, fmt.Errorf("non empty evaluation stack on return: %v", in)
	}

	return nil, nil
}

func (o *Return) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "return", o.Position)
}

// Result pushes a function result by index, or its address, to the evaluation
// stack.
type Result struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Result) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

func (o *Result) String() string {
	return fmt.Sprintf("\t%-*s\t%s%v, %v\t; %s", opw, "result", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Store operation stores a TOS value at address in the next stack position.
// The address is removed from the evaluation stack.
type Store struct {
	TypeID
	token.Position
}

func (o *Store) verify(in []TypeID) ([]TypeID, error) {
	if len(in) < 2 {
		return nil, fmt.Errorf("evaluation stack underflow")
	}

	return in[:len(in)-1], nil
}

func (o *Store) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "store", o.TypeID, o.Position)
}

// Int32Const operation pushes a string literal on the evaluation stack.
type StringConst struct {
	Value StringID
	token.Position
}

func (o *StringConst) verify(in []TypeID) ([]TypeID, error) { return append(in, TypeID(idInt8Ptr)), nil }

func (o *StringConst) String() string {
	return fmt.Sprintf("\t%-*s\t%q, %s\t; %s", opw, "const", o.Value, dict.S(idInt8Ptr), o.Position)
}

// Variable pushes a function local variable by index, or its address, to the
// evaluation stack.
type Variable struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Variable) verify(in []TypeID) ([]TypeID, error) { return append(in, o.TypeID), nil }

func (o *Variable) String() string {
	return fmt.Sprintf("\t%-*s\t%s%v, %v\t; %s", opw, "variable", addr(o.Address), o.Index, o.TypeID, o.Position)
}

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

func (o *VariableDeclaration) String() string {
	var s string
	switch {
	case o.Value != nil:
		s = fmt.Sprintf("(%v)(%v)", o.TypeID, o.Value)
	default:
		s = fmt.Sprintf("%v", o.TypeID)
	}
	return fmt.Sprintf("\t%-*s\t%v, %s, %s\t; %s %s", opw, "varDecl", o.Index, o.NameID, s, o.TypeName, o.Position)
}
