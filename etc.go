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
}

var (
	dict = xc.Dict

	idInt32   = dict.SID("int32")
	idInt8Ptr = dict.SID("*int8")
	idStart   = dict.SID("_start")
	idVoidPtr = dict.SID("*struct{}")

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

func validPtrBinop(tc TypeCache, a, b TypeID) bool {
	t := tc.MustType(a)
	u := tc.MustType(b)
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
