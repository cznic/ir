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
	_ Operation = (*Add)(nil)
	_ Operation = (*AllocResult)(nil)
	_ Operation = (*Argument)(nil)
	_ Operation = (*Arguments)(nil)
	_ Operation = (*BeginScope)(nil)
	_ Operation = (*Bool)(nil)
	_ Operation = (*Call)(nil)
	_ Operation = (*Drop)(nil)
	_ Operation = (*Dup)(nil)
	_ Operation = (*Element)(nil)
	_ Operation = (*EndScope)(nil)
	_ Operation = (*Eq)(nil)
	_ Operation = (*Extern)(nil)
	_ Operation = (*Field)(nil)
	_ Operation = (*Int32Const)(nil)
	_ Operation = (*Jmp)(nil)
	_ Operation = (*Jnz)(nil)
	_ Operation = (*Jz)(nil)
	_ Operation = (*Label)(nil)
	_ Operation = (*Leq)(nil)
	_ Operation = (*Load)(nil)
	_ Operation = (*Lt)(nil)
	_ Operation = (*Mul)(nil)
	_ Operation = (*Panic)(nil)
	_ Operation = (*PostIncrement)(nil)
	_ Operation = (*Result)(nil)
	_ Operation = (*Return)(nil)
	_ Operation = (*Store)(nil)
	_ Operation = (*StringConst)(nil)
	_ Operation = (*Sub)(nil)
	_ Operation = (*Variable)(nil)
	_ Operation = (*VariableDeclaration)(nil)
)

// Operation is a unit of execution.
type Operation interface {
	Pos() token.Position
	verify(*verifier) error
}

// Add operation subtracts the top stack item (b) and the previous one (a) and
// replaces both operands with a - b.
type Add struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Add) Pos() token.Position { return o.Position }

func (o *Add) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop()
}

func (o *Add) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "add", o.TypeID, o.Position)
}

// AllocResult operation reserves evaluation stack space for a result of type
// TypeID.
type AllocResult struct {
	TypeID
	TypeName NameID
	token.Position
}

// Pos implements Operation.
func (o *AllocResult) Pos() token.Position { return o.Position }

func (o *AllocResult) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

// Pos implements Operation.
func (o *Argument) Pos() token.Position { return o.Position }

func (o *Argument) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

// Pos implements Operation.
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

// Pos implements Operation.
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

// Bool operation converts TOS to a bool (ie. an int32).
type Bool struct {
	TypeID // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *Bool) Pos() token.Position { return o.Position }

func (o *Bool) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], o.TypeID; g != e {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	v.stack[n-1] = TypeID(idInt32)
	return nil
}

func (o *Bool) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "bool", o.TypeID, o.Position)
}

// Call operation performs a function call. The evaluation stack contains the
// space reseved for function results, is any, the function pointer and any
// function arguments.
type Call struct {
	Arguments int // Actual number of arguments passed to function.
	TypeID        // Type of the function pointer.
	token.Position
}

// Pos implements Operation.
func (o *Call) Pos() token.Position { return o.Position }

func (o *Call) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

// Pos implements Operation.
func (o *Drop) Pos() token.Position { return o.Position }

func (o *Drop) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if len(v.stack) == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	v.stack = v.stack[:len(v.stack)-1]
	return nil
}

func (o *Drop) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "drop", o.TypeID, o.Position)
}

// Dup operation duplicates the top stack item.
type Dup struct {
	TypeID
	token.Position
}

// Pos implements Operation.
func (o *Dup) Pos() token.Position { return o.Position }

func (o *Dup) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if len(v.stack) == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	v.stack = append(v.stack, v.stack[len(v.stack)-1])
	return nil
}

func (o *Dup) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "dup", o.TypeID, o.Position)
}

// Element replaces a pointer and index with the indexed element or its address.
type Element struct {
	Address   bool
	IndexType TypeID
	TypeID    // The indexed type.
	token.Position
}

// Pos implements Operation.
func (o *Element) Pos() token.Position { return o.Position }

func (o *Element) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if o.IndexType == 0 {
		return fmt.Errorf("missing index type")
	}

	switch t := v.typeCache.MustType(o.IndexType); t.Kind() {
	case Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		// ok
	default:
		return fmt.Errorf("invalid index type %s", t.ID())
	}

	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := o.TypeID, v.stack[n-2]; g != e {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	pt := v.typeCache.MustType(o.TypeID)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected a pointer type, have %v", o.TypeID)
	}

	t := pt.(*PointerType).Element
	if o.Address {
		t = t.Pointer()
	}
	v.stack = append(v.stack[:n-2], t.ID())
	return nil
}

func (o *Element) String() string {
	switch {
	case o.Address:
		return fmt.Sprintf("\t%-*s\t&[%v], %v\t; %s", opw, "element", o.IndexType, o.TypeID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t[%v], %v\t; %s", opw, "element", o.IndexType, o.TypeID, o.Position)
	}
}

// EndScope operation annotates leaving a block scope.
type EndScope struct {
	token.Position
}

// Pos implements Operation.
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

// Eq operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a == b or zero
// otherwise.
type Eq struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Eq) Pos() token.Position { return o.Position }

func (o *Eq) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop()
}

func (o *Eq) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "eq", o.TypeID, o.Position)
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

// Pos implements Operation.
func (o *Extern) Pos() token.Position { return o.Position }

func (o *Extern) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

// Field replaces a struct/union pointer at TOS with its field by index, or its
// address.
type Field struct {
	Address bool
	Index   int
	TypeID  // Pointer to a struct/union.
	token.Position
}

// Pos implements Operation.
func (o *Field) Pos() token.Position { return o.Position }

func (o *Field) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := o.TypeID, v.stack[n-1]; g != e {
		return fmt.Errorf("mismatched field pointer types, got %s, expected %s", g, e)
	}

	pt := v.typeCache.MustType(o.TypeID)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected a pointer type, have %v", o.TypeID)
	}

	t := pt.(*PointerType).Element
	if t.Kind() != Struct && t.Kind() != Union {
		return fmt.Errorf("expected a pointer to a struct/union, have %v", o.TypeID)
	}

	st := t.(*StructOrUnionType)
	if o.Index >= len(st.Fields) {
		return fmt.Errorf("invalid index")
	}

	t = st.Fields[o.Index]
	if o.Address {
		t = t.Pointer()
	}
	v.stack[n-1] = t.ID()
	return nil
}

func (o *Field) String() string {
	switch {
	case o.Address:
		return fmt.Sprintf("\t%-*s\t&%v, %v\t; %s", opw, "field", o.Index, o.TypeID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v, %v\t; %s", opw, "field", o.Index, o.TypeID, o.Position)
	}
}

// Int32Const operation pushes an int32 literal on the evaluation stack.
type Int32Const struct {
	Value int32
	token.Position
}

// Pos implements Operation.
func (o *Int32Const) Pos() token.Position { return o.Position }

func (o *Int32Const) verify(v *verifier) error {
	v.stack = append(v.stack, TypeID(idInt32))
	return nil
}

func (o *Int32Const) String() string {
	return fmt.Sprintf("\t%-*s\t%v, int32\t; %s", opw, "const", o.Value, o.Position)
}

// Jmp operation performs a branch to a named or numbered label.
type Jmp struct {
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jmp) Pos() token.Position { return o.Position }

func (o *Jmp) verify(v *verifier) error { return nil }

func (o *Jmp) String() string {
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jmp", o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jmp", o.Number, o.Position)
	}
}

// Jnz operation performs a branch to a named or numbered label if the top of
// the stack is non zero. The TOS type must be int32 and the operation removes
// TOS.
type Jnz struct {
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jnz) Pos() token.Position { return o.Position }

func (o *Jnz) verify(v *verifier) error { return v.branch() }

func (o *Jnz) String() string {
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jnz", o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jnz", o.Number, o.Position)
	}
}

// Jz operation performs a branch to a named or numbered label if the top of
// the stack is zero. The TOS type must be int32 and the operation removes TOS.
type Jz struct {
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jz) Pos() token.Position { return o.Position }

func (o *Jz) verify(v *verifier) error { return v.branch() }

func (o *Jz) String() string {
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jz", o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jz", o.Number, o.Position)
	}
}

// Label operation declares a named or numbered branch target.
type Label struct {
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Label) Pos() token.Position { return o.Position }

func (o *Label) verify(v *verifier) error {
	n := int(o.NameID)
	if n == 0 {
		n = o.Number
	}
	if _, ok := v.labels[n]; ok {
		return fmt.Errorf("label redefined")
	}

	v.labels[n] = struct{}{}
	return nil
}

func (o *Label) String() string {
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("%v:\t\t\t; %s", o.NameID, o.Position)
	default:
		return fmt.Sprintf("%v:\t\t\t; %s", o.Number, o.Position)
	}
}

// Leq operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a <= b or zero
// otherwise.
type Leq struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Leq) Pos() token.Position { return o.Position }

func (o *Leq) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop()
}

func (o *Leq) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "leq", o.TypeID, o.Position)
}

// Load replaces a pointer at TOS by its pointee.
type Load struct {
	TypeID // Pointer type.
	token.Position
}

// Pos implements Operation.
func (o *Load) Pos() token.Position { return o.Position }

func (o *Load) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := o.TypeID, v.stack[n-1]; g != e {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	pt := v.typeCache.MustType(o.TypeID)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected a pointer type, have %v", o.TypeID)
	}

	t := pt.(*PointerType).Element
	v.stack[n-1] = t.ID()
	return nil
}

func (o *Load) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "load", o.TypeID, o.Position)
}

// Lt operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a < b or zero
// otherwise.
type Lt struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Lt) Pos() token.Position { return o.Position }

func (o *Lt) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop()
}

func (o *Lt) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "lt", o.TypeID, o.Position)
}

// Mul operation subtracts the top stack item (b) and the previous one (a) and
// replaces both operands with a * b.
type Mul struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Mul) Pos() token.Position { return o.Position }

func (o *Mul) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop()
}

func (o *Mul) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "mul", o.TypeID, o.Position)
}

// Panic operation aborts execution with a stack trace.
type Panic struct {
	token.Position
}

// Pos implements Operation.
func (o *Panic) Pos() token.Position { return o.Position }

func (o *Panic) verify(v *verifier) error { return nil }

func (o *Panic) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "panic", o.Position)
}

// PostIncrement operation adds Delta to the value pointed to by address at TOS
// and replaces TOS by the value pointee had before the increment.
type PostIncrement struct {
	Delta  int
	TypeID // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *PostIncrement) Pos() token.Position { return o.Position }

func (o *PostIncrement) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	t := v.typeCache.MustType(v.stack[n-1])
	if t.Kind() != Pointer {
		return fmt.Errorf("expected a pointer at TOS, got %s ", v.stack[n-1])
	}

	t = t.(*PointerType).Element
	switch t.Kind() {
	case Array, Union, Struct, Function:
		return fmt.Errorf("invalid operand type %s ", v.stack[n-1])
	}

	if g, e := o.TypeID, t.ID(); g != e {
		return fmt.Errorf("mismatched operand types %s and %s", g, e)
	}

	v.stack[n-1] = o.TypeID
	return nil
}

func (o *PostIncrement) String() string {
	return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, o.TypeID.String()+"++", o.Delta, o.Position)
}

// Result pushes a function result by index, or its address, to the evaluation
// stack.
type Result struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

// Pos implements Operation.
func (o *Result) Pos() token.Position { return o.Position }

func (o *Result) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

// Return operation removes all function call arguments from the evaluation
// stack as well as the function pointer used in the call.
type Return struct {
	token.Position
}

// Pos implements Operation.
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

// Store operation stores a TOS value at address in the preceding stack
// position.  The address is removed from the evaluation stack.
type Store struct {
	TypeID
	token.Position
}

// Pos implements Operation.
func (o *Store) Pos() token.Position { return o.Position }

func (o *Store) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

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

	if g, e := o.TypeID, v.stack[p+1]; g != e {
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

// Pos implements Operation.
func (o *StringConst) Pos() token.Position { return o.Position }

func (o *StringConst) verify(v *verifier) error {
	v.stack = append(v.stack, TypeID(idInt8Ptr))
	return nil
}

func (o *StringConst) String() string {
	return fmt.Sprintf("\t%-*s\t%q, %s\t; %s", opw, "const", o.Value, dict.S(idInt8Ptr), o.Position)
}

// Sub operation subtracts the top stack item (b) and the previous one (a) and
// replaces both operands with a - b.
type Sub struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Sub) Pos() token.Position { return o.Position }

func (o *Sub) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop()
}

func (o *Sub) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "sub", o.TypeID, o.Position)
}

// Variable pushes a function local variable by index, or its address, to the
// evaluation stack.
type Variable struct {
	Address bool
	Index   int
	TypeID
	token.Position
}

// Pos implements Operation.
func (o *Variable) Pos() token.Position { return o.Position }

func (o *Variable) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if o.Index < 0 || o.Index >= len(v.variables) {
		return fmt.Errorf("invalid variable index")
	}

	t := v.typeCache.MustType(v.variables[o.Index])
	if o.Address {
		switch {
		case t.Kind() == Array:
			t = t.(*ArrayType).Item.Pointer()
		default:
			t = t.Pointer()
		}
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

// Pos implements Operation.
func (o *VariableDeclaration) Pos() token.Position { return o.Position }

func (o *VariableDeclaration) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.variables = append(v.variables, o.TypeID)
	return nil
}

func (o *VariableDeclaration) String() string {
	var s string
	switch {
	case o.Value != nil:
		s = fmt.Sprintf("%v(%v)", o.TypeID, o.Value)
	default:
		s = fmt.Sprintf("%v", o.TypeID)
	}
	return fmt.Sprintf("\t%-*s\t%v, %s, %s\t; %s %s", opw, "varDecl", o.Index, o.NameID, s, o.TypeName, o.Position)
}
