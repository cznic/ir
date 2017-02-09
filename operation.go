// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found v.s the LICENSE file.

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
	Pos() token.Position
	verify(*verifier) error
}

// AllocResult operation reserves evaluation stack space for a result of type
// TypeID.
type AllocResult struct {
	TypeID
	TypeName NameID
	token.Position
}

func (o *AllocResult) Pos() token.Position { return o.Position }

func (o *AllocResult) verify(v *verifier) error {
	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *AllocResult) String() string {
	return fmt.Sprintf("\t%-*s\t%v\t; %s %s", opw, "allocResult", o.TypeID, o.TypeName, o.Position)
}

// Argument pushes argument Index, or its address, to the evaluation stack.
type Argument struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

func (o *Argument) Pos() token.Position { return o.Position }

func (o *Argument) verify(v *verifier) error {
	args := v.typeCache.MustType(v.function.TypeID).(*FunctionType).Arguments
	if o.Index < 0 || o.Index >= len(args) {
		return fmt.Errorf("invalid argument index")
	}

	t := args[o.Index]
	if o.Address {
		t = t.Pointer()
	}
	if g, e := o.TypeID, t.ID(); g != e {
		return fmt.Errorf("expected type %s", e)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Argument) String() string {
	return fmt.Sprintf("\t%-*s\t%s%v, %v\t; %s", opw, "argument", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Arguments operation annotates that function results, if any, are allocated
// and a function pointer is at TOS. Evaluation of any function arguments
// follows.
type Arguments struct {
	token.Position
}

func (o *Arguments) Pos() token.Position { return o.Position }

func (o *Arguments) verify(v *verifier) error {
	if len(v.stack) == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	tid := v.stack[len(v.stack)-1]
	t := v.typeCache.MustType(tid)
	if t.Kind() != Pointer {
		return fmt.Errorf("expected a function pointer at TOS, got %s", tid)
	}

	t = t.(*PointerType).Element
	if t.Kind() != Function {
		return fmt.Errorf("expected a function pointer at TOS, got %s", t.ID())
	}

	results := t.(*FunctionType).Results
	if len(v.stack) < len(results)+1 {
		return fmt.Errorf("evaluation stack underflow")
	}

	for i, r := range results {
		// | #0 | fp |
		if g, e := v.stack[len(v.stack)-1-len(results)], r.ID(); g != e {
			return fmt.Errorf("mismatched result #%v, got %s, expected %s", i, g, e)
		}
	}

	return nil
}

func (o *Arguments) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "arguments", o.Position)
}

// BeginScope operation annotates entering a block scope.
type BeginScope struct {
	token.Position
}

func (o *BeginScope) Pos() token.Position { return o.Position }

func (o *BeginScope) verify(v *verifier) error {
	if len(v.stack) != 0 {
		return fmt.Errorf("non empty evaluation stack at scope begin")
	}

	v.blockLevel++
	return nil
}

func (o *BeginScope) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "beginScope", o.Position)
}

// Call operation performs a function call. The evaluation stack contains the
// space reseved for function results, is any, the function pointer and any
// function arguments.
type Call struct {
	Arguments int // Actual number of arguments passed to function.
	TypeID        // Type of the function pointer.
	token.Position
}

func (o *Call) Pos() token.Position { return o.Position }

func (o *Call) verify(v *verifier) error {
	if len(v.stack) < 1+o.Arguments {
		return fmt.Errorf("evaluation stack underflow")
	}

	fp := len(v.stack) - 1 - o.Arguments
	tid := v.stack[fp]
	t := v.typeCache.MustType(tid)
	if t.Kind() != Pointer {
		return fmt.Errorf("expected a function pointer before the function arguments, got %s", tid)
	}

	t = t.(*PointerType).Element
	if t.Kind() != Function {
		return fmt.Errorf("expected a function pointer before the function arguments, got %s", t.ID())
	}

	results := t.(*FunctionType).Results
	if len(v.stack) < len(results)+1+o.Arguments {
		return fmt.Errorf("evaluation stack underflow")
	}

	for i, r := range results {
		// | #0 | fp |
		if g, e := v.stack[fp-len(results)], r.ID(); g != e {
			return fmt.Errorf("mismatched result #%v, got %s, expected %s", i, g, e)
		}
	}

	args := t.(*FunctionType).Arguments
	for i, v := range v.stack[fp+1:] {
		if i >= len(args) {
			break
		}

		if g, e := args[i].ID(), v; g != e {
			return fmt.Errorf("invalid argument #%v type, expected %s", i, e)
		}
	}

	v.stack = v.stack[:fp]
	return nil
}

func (o *Call) String() string {
	return fmt.Sprintf("\t%-*s\t%v, %s\t; %s", opw, "call", o.Arguments, o.TypeID, o.Position)
}

// Drop operation removes one item from the evaluation stack.
type Drop struct {
	TypeID
	token.Position
}

func (o *Drop) Pos() token.Position { return o.Position }

func (o *Drop) verify(v *verifier) error {
	if len(v.stack) == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	v.stack = v.stack[:len(v.stack)-1]
	return nil
}

func (o *Drop) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "drop", o.TypeID, o.Position)
}

// EndScope operation annotates leaving a block scope.
type EndScope struct {
	token.Position
}

func (o *EndScope) Pos() token.Position { return o.Position }

func (o *EndScope) verify(v *verifier) error {
	if len(v.stack) != 0 {
		return fmt.Errorf("non empty evaluation stack at scope end")
	}

	if v.blockLevel == 0 {
		return fmt.Errorf("unbalanced end scope")
	}

	v.blockLevel--
	if v.blockLevel == 0 {
		if _, ok := v.function.Body[v.ip-1].(*Return); !ok {
			return fmt.Errorf("missing return before end of function")
		}
	}

	return nil
}

func (o *EndScope) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "endScope", o.Position)
}

// Extern operation pushes an external definition on the evaluation stack.
type Extern struct {
	Address bool
	Index   int // A negative value or object index as resolved by the linker.
	NameID
	TypeID
	TypeName NameID
	token.Position
}

func (o *Extern) Pos() token.Position { return o.Position }

func (o *Extern) verify(v *verifier) error {
	//TODO add context to v so that on .Index >= 0 the linker result can be verified.
	t := v.typeCache.MustType(o.TypeID)
	if o.Address && t.Kind() != Pointer {
		return fmt.Errorf("expected pointer type, have %s", o.TypeID)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Extern) String() string {
	s := ""
	if o.Index >= 0 {
		s = fmt.Sprintf("%v, ", o.Index)
	}
	return fmt.Sprintf("extern\t%-*s\t%s\t; %s %s", opw, s+addr(o.Address)+o.NameID.String(), o.TypeID, o.TypeName, o.Position)
}

// Int32Const operation pushes an int32 literal on the evaluation stack.
type Int32Const struct {
	Value int32
	token.Position
}

func (o *Int32Const) Pos() token.Position { return o.Position }

func (o *Int32Const) verify(v *verifier) error {
	v.stack = append(v.stack, TypeID(idInt32))
	return nil
}

func (o *Int32Const) String() string {
	return fmt.Sprintf("\t%-*s\t%v, int32\t; %s", opw, "const", o.Value, o.Position)
}

// Panic operation aborts execution with a stack trace.
type Panic struct {
	token.Position
}

func (o *Panic) Pos() token.Position { return o.Position }

func (o *Panic) verify(v *verifier) error { return nil }

func (o *Panic) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "panic", o.Position)
}

// Return operation removes all function call arguments from the evaluation
// stack as well as the function pointer used v.s the call.
type Return struct {
	token.Position
}

func (o *Return) Pos() token.Position { return o.Position }

func (o *Return) verify(v *verifier) error {
	if len(v.stack) != 0 {
		return fmt.Errorf("non empty evaluation stack on return: %v", v.stack)
	}

	return nil
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

func (o *Result) Pos() token.Position { return o.Position }

func (o *Result) verify(v *verifier) error {
	results := v.typeCache.MustType(v.function.TypeID).(*FunctionType).Results
	if o.Index < 0 || o.Index >= len(results) {
		return fmt.Errorf("invalid result index")
	}

	t := results[o.Index]
	if o.Address {
		t = t.Pointer()
	}
	if g, e := o.TypeID, t.ID(); g != e {
		return fmt.Errorf("expected type %s", e)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Result) String() string {
	return fmt.Sprintf("\t%-*s\t%s%v, %v\t; %s", opw, "result", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Store operation stores a TOS value at address v.s the preceding stack
// position.  The address is removed from the evaluation stack.
type Store struct {
	TypeID
	token.Position
}

func (o *Store) Pos() token.Position { return o.Position }

func (o *Store) verify(v *verifier) error {
	if len(v.stack) < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	p := len(v.stack) - 2
	tid := v.stack[p]
	pt := v.typeCache.MustType(tid)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected pointer and value at TOS, got %s and %s", tid, v.stack[p+1])
	}

	if g, e := pt.(*PointerType).Element.ID(), v.stack[p+1]; g != e {
		return fmt.Errorf("mismatched address and value type: %s and %s", g, e)
	}

	v.stack = append(v.stack[:p], v.stack[p+1])
	return nil
}

func (o *Store) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "store", o.TypeID, o.Position)
}

// StringConst operation pushes a string literal on the evaluation stack.
type StringConst struct {
	Value StringID
	token.Position
}

func (o *StringConst) Pos() token.Position { return o.Position }

func (o *StringConst) verify(v *verifier) error {
	v.stack = append(v.stack, TypeID(idInt8Ptr))
	return nil
}

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

func (o *Variable) Pos() token.Position { return o.Position }

func (o *Variable) verify(v *verifier) error {
	if o.Index < 0 || o.Index >= len(v.variables) {
		return fmt.Errorf("invalid variable index")
	}

	t := v.typeCache.MustType(v.variables[o.Index])
	if o.Address {
		t = t.Pointer()
	}
	if g, e := o.TypeID, t.ID(); g != e {
		return fmt.Errorf("expected type %s", e)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

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

func (o *VariableDeclaration) Pos() token.Position { return o.Position }

func (o *VariableDeclaration) verify(v *verifier) error {
	v.variables = append(v.variables, o.TypeID)
	return nil
}

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
