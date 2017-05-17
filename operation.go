// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
	"go/token"

	"github.com/cznic/internal/buffer"
)

const opw = 16

var (
	_ Operation = (*Add)(nil)
	_ Operation = (*AllocResult)(nil)
	_ Operation = (*And)(nil)
	_ Operation = (*Argument)(nil)
	_ Operation = (*Arguments)(nil)
	_ Operation = (*BeginScope)(nil)
	_ Operation = (*Bool)(nil)
	_ Operation = (*Call)(nil)
	_ Operation = (*CallFP)(nil)
	_ Operation = (*Const)(nil)
	_ Operation = (*Const32)(nil)
	_ Operation = (*Const64)(nil)
	_ Operation = (*ConstC128)(nil)
	_ Operation = (*Convert)(nil)
	_ Operation = (*Copy)(nil)
	_ Operation = (*Cpl)(nil)
	_ Operation = (*Div)(nil)
	_ Operation = (*Drop)(nil)
	_ Operation = (*Dup)(nil)
	_ Operation = (*Element)(nil)
	_ Operation = (*EndScope)(nil)
	_ Operation = (*Eq)(nil)
	_ Operation = (*Field)(nil)
	_ Operation = (*FieldValue)(nil)
	_ Operation = (*Geq)(nil)
	_ Operation = (*Global)(nil)
	_ Operation = (*Gt)(nil)
	_ Operation = (*Jmp)(nil)
	_ Operation = (*JmpP)(nil)
	_ Operation = (*Jnz)(nil)
	_ Operation = (*Jz)(nil)
	_ Operation = (*Label)(nil)
	_ Operation = (*Leq)(nil)
	_ Operation = (*Load)(nil)
	_ Operation = (*Lsh)(nil)
	_ Operation = (*Lt)(nil)
	_ Operation = (*Mul)(nil)
	_ Operation = (*Neg)(nil)
	_ Operation = (*Neq)(nil)
	_ Operation = (*Nil)(nil)
	_ Operation = (*Not)(nil)
	_ Operation = (*Or)(nil)
	_ Operation = (*Panic)(nil)
	_ Operation = (*PostIncrement)(nil)
	_ Operation = (*PreIncrement)(nil)
	_ Operation = (*PtrDiff)(nil)
	_ Operation = (*Rem)(nil)
	_ Operation = (*Result)(nil)
	_ Operation = (*Return)(nil)
	_ Operation = (*Rsh)(nil)
	_ Operation = (*Store)(nil)
	_ Operation = (*StringConst)(nil)
	_ Operation = (*Sub)(nil)
	_ Operation = (*Switch)(nil)
	_ Operation = (*Variable)(nil)
	_ Operation = (*VariableDeclaration)(nil)
	_ Operation = (*Xor)(nil)
)

// Operation is a unit of execution.
type Operation interface {
	Pos() token.Position
	verify(*verifier) error
}

// Add operation adds the top stack item (b) and the previous one (a) and
// replaces both operands with a + b.
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

	return v.binop(o.TypeID)
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

// And operation replaces TOS with the bitwise and of the top two stack items.
type And struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *And) Pos() token.Position { return o.Position }

func (o *And) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop(o.TypeID)
}

func (o *And) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "and", o.TypeID, o.Position)
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
		u := v.typeCache.MustType(e)
		ok := u.Kind() == Array && g == u.(*ArrayType).Item.Pointer().ID()
		if !ok {
			return fmt.Errorf("have %s, expected type %s", g, e)
		}
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Argument) String() string {
	return fmt.Sprintf("\t%-*s\t%s#%v, %v\t; %s", opw, "argument", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Arguments operation annotates that function results, if any, are allocated
// and a function pointer is at TOS. Evaluation of any function arguments
// follows.
type Arguments struct {
	token.Position
	FunctionPointer bool // TOS contains a function pointer for a subsequent CallFP. Determined by linker.
}

// Pos implements Operation.
func (o *Arguments) Pos() token.Position { return o.Position }

func (o *Arguments) verify(v *verifier) error {
	return nil // Verified in Call/CallFP.
}

func (o *Arguments) String() string {
	s := ""
	if o.FunctionPointer {
		s = "fp"
	}
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "arguments", s, o.Position)
}

// BeginScope operation annotates entering a block scope.
type BeginScope struct {
	// Evaluation stack may be non-empty on entering the scope. See
	// https://gcc.gnu.org/onlinedocs/gcc/Statement-Exprs.html
	Value bool
	token.Position
}

// Pos implements Operation.
func (o *BeginScope) Pos() token.Position { return o.Position }

func (o *BeginScope) verify(v *verifier) error {
	if o.Value {
		v.blockValueLevel++
	}
	if len(v.stack) != 0 && v.blockValueLevel == 0 {
		return fmt.Errorf("non empty evaluation stack at scope begin")
	}

	return nil
}

func (o *BeginScope) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "beginScope", o.Position)
}

// Bool operation converts TOS to a bool (ie. an int32) such that the result
// reflects if the operand was non zero.
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

	if g, e := v.stack[n-1], o.TypeID; g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	v.stack[n-1] = idInt32
	return nil
}

func (o *Bool) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "bool", o.TypeID, o.Position)
}

// Call operation performs a static function call. The evaluation stack
// contains the space reseved for function results, if any, and any function
// arguments. On return all arguments are removed from the stack.
type Call struct {
	Arguments int  // Actual number of arguments passed to function.
	Comma     bool // The call operation is produced by the C comma operator for a void function.
	Index     int  // A negative value or an function object index as resolved by the linker.
	TypeID         // Type of the function.
	token.Position
}

// Pos implements Operation.
func (o *Call) Pos() token.Position { return o.Position }

func (o *Call) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	t := v.typeCache.MustType(o.TypeID)
	if t.Kind() != Function {
		return fmt.Errorf("expected function type, got %s", o.TypeID)
	}

	if len(v.stack) < o.Arguments {
		return fmt.Errorf("evaluation stack underflow")
	}

	ap := len(v.stack) - o.Arguments
	results := t.(*FunctionType).Results
	if len(v.stack) < len(results)+o.Arguments {
		return fmt.Errorf("evaluation stack underflow")
	}

	for i, r := range results {
		if g, e := v.stack[ap-len(results)+i], r.ID(); g != e && !v.assignable(g, e) {
			return fmt.Errorf("mismatched result #%v, got %s, expected %s", i, g, e)
		}
	}

	args := t.(*FunctionType).Arguments
	for i, val := range v.stack[ap:] {
		if i >= len(args) {
			break
		}

		if g, e := val, args[i].ID(); g != e && !v.assignable(g, e) {
			u := v.typeCache.MustType(e)
			if u.Kind() == Pointer {
				u = u.(*PointerType).Element
			}
			ok := u.Kind() == Array && g == u.(*ArrayType).Item.Pointer().ID()
			if !ok {
				return fmt.Errorf("invalid argument #%v type, got %v, expected %s", i, g, e)
			}
		}
	}

	v.stack = v.stack[:ap]
	return nil
}

func (o *Call) String() string {
	sc := ""
	if o.Comma {
		sc = "(,)"
	}
	s := ""
	if o.Index >= 0 {
		s = fmt.Sprintf("#%v, ", o.Index)
	}
	return fmt.Sprintf("\t%-*s\t%s%v, %s\t; %s", opw, "call"+sc, s, o.Arguments, o.TypeID, o.Position)
}

// CallFP operation performs a function pointer call. The evaluation stack
// contains the space reseved for function results, if any, the function
// pointer and any function arguments. On return all arguments and the function
// pointer are removed from the stack.
type CallFP struct {
	Arguments int  // Actual number of arguments passed to function.
	Comma     bool // The call FP operation is produced by the C comma operator for a void function.
	TypeID         // Type of the function pointer.
	token.Position
}

// Pos implements Operation.
func (o *CallFP) Pos() token.Position { return o.Position }

func (o *CallFP) verify(v *verifier) error {
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
		if g, e := v.stack[fp-len(results)+i], r.ID(); g != e && !v.assignable(g, e) {
			return fmt.Errorf("mismatched result #%v, got %s, expected %s", i, g, e)
		}
	}

	args := t.(*FunctionType).Arguments
	for i, val := range v.stack[fp+1:] {
		if i >= len(args) {
			break
		}

		if g, e := val, args[i].ID(); g != e && !v.assignable(g, e) {
			u := v.typeCache.MustType(e)
			if u.Kind() == Pointer {
				u = u.(*PointerType).Element
			}
			ok := u.Kind() == Array && g == u.(*ArrayType).Item.Pointer().ID()
			if !ok {
				return fmt.Errorf("invalid argument #%v type, got %v, expected %s", i, g, e)
			}
		}
	}

	v.stack = v.stack[:fp]
	return nil
}

func (o *CallFP) String() string {
	sc := ""
	if o.Comma {
		sc = "(,)"
	}
	return fmt.Sprintf("\t%-*s\t%v, %s\t; %s", opw, "callfp"+sc, o.Arguments, o.TypeID, o.Position)
}

// Const operation pushes a constant value on the evaluation stack.
type Const struct {
	TypeID
	Value Value
	token.Position
}

// Pos implements Operation.
func (o *Const) Pos() token.Position { return o.Position }

func (o *Const) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Const) String() string {
	return fmt.Sprintf("\t%-*s\t%v, %v\t; %s", opw, "const", o.Value, o.TypeID, o.Position)
}

// Const32 operation pushes a 32 bit value on the evaluation stack.
type Const32 struct {
	LOp bool // This operation is an artifact of || or &&.
	TypeID
	Value int32
	token.Position
}

// Pos implements Operation.
func (o *Const32) Pos() token.Position { return o.Position }

func (o *Const32) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Const32) String() string {
	s := ""
	if o.LOp {
		s = "(nop)"
	}
	return fmt.Sprintf("\t%-*s\t%#x, %v\t; %s", opw, "const"+s, uint32(o.Value), o.TypeID, o.Position)
}

// Const64 operation pushes a 64 bit value on the evaluation stack.
type Const64 struct {
	TypeID
	Value int64
	token.Position
}

// Pos implements Operation.
func (o *Const64) Pos() token.Position { return o.Position }

func (o *Const64) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Const64) String() string {
	return fmt.Sprintf("\t%-*s\t%#x, %v\t; %s", opw, "const", uint64(o.Value), o.TypeID, o.Position)
}

// ConstC128 operation pushes a complex128 value on the evaluation stack.
type ConstC128 struct {
	TypeID
	Value complex128
	token.Position
}

// Pos implements Operation.
func (o *ConstC128) Pos() token.Position { return o.Position }

func (o *ConstC128) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *ConstC128) String() string {
	return fmt.Sprintf("\t%-*s\t%v, %v\t; %s", opw, "const", o.Value, o.TypeID, o.Position)
}

// Convert operation converts TOS to the result type.
type Convert struct {
	Result TypeID // Conversion type.
	TypeID        // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *Convert) Pos() token.Position { return o.Position }

func (o *Convert) verify(v *verifier) error {
	if o.TypeID == 0 || o.Result == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], o.TypeID; g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	v.stack[n-1] = o.Result
	return nil
}

func (o *Convert) String() string {
	return fmt.Sprintf("\t%-*s\t%s, %s\t; %s", opw, "convert", o.TypeID, o.Result, o.Position)
}

// Copy assigns source, which address is at TOS, to dest, which address is the
// previous stack item. The source address is removed from the stack.
type Copy struct {
	TypeID // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *Copy) Pos() token.Position { return o.Position }

func (o *Copy) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	t := v.typeCache.MustType(o.TypeID)
	if t.Kind() == Array {
		t = t.(*ArrayType).Item
	}
	t = t.Pointer()
	if g, e := v.stack[n-2], t.ID(); g != e && g != idVoidPtr {
		return fmt.Errorf("mismatched destination type, got %s, expected %s", g, e)
	}

	if g, e := v.stack[n-1], t.ID(); g != e && g != idVoidPtr {
		return fmt.Errorf("mismatched source type, got %s, expected %s", g, e)
	}

	v.stack = v.stack[:n-1]
	return nil
}

func (o *Copy) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "copy", o.TypeID, o.Position)
}

// Cpl operation replaces TOS with ^TOS (bitwise complement).
type Cpl struct {
	TypeID // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *Cpl) Pos() token.Position { return o.Position }

func (o *Cpl) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.unop(true)
}

func (o *Cpl) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "cpl", o.TypeID, o.Position)
}

// Div operation subtracts the top stack item (b) and the previous one (a) and
// replaces both operands with a / b. The operation panics if operands are
// integers and b == 0.
type Div struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Div) Pos() token.Position { return o.Position }

func (o *Div) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop(o.TypeID)
}

func (o *Div) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "div", o.TypeID, o.Position)
}

// Drop operation removes one item from the evaluation stack.
type Drop struct {
	Comma bool // The drop operation is produced by the C comma operator.
	LOp   bool // This operation is an artifact of || or &&.
	TypeID
	token.Position
}

// Pos implements Operation.
func (o *Drop) Pos() token.Position { return o.Position }

func (o *Drop) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	t := v.typeCache.MustType(o.TypeID)
	switch t.Kind() {
	case Array:
		t = t.(*ArrayType).Item.Pointer()
	}
	if g, e := v.stack[n-1], t.ID(); !v.assignable(g, e) {
		return fmt.Errorf("operand type mismatch, got %s, expected %s", g, e)
	}
	v.stack = v.stack[:len(v.stack)-1]
	return nil
}

func (o *Drop) String() string {
	s := ""
	if o.Comma {
		s = "(,)"
	}
	s2 := ""
	if o.LOp {
		s2 = "(nop)"
	}
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "drop"+s+s2, o.TypeID, o.Position)
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

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], o.TypeID; g != e && !v.assignable(g, e) {
		return fmt.Errorf("operand type mismatch, got %s, expected %s", g, e)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Dup) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "dup", o.TypeID, o.Position)
}

// Element replaces a pointer and index with the indexed element or its address.
type Element struct {
	Address   bool
	IndexType TypeID
	Neg       bool // Negate the index expression.
	TypeID         // The indexed type.
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
		ok := false
		if e2 := v.typeCache.MustType(e); e2.Kind() == Pointer {
			e3 := e2.(*PointerType).Element
			if e3.Kind() == Array {
				ok = g == e3.(*ArrayType).Item.Pointer().ID()
			}
		}
		if !ok {
			ok = v.isVoidPtr(e) && v.isPtr(g)
		}
		if !ok {
			return fmt.Errorf("mismatched type, got %s, expected %s", g, e)
		}
	}

	pt := v.typeCache.MustType(o.TypeID)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected a pointer type, have %v", o.TypeID)
	}

	t := pt.(*PointerType).Element
	if o.Address {
		if t.Kind() == Array {
			t = t.(*ArrayType).Item
		}
		t = t.Pointer()
	}
	v.stack = append(v.stack[:n-2], t.ID())
	return nil
}

func (o *Element) String() string {
	s := ""
	if o.Neg {
		s = "-"
	}
	switch {
	case o.Address:
		return fmt.Sprintf("\t%-*s\t&[%s%v], %v\t; %s", opw, "element", s, o.IndexType, o.TypeID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t[%s%v], %v\t; %s", opw, "element", s, o.IndexType, o.TypeID, o.Position)
	}
}

// EndScope operation annotates leaving a block scope.
type EndScope struct {
	// Leaving the scope may leave values on the evaluation stack. See
	// https://gcc.gnu.org/onlinedocs/gcc/Statement-Exprs.html
	Value bool
	token.Position
}

// Pos implements Operation.
func (o *EndScope) Pos() token.Position { return o.Position }

func (o *EndScope) verify(v *verifier) error {
	if len(v.stack) != 0 && v.blockValueLevel == 0 {
		return fmt.Errorf("non empty evaluation stack at scope end")
	}

	if o.Value {
		v.blockValueLevel--
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

	return v.relop(o.TypeID)
}

func (o *Eq) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "eq", o.TypeID, o.Position)
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

	if g, e := o.TypeID, v.stack[n-1]; g != e && !v.assignable(g, e) {
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
		switch t.Kind() {
		case Array:
			t = t.(*ArrayType).Item.Pointer()
		default:
			t = t.Pointer()
		}
	}
	v.stack[n-1] = t.ID()
	return nil
}

func (o *Field) String() string {
	switch {
	case o.Address:
		return fmt.Sprintf("\t%-*s\t&#%v, %v\t; %s", opw, "field", o.Index, o.TypeID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t#%v, %v\t; %s", opw, "field", o.Index, o.TypeID, o.Position)
	}
}

// FieldValue replaces a struct/union at TOS with its field by index.
type FieldValue struct {
	Index  int
	TypeID // Struct/union type.
	token.Position
}

// Pos implements Operation.
func (o *FieldValue) Pos() token.Position { return o.Position }

func (o *FieldValue) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	t := v.typeCache.MustType(o.TypeID)
	if t.Kind() != Struct && t.Kind() != Union {
		return fmt.Errorf("expected struct/union type, have '%s'", t)
	}

	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := o.TypeID, v.stack[n-1]; g != e {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	st := t.(*StructOrUnionType)
	if o.Index >= len(st.Fields) {
		return fmt.Errorf("invalid index")
	}

	v.stack[n-1] = st.Fields[o.Index].ID()
	return nil
}

func (o *FieldValue) String() string {
	return fmt.Sprintf("\t%-*s\t#%v, %v\t; %s", opw, "fieldvalue", o.Index, o.TypeID, o.Position)
}

// Geq operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a >= b or zero
// otherwise.
type Geq struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Geq) Pos() token.Position { return o.Position }

func (o *Geq) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop(o.TypeID)
}

func (o *Geq) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "geq", o.TypeID, o.Position)
}

// Global operation pushes a global variable, or its address, to the evaluation
// stack.
type Global struct {
	Address bool
	Index   int // A negative value or an object index as resolved by the linker.
	Linkage
	NameID
	TypeID
	TypeName NameID
	token.Position
}

// Pos implements Operation.
func (o *Global) Pos() token.Position { return o.Position }

func (o *Global) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if o.Linkage != ExternalLinkage && o.Linkage != InternalLinkage {
		return fmt.Errorf("invalid linkage")
	}

	//TODO add context to v so that on .Index >= 0 the linker result can be verified.
	t := v.typeCache.MustType(o.TypeID)
	if o.Address && t.Kind() != Pointer {
		return fmt.Errorf("expected pointer type, have %s", o.TypeID)
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Global) String() string {
	s := ""
	if o.Index >= 0 {
		s = fmt.Sprintf("#%v, ", o.Index)
	}
	return fmt.Sprintf("\t%-*s\t%s, %s\t; %s %s", opw, "global", s+addr(o.Address)+o.NameID.String(), o.TypeID, o.TypeName, o.Position)
}

// Gt operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a > b or zero
// otherwise.
type Gt struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Gt) Pos() token.Position { return o.Position }

func (o *Gt) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop(o.TypeID)
}

func (o *Gt) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "gt", o.TypeID, o.Position)
}

// Jmp operation performs a branch to a named or numbered label.
type Jmp struct {
	Cond bool // This operation is an artifact of the conditional operator.
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jmp) Pos() token.Position { return o.Position }

func (o *Jmp) verify(v *verifier) error { return nil }

func (o *Jmp) String() string {
	s := ""
	if o.Cond {
		s = "(nop)"
	}
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jmp"+s, o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jmp"+s, o.Number, o.Position)
	}
}

// JmpP operation performs a branch to pointer at TOS.
type JmpP struct {
	token.Position
}

// Pos implements Operation.
func (o *JmpP) Pos() token.Position { return o.Position }

func (o *JmpP) verify(v *verifier) error {
	n := len(v.stack)
	if n != 1 {
		return fmt.Errorf("evaluation stack must have exactly one item")
	}

	g := v.typeCache.MustType(v.stack[0])
	for g.Kind() == Pointer {
		g = g.(*PointerType).Element
	}
	if g, e := g.ID(), idVoid; g != e {
		return fmt.Errorf("invalid TOS type, expected %v, have %s", e, g)
	}

	v.stack = v.stack[:0]
	return nil
}

func (o *JmpP) String() string {
	return fmt.Sprintf("\t%-*s\t(sp)\t; %s", opw, "jmp", o.Position)
}

// Jnz operation performs a branch to a named or numbered label if the top of
// the stack is non zero. The TOS type must be int32 and the operation removes
// TOS.
type Jnz struct {
	LOp bool // This operation is an artifact of || or &&.
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jnz) Pos() token.Position { return o.Position }

func (o *Jnz) verify(v *verifier) error { return v.branch() }

func (o *Jnz) String() string {
	s := ""
	if o.LOp {
		s = "(nop)"
	}
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jnz"+s, o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jnz"+s, o.Number, o.Position)
	}
}

// Jz operation performs a branch to a named or numbered label if the top of
// the stack is zero. The TOS type must be int32 and the operation removes TOS.
type Jz struct {
	LOp bool // This operation is an artifact of || or && or the conditional operator.
	NameID
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Jz) Pos() token.Position { return o.Position }

func (o *Jz) verify(v *verifier) error { return v.branch() }

func (o *Jz) String() string {
	s := ""
	if o.LOp {
		s = "(nop)"
	}
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jz"+s, o.NameID, o.Position)
	default:
		return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, "jz"+s, o.Number, o.Position)
	}
}

// Label operation declares a named or numbered branch target. A valid Label
// must have a non zero NameID or non negative Number.
type Label struct {
	Cond bool // This operation is an artifact of the conditional operator.
	LAnd bool // This operation is an artifact of &&.
	LOr  bool // This operation is an artifact of ||.
	NameID
	Nop    bool // This operation is an artifact of the conditional operator.
	Number int
	token.Position
}

// Pos implements Operation.
func (o *Label) Pos() token.Position { return o.Position }

// A valid Label must have a non zero NameID or a non negative Number.
func (o *Label) IsValid() bool { return o.NameID > 0 || o.Number >= 0 }

func (o *Label) verify(v *verifier) error {
	if o.NameID != 0 && len(v.stack) != 0 {
		return fmt.Errorf("non empty evaluation stack at named label")
	}

	if !o.IsValid() {
		return fmt.Errorf("invalid label")
	}

	return nil
}

func (o *Label) String() string {
	s := ""
	switch {
	case o.LAnd:
		s = "(&&)"
	case o.LOr:
		s = "(||)"
	case o.Cond:
		s = "(a?b:c)"
	case o.Nop:
		s = "(nop)"
	}
	switch {
	case o.NameID != 0:
		return fmt.Sprintf("%v%s:\t\t\t; %s", o.NameID, s, o.Position)
	default:
		return fmt.Sprintf("%v%s:\t\t\t; %s", o.Number, s, o.Position)
	}
}

func (o *Label) str() string {
	switch {
	case o.NameID != 0:
		return o.NameID.String()
	default:
		return fmt.Sprint(o.Number)
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

	return v.relop(o.TypeID)
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

	if g, e := o.TypeID, v.stack[n-1]; g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched types, got %s, expected %s", g, e)
	}

	pt := v.typeCache.MustType(o.TypeID)
	if pt.Kind() != Pointer {
		return fmt.Errorf("expected a pointer type, have %v", o.TypeID)
	}

	v.stack[n-1] = pt.(*PointerType).Element.ID()
	return nil
}

func (o *Load) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "load", o.TypeID, o.Position)
}

// Lsh operation uses the top stack item (b), which must be an int32, and the
// previous one (a), which must be an integral type and replaces both operands
// with a << b.
type Lsh struct {
	TypeID // Operand (a) type.
	token.Position
}

// Pos implements Operation.
func (o *Lsh) Pos() token.Position { return o.Position }

func (o *Lsh) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	switch v.typeCache.MustType(o.TypeID).Kind() {
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
	default:
		return fmt.Errorf("left operand of a shift must be an integral type")
	}

	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-2], o.TypeID; g != e {
		return fmt.Errorf("mismatched operand type, got %s, expected %s", g, e)
	}

	if g, e := v.stack[n-1], idInt32; g != e {
		return fmt.Errorf("mismatched shift count type, got %s, expected %s", g, e)
	}

	v.stack = v.stack[:n-1]
	return nil
}

func (o *Lsh) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "lsh", o.TypeID, o.Position)
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

	return v.relop(o.TypeID)
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

	return v.binop(o.TypeID)
}

func (o *Mul) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "mul", o.TypeID, o.Position)
}

// Neg operation replaces TOS with 0-TOS.
type Neg struct {
	TypeID // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *Neg) Pos() token.Position { return o.Position }

func (o *Neg) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.unop(false)
}

func (o *Neg) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "neg", o.TypeID, o.Position)
}

// Neq operation compares the top stack item (b) and the previous one (a) and
// replaces both operands with a non zero int32 value if a != b or zero
// otherwise.
type Neq struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Neq) Pos() token.Position { return o.Position }

func (o *Neq) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.relop(o.TypeID)
}

func (o *Neq) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "neq", o.TypeID, o.Position)
}

// Nil pushes a typed nil to TOS.
type Nil struct {
	TypeID // Pointer type.
	token.Position
}

// Pos implements Operation.
func (o *Nil) Pos() token.Position { return o.Position }

func (o *Nil) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *Nil) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "nil", o.TypeID, o.Position)
}

// Not replaces the boolean value at TOS with !value. The TOS type must be
// int32.
type Not struct {
	token.Position
}

// Pos implements Operation.
func (o *Not) Pos() token.Position { return o.Position }

func (o *Not) verify(v *verifier) error {
	n := len(v.stack)
	if n == 0 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-1], idInt32; g != e {
		return fmt.Errorf("unexpected type %s (expected %s)", g, e)
	}

	return nil
}

func (o *Not) String() string {
	return fmt.Sprintf("\t%-*s\t\t; %s", opw, "not", o.Position)
}

// Or operation replaces TOS with the bitwise or of the top two stack items.
type Or struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Or) Pos() token.Position { return o.Position }

func (o *Or) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop(o.TypeID)
}

func (o *Or) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "or", o.TypeID, o.Position)
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
// and replaces TOS by the value pointee had before the increment. If Bits is
// non zero then the effective operand type is BitFieldType and the bit field
// starts at bit BitOffset.
type PostIncrement struct {
	BitFieldType TypeID
	BitOffset    int
	Bits         int
	Delta        int
	TypeID       // Operand type.
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

	if g, e := o.TypeID, t.ID(); g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched operand types %s and %s", g, e)
	}
	switch {
	case o.Bits != 0:
		v.stack[n-1] = o.BitFieldType
	default:
		v.stack[n-1] = o.TypeID
	}
	return nil
}

func (o *PostIncrement) String() string {
	var s string
	if o.Bits != 0 {
		s = fmt.Sprintf(":%d@%d:%v", o.Bits, o.BitOffset, o.BitFieldType)
	}
	return fmt.Sprintf("\t%-*s\t%v\t; %s", opw, o.TypeID.String()+s+"++", o.Delta, o.Position)
}

// PreIncrement operation adds Delta to the value pointed to by address at TOS
// and replaces TOS by the new value of the pointee. If Bits is non zero then
// the effective operand type is BitFieldType and the bit field starts at bit
// BitOffset.
type PreIncrement struct {
	BitFieldType TypeID
	BitOffset    int
	Bits         int
	Delta        int
	TypeID       // Operand type.
	token.Position
}

// Pos implements Operation.
func (o *PreIncrement) Pos() token.Position { return o.Position }

func (o *PreIncrement) verify(v *verifier) error {
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

	if g, e := o.TypeID, t.ID(); g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched operand types %s and %s", g, e)
	}

	switch {
	case o.Bits != 0:
		v.stack[n-1] = o.BitFieldType
	default:
		v.stack[n-1] = o.TypeID
	}
	return nil
}

func (o *PreIncrement) String() string {
	var s string
	if o.Bits != 0 {
		s = fmt.Sprintf(":%d@%d:%v", o.Bits, o.BitOffset, o.BitFieldType)
	}
	return fmt.Sprintf("\t%-*s%s\t%v\t; %s", opw, "++"+o.TypeID.String(), s, o.Delta, o.Position)
}

// PtrDiff operation subtracts the top stack item (b) and the previous one (a)
// and replaces both operands with a - b of type TypeID.
type PtrDiff struct {
	PtrType TypeID
	TypeID  // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *PtrDiff) Pos() token.Position { return o.Position }

func (o *PtrDiff) verify(v *verifier) error {
	if o.TypeID == 0 || o.PtrType == 0 {
		return fmt.Errorf("missing type")
	}

	if v.typeCache.MustType(o.PtrType).Kind() != Pointer {
		return fmt.Errorf("expected pointer type, have '%s'", o.PtrType)
	}

	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g := v.stack[n-2]; v.typeCache.MustType(g).Kind() != Pointer {
		return fmt.Errorf("pointer type required, have %s", g)
	}

	if g, e := v.stack[n-2], v.stack[n-1]; g != e && !v.assignable(g, e) {
		return fmt.Errorf("mismatched operand types %s and %s", g, e)
	}

	v.stack = append(v.stack[:n-2], o.TypeID)
	return nil
}

func (o *PtrDiff) String() string {
	return fmt.Sprintf("\t%-*s\t%s, %s\t; %s", opw, "ptrDiff", o.PtrType, o.TypeID, o.Position)
}

// Rem operation divides the top stack item (b) and the previous one (a) and
// replaces both operands with a % b. The operation panics if b == 0.
type Rem struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Rem) Pos() token.Position { return o.Position }

func (o *Rem) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop(o.TypeID)
}

func (o *Rem) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "rem", o.TypeID, o.Position)
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
	return fmt.Sprintf("\t%-*s\t%s#%v, %v\t; %s", opw, "result", addr(o.Address), o.Index, o.TypeID, o.Position)
}

// Return operation removes all function call arguments from the evaluation
// stack as well as the function pointer used in the call, if any.
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

// Rsh operation uses the top stack item (b), which must be an int32, and the
// previous one (a), which must be an integral type and replaces both operands
// with a >> b.
type Rsh struct {
	TypeID // Operand (a) type.
	token.Position
}

// Pos implements Operation.
func (o *Rsh) Pos() token.Position { return o.Position }

func (o *Rsh) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	switch v.typeCache.MustType(o.TypeID).Kind() {
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
	default:
		return fmt.Errorf("left operand of a shift must be an integral type")
	}

	n := len(v.stack)
	if n < 2 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[n-2], o.TypeID; g != e {
		return fmt.Errorf("mismatched operand type, got %s, expected %s", g, e)
	}

	if g, e := v.stack[n-1], idInt32; g != e {
		return fmt.Errorf("mismatched shift count type, got %s, expected %s", g, e)
	}

	v.stack = v.stack[:n-1]
	return nil
}

func (o *Rsh) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "rsh", o.TypeID, o.Position)
}

// Store operation stores a TOS value at address in the preceding stack
// position.  The address is removed from the evaluation stack.  If Bits is non
// zero then the destination is a bit field starting at bit BitOffset.
type Store struct {
	BitOffset int
	Bits      int
	TypeID    // Type of the value.
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
		return fmt.Errorf("expected pointer and value at TOS, got %s and %s (%v)", tid, v.stack[p+1], v.stack)
	}

	if g, e := pt.(*PointerType).Element.ID(), v.stack[p+1]; !v.assignable(g, e) {
		return fmt.Errorf("mismatched operand types: %s and %s", g, e)
	}

	v.stack = append(v.stack[:p], v.stack[p+1])
	return nil
}

func (o *Store) String() string {
	var s string
	if o.Bits != 0 {
		s = fmt.Sprintf(":%d@%d", o.Bits, o.BitOffset)
	}
	return fmt.Sprintf("\t%-*s\t%s%s\t; %s", opw, "store", o.TypeID, s, o.Position)
}

// StringConst operation pushes a string value on the evaluation stack.
type StringConst struct {
	Value  StringID
	TypeID // Type of the pointer to the string value.
	token.Position
}

// Pos implements Operation.
func (o *StringConst) Pos() token.Position { return o.Position }

func (o *StringConst) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	v.stack = append(v.stack, o.TypeID)
	return nil
}

func (o *StringConst) String() string {
	return fmt.Sprintf("\t%-*s\t%q, %s\t; %s", opw, "const", o.Value, o.TypeID, o.Position)
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

	return v.binop(o.TypeID)
}

func (o *Sub) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "sub", o.TypeID, o.Position)
}

// Switch jumps to a label according to a value at TOS or to a default label.
// The value at TOS is removed from the evaluation stack.
type Switch struct {
	Default Label
	Labels  []Label
	TypeID  // Operand type.
	Values  []Value
	token.Position
}

// Pos implements Operation.
func (o *Switch) Pos() token.Position { return o.Position }

func (o *Switch) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	if !o.Default.IsValid() {
		return fmt.Errorf("invalid default case")
	}

	if g, e := len(o.Values), len(o.Labels); g != e {
		return fmt.Errorf("mismatched number of values and cases")
	}

	p := len(v.stack)
	if p < 1 {
		return fmt.Errorf("evaluation stack underflow")
	}

	if g, e := v.stack[p-1], o.TypeID; g != e {
		return fmt.Errorf("mismatched operand types: %s and %s", g, e)
	}

	for _, v := range o.Values {
		switch x := v.(type) {
		case *Int32Value:
			switch o.TypeID {
			case idInt32, idUint32:
				// ok
			default:
				return fmt.Errorf("invalid switch case value of type %v", o.TypeID)
			}
		case *Int64Value:
			switch o.TypeID {
			case idInt64, idUint64:
				// ok
			default:
				return fmt.Errorf("invalid switch case value of type %v", o.TypeID)
			}
		default:
			return fmt.Errorf("unsupported switch case value %T", x)
		}
	}

	v.stack = v.stack[:p-1]
	return nil
}

func (o *Switch) String() string {
	var buf buffer.Bytes

	defer buf.Close()

	for i, v := range o.Values {
		var l Label
		if i < len(o.Labels) {
			l = o.Labels[i]
		}
		switch x := v.(type) {
		case *Int32Value, *Int64Value:
			fmt.Fprintf(&buf, "\n\tcase %v:", x)
		default:
			panic(fmt.Errorf("unsupported switch case value %T", x))
		}
		fmt.Fprintf(&buf, "\tgoto %v\t; %v", l.str(), l.Position)
	}
	fmt.Fprintf(&buf, "\n\tdefault:\tgoto %v\t; %v", o.Default.str(), o.Default.Position)
	return fmt.Sprintf("\t%-*s\t%s\t; %s%s", opw, "switch", o.TypeID, o.Position, buf.Bytes())
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
	return fmt.Sprintf("\t%-*s\t%s#%v, %v\t; %s", opw, "variable", addr(o.Address), o.Index, o.TypeID, o.Position)
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
	return fmt.Sprintf("\t%-*s\t#%v, %s, %s\t; %s %s", opw, "varDecl", o.Index, o.NameID, s, o.TypeName, o.Position)
}

// Xor operation replaces TOS with the bitwise xor of the top two stack items.
type Xor struct {
	TypeID // Operands type.
	token.Position
}

// Pos implements Operation.
func (o *Xor) Pos() token.Position { return o.Position }

func (o *Xor) verify(v *verifier) error {
	if o.TypeID == 0 {
		return fmt.Errorf("missing type")
	}

	return v.binop(o.TypeID)
}

func (o *Xor) String() string {
	return fmt.Sprintf("\t%-*s\t%s\t; %s", opw, "xor", o.TypeID, o.Position)
}
