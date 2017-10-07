// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO) //TODOOK
}

// ============================================================================

var (
	types     = TypeCache{}
	testModel = MemoryModel{
		Int8:     MemoryModelItem{Align: 1, Size: 1, StructAlign: 1},
		Int16:    MemoryModelItem{Align: 2, Size: 2, StructAlign: 2},
		Int32:    MemoryModelItem{Align: 4, Size: 4, StructAlign: 4},
		Int64:    MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
		Function: MemoryModelItem{Align: 8, Size: 8, StructAlign: 8},
	}
)

func TestLexer(t *testing.T) {
	for _, v := range []struct {
		src string
		tk  tok
	}{
		{"(", tok('(')},
		{")", tok(')')},
		{",", tok(',')},
		{"...", tokEllipsis},
		{"0", tokNumber},
		{"?", tokIllegal},
		{"[", tok('[')},
		{"]", tok(']')},
		{"complex128", tokC128},
		{"complex256", tokC256},
		{"complex64", tokC64},
		{"float128", tokF128},
		{"float32", tokF32},
		{"float64", tokF64},
		{"func", tokFunc},
		{"int16", tokI16},
		{"int32", tokI32},
		{"int64", tokI64},
		{"int8", tokI8},
		{"struct", tokStruct},
		{"uint16", tokU16},
		{"uint32", tokU32},
		{"uint64", tokU64},
		{"uint8", tokU8},
		{"union", tokUnion},
		{"{", tok('{')},
		{"}", tok('}')},
		{fmt.Sprint(uint64(math.MaxInt64)), tokNumber},
	} {

		b := []byte(fmt.Sprintf("(%s)", v.src))
		if g, e := types.lex(&b), tok('('); g != e {
			t.Fatal(g, e)
		}

		tk, n := types.lex2(&b)
		if g, e := tk, v.tk; g != e {
			t.Fatal(g, e)
		}

		if tk == tokNumber {
			n64, err := strconv.ParseUint(v.src, 10, 64)
			if err != nil {
				panic("internal error")
			}

			if g, e := uint64(n), n64; g != e {
				t.Fatal(g, e)
			}
		}

		if g, e := types.lex(&b), tok(')'); g != e {
			t.Fatal(g, e)
		}

		if g, e := types.lex(&b), tokEOF; g != e {
			t.Fatal(g, e)
		}
	}
}

func benchmarkLexer(b *testing.B) {
	a := [][]byte{
		[]byte("("),
		[]byte(")"),
		[]byte(","),
		[]byte("..."),
		[]byte("0"),
		[]byte("["),
		[]byte("]"),
		[]byte("complex128"),
		[]byte("complex256"),
		[]byte("complex64"),
		[]byte("float128"),
		[]byte("float32"),
		[]byte("float64"),
		[]byte("func"),
		[]byte("int16"),
		[]byte("int32"),
		[]byte("int64"),
		[]byte("int8"),
		[]byte("struct"),
		[]byte("uint16"),
		[]byte("uint32"),
		[]byte("uint64"),
		[]byte("uint8"),
		[]byte("union"),
		[]byte("{"),
		[]byte("}"),
		[]byte("9223372036854775807"),
	}
	n := 0
	for _, v := range a {
		n += len(v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range a {
			if tok := types.lex(&v); tok < 0 || tok >= tokIllegal {
				b.Fatalf("internal error %q", v)
			}
		}
	}
	b.SetBytes(int64(n))
}

func TestParser(t *testing.T) {
	for _, v := range []string{
		"*int8",
		"[0]int8",
		"complex128",
		"complex256",
		"complex64",
		"float128",
		"float32",
		"float64",
		"func()",
		"func()(int32,int64)",
		"func()int32",
		"func(*float32,int32,*func(float64),*func(float32,float32))int32",
		"func(...)",
		"func(int8)",
		"func(int8)(int32,int64)",
		"func(int8)int32",
		"func(int8,...)",
		"func(int8,int16)",
		"func(int8,int16)(int32,int64)",
		"func(int8,int16)int32",
		"int16",
		"int32",
		"int64",
		"int8",
		"struct{ int8,a int16}",
		"struct{_ int8, int16}",
		"struct{ int8, int16}",
		"struct{_ int8,a int16}",
		"struct{a int8,a struct{a int16,a int32},a int64}",
		"struct{a int8}",
		"struct{}",
		"uint16",
		"uint32",
		"uint64",
		"uint8",
		"union{a int8,b int16}",
		"union{c int8}",
		"union{}",
	} {
		for _, suffix := range []string{
			"",
			"(",
			")",
			",",
			".",
			"?",
			"[",
			"]",
			"{",
			"}",
		} {
			id := dict.SID(v + suffix)
			typ, err := types.Type(TypeID(id))
			if err != nil {
				if suffix == "" {
					t.Fatal(v, suffix, err)
				}

				continue
			}

			if suffix != "" {
				t.Fatal(v, suffix)
			}

			if g, e := typ.ID().String(), v; g != e {
				t.Fatalf("%q %q", g, e)
			}

			s := "9" + v
			if typ, err = types.Type(TypeID(dict.SID(s))); err == nil {
				t.Fatalf("%q", s)
			}
		}
	}
	for id, v := range types {
		t.Logf("%d: %q", id, dict.S(int(id)))
		if g, e := v.ID(), id; g != e {
			t.Fatalf("%q %d %d", dict.S(int(id)), g, e)
		}
	}
}

func TestParser2(t *testing.T) {
	types = TypeCache{}
	if _, err := types.Type(TypeID(dict.SID("struct{a int8,b struct{c int16,d int32},e int64}"))); err != nil {
		t.Fatal(err)
	}

	if g, e := len(types), 6; g != e {
		t.Fatal(g, e)
	}

	var a []string
	for k := range types {
		a = append(a, string(dict.S(int(k))))
	}
	sort.Strings(a)
	if g, e := strings.Join(a, "\n"), strings.TrimSpace(`
int16
int32
int64
int8
struct{a int8,b struct{c int16,d int32},e int64}
struct{c int16,d int32}
`); g != e {
		t.Fatalf("==== got\n%s\n==== exp\n%s", g, e)
	}
}

func TestAlignSize(t *testing.T) {
	for i, v := range []struct {
		src   string
		align int
		size  int64
	}{
		{"[0]int16", 2, 0},
		{"[0]int8", 1, 0},
		{"[1]int16", 2, 2},
		{"[1]int8", 1, 1},
		{"[2]int16", 2, 4},
		{"[2]int8", 1, 2},
		{"[2]struct{_ [3]int8,_ int64}", 8, 32},
		{"[2]struct{_ int64,_ [3]int8}", 8, 32},
		{"[2]struct{_ int64,_ int8}", 8, 32},
		{"[2]struct{_ int8,_ int64}", 8, 32},
		{"[2]union{_ [3]int8,_ int64}", 8, 16},
		{"[2]union{_ int64,_ [3]int8}", 8, 16},
		{"[2]union{_ int64,_ int8}", 8, 16},
		{"[2]union{_ int8,_ int64}", 8, 16},
		{"func()", 8, 8},
		{"struct{_ int32,_ struct{},_ int32}", 4, 8},
		{"struct{_ int64,_ int8}", 8, 16},
		{"struct{_ int64}", 8, 8},
		{"struct{}", 1, 0},
		{"union{_ int64,_ int8}", 8, 8},
		{"union{_ int64}", 8, 8},
		{"union{}", 1, 0},
	} {
		typ, err := types.Type(TypeID(dict.SID(v.src)))
		if err != nil {
			t.Fatal(err)
		}

		if g, e := testModel.Alignof(typ), v.align; g != e {
			t.Fatalf("#%v: %s: align %v %v", i, v.src, g, e)
		}

		if g, e := testModel.Sizeof(typ), v.size; g != e {
			t.Fatalf("#%v: %s: size %v %v", i, v.src, g, e)
		}
	}
}

func TestLayoutOffset(t *testing.T) {
	for it, v := range []struct {
		src string
		off []int64
	}{
		{"struct{_ int16,_ int8,_ int8,_ int16}", []int64{0, 2, 3, 4}},
		{"struct{_ int16,_ int8,_ int8,_ int32}", []int64{0, 2, 3, 4}},
		{"struct{_ int16,_ int8,_ int8,_ int64}", []int64{0, 2, 3, 8}},
		{"struct{_ int16,_ int8,_ int8}", []int64{0, 2, 3}},
		{"struct{_ int16,_ int8}", []int64{0, 2}},
		{"struct{_ int8,_ int16}", []int64{0, 2}},
		{"struct{_ int8}", []int64{0}},
		{"struct{}", nil},
		{"union{_ int16,_ int8,_ int8,_ int16}", []int64{0, 0, 0, 0}},
		{"union{_ int16,_ int8,_ int8,_ int32}", []int64{0, 0, 0, 0}},
		{"union{_ int16,_ int8,_ int8,_ int64}", []int64{0, 0, 0, 0}},
		{"union{_ int16,_ int8,_ int8}", []int64{0, 0, 0}},
		{"union{_ int16,_ int8}", []int64{0, 0}},
		{"union{_ int8,_ int16}", []int64{0, 0}},
		{"union{_ int8}", []int64{0}},
		{"union{}", nil},
	} {
		typ, err := types.Type(TypeID(dict.SID(v.src)))
		if err != nil {
			t.Fatal(err)
		}

		fields := testModel.Layout(typ.(*StructOrUnionType))
		if g, e := len(fields), len(v.off); g != e {
			t.Fatalf("%s: fields %v %v", v.src, g, e)
		}

		for i, f := range fields {
			if g, e := f.Offset, v.off[i]; g != e {
				t.Fatalf("#%v: %s.%v: off %v %v", it, v.src, i, g, e)
			}
		}
	}
}

func TestLayoutSize(t *testing.T) {
	for it, v := range []struct {
		src string
		sz  []int64
	}{
		{"struct{_ int16,_ int8,_ int8,_ int16}", []int64{2, 1, 1, 2}},
		{"struct{_ int16,_ int8,_ int8,_ int32}", []int64{2, 1, 1, 4}},
		{"struct{_ int16,_ int8,_ int8,_ int64}", []int64{2, 1, 1, 8}},
		{"struct{_ int16,_ int8,_ int8}", []int64{2, 1, 1}},
		{"struct{_ int16,_ int8}", []int64{2, 1}},
		{"struct{_ int8,_ int16}", []int64{1, 2}},
		{"struct{_ int8}", []int64{1}},
		{"struct{}", nil},
		{"union{_ int16,_ int8,_ int8,_ int16}", []int64{2, 1, 1, 2}},
		{"union{_ int16,_ int8,_ int8,_ int32}", []int64{2, 1, 1, 4}},
		{"union{_ int16,_ int8,_ int8,_ int64}", []int64{2, 1, 1, 8}},
		{"union{_ int16,_ int8,_ int8}", []int64{2, 1, 1}},
		{"union{_ int16,_ int8}", []int64{2, 1}},
		{"union{_ int8,_ int16}", []int64{1, 2}},
		{"union{_ int8}", []int64{1}},
		{"union{}", nil},
	} {
		typ, err := types.Type(TypeID(dict.SID(v.src)))
		if err != nil {
			t.Fatal(err)
		}

		fields := testModel.Layout(typ.(*StructOrUnionType))
		if g, e := len(fields), len(v.sz); g != e {
			t.Fatalf("%s: fields %v %v", v.src, g, e)
		}

		for i, f := range fields {
			if g, e := f.Size, v.sz[i]; g != e {
				t.Fatalf("#%v: %s.%v: size %v %v", it, v.src, i, g, e)
			}
		}
	}
}

func TestLayoutPadding(t *testing.T) {
	for it, v := range []struct {
		src string
		p   []int
	}{
		{"struct{_ int16,_ int8,_ int8,_ int16}", []int{0, 0, 0, 0}},
		{"struct{_ int16,_ int8,_ int8,_ int32}", []int{0, 0, 0, 0}},
		{"struct{_ int16,_ int8,_ int8,_ int64}", []int{0, 0, 4, 0}},
		{"struct{_ int16,_ int8,_ int8}", []int{0, 0, 0}},
		{"struct{_ int16,_ int8}", []int{0, 1}},
		{"struct{_ int8,_ int16}", []int{1, 0}},
		{"struct{_ int8}", []int{0}},
		{"struct{}", nil},
		{"union{_ int16,_ int8,_ int8, int16}", []int{0, 1, 1, 0}},
		{"union{_ int16,_ int8,_ int8, int32}", []int{2, 3, 3, 0}},
		{"union{_ int16,_ int8,_ int8, int64}", []int{6, 7, 7, 0}},
		{"union{_ int16,_ int8,_ int8}", []int{0, 1, 1}},
		{"union{_ int16,_ int8}", []int{0, 1}},
		{"union{_ int8,_ int16}", []int{1, 0}},
		{"union{_ int8}", []int{0}},
		{"union{}", nil},
	} {
		typ, err := types.Type(TypeID(dict.SID(v.src)))
		if err != nil {
			t.Fatal(err)
		}

		fields := testModel.Layout(typ.(*StructOrUnionType))
		if g, e := len(fields), len(v.p); g != e {
			t.Fatalf("%s: fields %v %v", v.src, g, e)
		}

		for i, f := range fields {
			if g, e := f.Padding, v.p[i]; g != e {
				t.Fatalf("#%v: %s.%v: padding %v %v", it, v.src, i, g, e)
			}
		}
	}
}

func benchmarkParser(b *testing.B) {
	a := [][]byte{
		[]byte("*int8"),
		[]byte("[0]int8"),
		[]byte("complex128"),
		[]byte("complex256"),
		[]byte("complex64"),
		[]byte("float128"),
		[]byte("float32"),
		[]byte("float64"),
		[]byte("func()"),
		[]byte("func()(int32,int64)"),
		[]byte("func()int32"),
		[]byte("func(int8)"),
		[]byte("func(int8)(int32,int64)"),
		[]byte("func(int8)int32"),
		[]byte("func(int8,int16)"),
		[]byte("func(int8,int16)(int32,int64)"),
		[]byte("func(int8,int16)int32"),
		[]byte("int16"),
		[]byte("int32"),
		[]byte("int64"),
		[]byte("int8"),
		[]byte("struct{_ int8,_ int16}"),
		[]byte("struct{_ int8}"),
		[]byte("struct{}"),
		[]byte("uint16"),
		[]byte("uint32"),
		[]byte("uint64"),
		[]byte("uint8"),
		[]byte("union{_ int8,_ int16}"),
		[]byte("union{_ int8}"),
		[]byte("union{}"),
	}
	n := 0
	for _, v := range a {
		n += len(v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range a {
			if _, err := types.parse(&v, 0); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.SetBytes(int64(n))
}

func benchmarkTypeCache(b *testing.B) {
	a := []TypeID{
		TypeID(dict.SID("*int8")),
		TypeID(dict.SID("[0]int8")),
		TypeID(dict.SID("complex128")),
		TypeID(dict.SID("complex256")),
		TypeID(dict.SID("complex64")),
		TypeID(dict.SID("float128")),
		TypeID(dict.SID("float32")),
		TypeID(dict.SID("float64")),
		TypeID(dict.SID("func()")),
		TypeID(dict.SID("func()(int32,int64)")),
		TypeID(dict.SID("func()int32")),
		TypeID(dict.SID("func(int8)")),
		TypeID(dict.SID("func(int8)(int32,int64)")),
		TypeID(dict.SID("func(int8)int32")),
		TypeID(dict.SID("func(int8,int16)")),
		TypeID(dict.SID("func(int8,int16)(int32,int64)")),
		TypeID(dict.SID("func(int8,int16)int32")),
		TypeID(dict.SID("int16")),
		TypeID(dict.SID("int32")),
		TypeID(dict.SID("int64")),
		TypeID(dict.SID("int8")),
		TypeID(dict.SID("struct{_ int8,_ int16}")),
		TypeID(dict.SID("struct{_ int8}")),
		TypeID(dict.SID("struct{}")),
		TypeID(dict.SID("uint16")),
		TypeID(dict.SID("uint32")),
		TypeID(dict.SID("uint64")),
		TypeID(dict.SID("uint8")),
		TypeID(dict.SID("union{_ int8,_ int16}")),
		TypeID(dict.SID("union{_ int8}")),
		TypeID(dict.SID("union{}")),
	}
	n := 0
	for _, v := range a {
		n += len(dict.S(int(v)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range a {
			if _, err := types.Type(v); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.SetBytes(int64(n))
}

func Benchmark(b *testing.B) {
	b.Run("Lexer", benchmarkLexer)
	b.Run("Parser", benchmarkParser)
	b.Run("TypeCache", benchmarkTypeCache)
}

func TestGobTypeID(t *testing.T) {
	const c = "The quick brown fox type"
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	in := TypeID(dict.SID(c))
	if err := enc.Encode(in); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(buf.Bytes(), []byte(c)) {
		t.Fatal("TypeID gob encoding fail")
	}

	out := TypeID(-1)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}

	if g, e := in, out; g != e {
		t.Fatal(g, e)
	}
}

func TestGobNameID(t *testing.T) {
	const c = "The quick brown fox name"
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	in := NameID(dict.SID(c))
	if err := enc.Encode(in); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(buf.Bytes(), []byte(c)) {
		t.Fatal("NameID gob encoding fail")
	}

	out := NameID(-1)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}

	if g, e := in, out; g != e {
		t.Fatal(g, e)
	}
}

func TestGobStringID(t *testing.T) {
	const c = "The quick brown fox string"
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	in := StringID(dict.SID(c))
	if err := enc.Encode(in); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(buf.Bytes(), []byte(c)) {
		t.Fatal("NameID gob encoding fail")
	}

	out := StringID(-1)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}

	if g, e := in, out; g != e {
		t.Fatal(g, e)
	}
}

func TestObjectsGob(t *testing.T) {
	out := Objects{
		&FunctionDefinition{
			Body: []Operation{
				&Result{
					Address: true,
				},
			},
		},
	}

	f := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(f)
	if err := enc.Encode(out); err != nil {
		t.Fatal(err)
	}

	var in Objects
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&in); err != nil {
		t.Fatal(err)
	}

	if g, e := PrettyString(in), PrettyString(out); g != e {
		t.Fatalf("got\n%s\nexp\n%s", g, e)
	}
}

func TestObjects(t *testing.T) {
	out := Objects{
		&FunctionDefinition{
			Body: []Operation{
				&Result{
					Address: true,
				},
			},
		},
	}

	f, err := ioutil.TempFile("", "test-ir-")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Objects(out).WriteTo(f); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		t.Fatal(err)
	}

	var in Objects
	if _, err := in.ReadFrom(f); err != nil {
		t.Fatal(err)
	}

	if g, e := PrettyString(in), PrettyString(out); g != e {
		t.Fatalf("got\n%s\nexp\n%s", g, e)
	}
}
