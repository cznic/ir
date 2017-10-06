// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"encoding/gob"
	"fmt"
	"go/token"
	"reflect"

	"github.com/cznic/strutil"
	"github.com/cznic/xc"
)

func init() {
	gob.Register(&DataDefinition{})
	gob.Register(&FunctionDefinition{})
	gob.Register(NameID(0))
	gob.Register(StringID(0))
	gob.Register(TypeID(0))

	gob.Register(&Add{})
	gob.Register(&AllocResult{})
	gob.Register(&And{})
	gob.Register(&Argument{})
	gob.Register(&Arguments{})
	gob.Register(&BeginScope{})
	gob.Register(&Bool{})
	gob.Register(&Call{})
	gob.Register(&CallFP{})
	gob.Register(&Const{})
	gob.Register(&Const32{})
	gob.Register(&Const64{})
	gob.Register(&ConstC128{})
	gob.Register(&Convert{})
	gob.Register(&Copy{})
	gob.Register(&Cpl{})
	gob.Register(&Div{})
	gob.Register(&Drop{})
	gob.Register(&Dup{})
	gob.Register(&Element{})
	gob.Register(&EndScope{})
	gob.Register(&Eq{})
	gob.Register(&Field{})
	gob.Register(&FieldValue{})
	gob.Register(&Geq{})
	gob.Register(&Global{})
	gob.Register(&Gt{})
	gob.Register(&Jmp{})
	gob.Register(&JmpP{})
	gob.Register(&Jnz{})
	gob.Register(&Jz{})
	gob.Register(&Label{})
	gob.Register(&Leq{})
	gob.Register(&Load{})
	gob.Register(&Lsh{})
	gob.Register(&Lt{})
	gob.Register(&Mul{})
	gob.Register(&Neg{})
	gob.Register(&Neq{})
	gob.Register(&Nil{})
	gob.Register(&Not{})
	gob.Register(&Or{})
	gob.Register(&Panic{})
	gob.Register(&PostIncrement{})
	gob.Register(&PreIncrement{})
	gob.Register(&PtrDiff{})
	gob.Register(&Rem{})
	gob.Register(&Result{})
	gob.Register(&Return{})
	gob.Register(&Rsh{})
	gob.Register(&Store{})
	gob.Register(&StringConst{})
	gob.Register(&Sub{})
	gob.Register(&Switch{})
	gob.Register(&Variable{})
	gob.Register(&VariableDeclaration{})
	gob.Register(&Xor{})

	gob.Register(&AddressValue{})
	gob.Register(&Complex128Value{})
	gob.Register(&Complex64Value{})
	gob.Register(&CompositeValue{})
	gob.Register(&DesignatedValue{})
	gob.Register(&Float32Value{})
	gob.Register(&Float64Value{})
	gob.Register(&Int32Value{})
	gob.Register(&Int64Value{})
	gob.Register(&StringValue{})
	gob.Register(&WideStringValue{})
}

var (
	dict = xc.Dict

	idBuiltinPrefix = dict.SID("__builtin_")
	idInt16         = TypeID(dict.SID("int16"))
	idInt32         = TypeID(dict.SID("int32"))
	idInt64         = TypeID(dict.SID("int64"))
	idInt8          = TypeID(dict.SID("int8"))
	idStart         = dict.SID("_start")
	idUint32        = TypeID(dict.SID("uint32"))
	idUint64        = TypeID(dict.SID("uint64"))
	idVoid          = TypeID(dict.SID("struct{}"))

	printHooks = strutil.PrettyPrintHooks{
		reflect.TypeOf(NameID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			x := v.(NameID)
			if x == 0 {
				return
			}

			f.Format(prefix)
			f.Format("%s", dict.S(int(x)))
			f.Format(suffix)
		},
		reflect.TypeOf(StringID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			x := v.(StringID)
			if x == 0 {
				return
			}

			f.Format(prefix)
			f.Format("%q", dict.S(int(x)))
			f.Format(suffix)
		},
		reflect.TypeOf(TypeID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			x := v.(TypeID)
			if x == 0 {
				return
			}

			f.Format(prefix)
			f.Format("%s", dict.S(int(x)))
			f.Format(suffix)
		},
		reflect.TypeOf(token.Position{}): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			x := v.(token.Position)
			if !x.IsValid() {
				return
			}

			f.Format(prefix)
			f.Format("%s", x)
			f.Format(suffix)
		},
		reflect.TypeOf(Linkage(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			x := v.(Linkage)
			if x == 0 {
				return
			}

			f.Format(prefix)
			f.Format("%s", x)
			f.Format(suffix)
		},
	}
)

// PrettyString turns certain things, produced by this package, into neatly
// format text.
func PrettyString(v interface{}) string {
	switch x := v.(type) {
	case *BeginScope:
		return fmt.Sprintf("beginScope\t; %s", x.Position)
	default:
		return strutil.PrettyString(v, "", "", printHooks)
	}
}

func addr(n bool) string {
	if n {
		return "&"
	}

	return ""
}

func unconvert(p *[]Operation) {
	s := *p
	w := 0
	for _, v := range s {
		switch x := v.(type) {
		case *Convert:
			if x.TypeID == x.Result {
				continue
			}
		}

		s[w] = v
		w++
	}
	*p = s[:w]
}
