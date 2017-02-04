// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"encoding/gob"
	"go/token"
	"reflect"

	"github.com/cznic/strutil"
	"github.com/cznic/xc"
)

func init() {
	gob.Register(&Declaration{})
	gob.Register(&FunctionDefinition{})
	gob.Register(NameID(0))
	gob.Register(StringID(0))
	gob.Register(TypeID(0))
}

var (
	dict = xc.Dict

	printHooks = strutil.PrettyPrintHooks{
		reflect.TypeOf(NameID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			f.Format(prefix)
			f.Format("%s", dict.S(int(v.(NameID))))
			f.Format(suffix)
		},
		reflect.TypeOf(StringID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			f.Format(prefix)
			f.Format("%q", dict.S(int(v.(StringID))))
			f.Format(suffix)
		},
		reflect.TypeOf(TypeID(0)): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			f.Format(prefix)
			f.Format("%s", dict.S(int(v.(TypeID))))
			f.Format(suffix)
		},
		reflect.TypeOf(token.Position{}): func(f strutil.Formatter, v interface{}, prefix, suffix string) {
			f.Format(prefix)
			f.Format("%s", v.(token.Position))
			f.Format(suffix)
		},
	}
)

func pretty(v interface{}) string { return strutil.PrettyString(v, "", "", printHooks) }
