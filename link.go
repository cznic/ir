// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/cznic/internal/buffer"
)

const (
	binaryVersion = 1 // Compatibility version of Objects.
)

var (
	_ io.ReaderFrom = (*Objects)(nil)
	_ io.Writer     = (*counter)(nil)
	_ io.WriterTo   = (Objects)(nil)

	magic = []byte{0x64, 0xe0, 0xc8, 0x8e, 0xca, 0xeb, 0x80, 0x65}

	main = Objects{
		&FunctionDefinition{
			ObjectBase: ObjectBase{Linkage: ExternalLinkage, NameID: idMain},
			Body: []Operation{
				&Result{Address: true, TypeID: idPint32},
				&Const32{TypeID: idInt32},
				&Store{TypeID: idInt32},
				&Drop{TypeID: idInt32},
				&BeginScope{},
				&Return{},
				&EndScope{},
			},
		},
	}
)

type counter int64

func (c *counter) Write(b []byte) (int, error) {
	*c += counter(len(b))
	return len(b), nil
}

// Objects represent []Object implementing io.ReaderFrom and io.WriterTo.
type Objects []Object

// ReadFrom reads o from r.
func (o *Objects) ReadFrom(r io.Reader) (n int64, err error) {
	var c counter
	*o = nil
	r = io.TeeReader(r, &c)
	gr, err := gzip.NewReader(r)
	if err != nil {
		return 0, err
	}

	if len(gr.Header.Extra) < len(magic) || !bytes.Equal(gr.Header.Extra[:len(magic)], magic) {
		return int64(c), fmt.Errorf("unrecognized file format")
	}

	buf := gr.Header.Extra[len(magic):]
	a := bytes.Split(buf, []byte{'|'})
	if len(a) != 3 {
		return int64(c), fmt.Errorf("corrupted file")
	}

	if s := string(a[0]); s != runtime.GOOS {
		return int64(c), fmt.Errorf("invalid platform %q", s)
	}

	if s := string(a[1]); s != runtime.GOARCH {
		return int64(c), fmt.Errorf("invalid architecture %q", s)
	}

	v, err := strconv.ParseUint(string(a[2]), 10, 64)
	if err != nil {
		return int64(c), err
	}

	if v != binaryVersion {
		return int64(c), fmt.Errorf("invalid version number %v", v)
	}

	err = gob.NewDecoder(gr).Decode(o)
	return int64(c), err
}

// WriteTo writes o to w.
func (o Objects) WriteTo(w io.Writer) (n int64, err error) {
	var c counter
	gw := gzip.NewWriter(io.MultiWriter(w, &c))
	gw.Header.Comment = "IR objects"
	var buf buffer.Bytes
	buf.Write(magic)
	fmt.Fprintf(&buf, fmt.Sprintf("%s|%s|%v", runtime.GOOS, runtime.GOARCH, binaryVersion))
	gw.Header.Extra = buf.Bytes()
	buf.Close()
	gw.Header.ModTime = time.Now()
	gw.Header.OS = 255 // Unknown OS.
	enc := gob.NewEncoder(gw)
	if err := enc.Encode(o); err != nil {
		return int64(c), err
	}

	if err := gw.Close(); err != nil {
		return int64(c), err
	}

	return int64(c), nil
}

// LinkMain returns all objects transitively referenced from function _start or
// an error, if any. Linking may mutate passed objects. It's the caller
// responsibility to ensure all translationUnits were produced for the same
// architecture and platform.
//
// LinkMain panics when passed no data.
func LinkMain(translationUnits ...[]Object) (_ []Object, err error) {
	if !Testing {
		defer func() {
			switch x := recover().(type) {
			case nil:
				// nop
			case error:
				if err == nil {
					err = x
				}
			default:
				err = fmt.Errorf("ir.LinkMain PANIC: %v", x)
			}
		}()
	}
	l := newLinker(translationUnits)
	l.linkMain()
	return l.out, nil
}

// LinkLib returns all objects with external linkage defined in
// translationUnits.  Linking may mutate passed objects. It's the caller
// responsibility to ensure all translationUnits were produced for the same
// architecture and platform.
//
// LinkLib panics when passed no data.
func LinkLib(translationUnits ...[]Object) (_ []Object, err error) {
	if !Testing {
		defer func() {
			switch x := recover().(type) {
			case nil:
				// nop
			case error:
				if err == nil {
					err = x
				}
			default:
				err = fmt.Errorf("ir.LinkLib PANIC %v", x)
			}
		}()
	}
	ok := false
search:
	for _, v := range translationUnits {
		for _, v := range v {
			switch x := v.(type) {
			case *FunctionDefinition:
				if x.NameID == idMain {
					ok = true
					break search
				}
			}
		}
	}
	if !ok {
		translationUnits = append(translationUnits, main)
	}
	l := newLinker(translationUnits)
	l.link()
	return l.out, nil
}

type extern struct {
	unit  int
	index int
}

type intern struct {
	NameID
	unit int
}

type linker struct {
	defined   map[extern]int    // unit, unit index: out index
	extern    map[NameID]extern // name: unit, unit index
	in        [][]Object
	intern    map[intern]int // name, unit: unit index
	out       []Object
	typeCache TypeCache
}

func newLinker(in [][]Object) *linker {
	l := &linker{
		defined:   map[extern]int{},
		extern:    map[NameID]extern{},
		in:        in,
		intern:    map[intern]int{},
		typeCache: TypeCache{},
	}

	l.collectSymbols()
	return l
}

func (l *linker) collectSymbols() {
	for unit, v := range l.in {
		for i, v := range v {
			switch x := v.(type) {
			case *DataDefinition:
				switch x.Linkage {
				case ExternalLinkage:
					switch ex, ok := l.extern[x.NameID]; {
					case ok:
						switch def := l.in[ex.unit][ex.index].(type) {
						case *DataDefinition:
							if x.TypeID != def.TypeID {
								panic("ir.linker internal error")
							}

							if x.Value != nil && def.Value == nil {
								def.Value = x.Value
							}
						default:
							panic(fmt.Errorf("ir.linker internal error %T", def))
						}
					default:
						l.extern[x.NameID] = extern{unit: unit, index: i}
					}
				case InternalLinkage:
					k := intern{x.NameID, unit}
					switch _, ok := l.intern[k]; {
					case ok:
						panic(fmt.Errorf("ir.linker TODO: %T(%v)", x, x))
					default:
						l.intern[k] = i
					}
				default:
					panic("ir.linker internal error")
				}
			case *FunctionDefinition:
				switch x.Linkage {
				case ExternalLinkage:
					switch ex, ok := l.extern[x.NameID]; {
					case ok:
						switch def := l.in[ex.unit][ex.index].(type) {
						case *FunctionDefinition:
							if x.TypeID != def.TypeID {
								panic("internal error")
							}

							if len(def.Body) != 1 {
								break
							}

							if _, ok := def.Body[0].(*Panic); ok {
								l.extern[x.NameID] = extern{unit: unit, index: i}
								break
							}

							panic(fmt.Errorf("%s: ir.linker internal error %s", x.Position, x.NameID))
						default:
							panic(fmt.Errorf("ir.linker internal error %T", def))
						}
					default:
						l.extern[x.NameID] = extern{unit: unit, index: i}
					}
				case InternalLinkage:
					k := intern{x.NameID, unit}
					switch _, ok := l.intern[k]; {
					case ok:
						panic(fmt.Errorf("TODO: %T(%v)", x, x))
					default:
						l.intern[k] = i
					}
				default:
					panic("ir.linker internal error")
				}
			default:
				panic(fmt.Errorf("ir.linker internal error: %T(%v)", x, x))
			}
		}
	}
}

func (l *linker) initializer(op *VariableDeclaration, v Value) {
	switch x := v.(type) {
	case
		*Complex128Value,
		*Float32Value,
		*Float64Value,
		*Int32Value,
		*Int64Value,
		*StringValue,
		*WideStringValue,
		nil:
		// ok
	case *AddressValue:
		switch x.Linkage {
		case ExternalLinkage:
			e, ok := l.extern[x.NameID]
			if !ok {
				panic(fmt.Errorf("%s: ir.linker undefined extern %s", op.Position, x.NameID))
			}

			x.Index = l.define(e)
		default:
			panic(fmt.Errorf("ir.linker internal error %s", x.Linkage))
		}
	case *CompositeValue:
		for _, v := range x.Values {
			l.initializer(op, v)
		}
	default:
		panic(fmt.Errorf("ir.linker internal error: %T %v", x, op))
	}
}

func (l *linker) checkCalls(p *[]Operation) {
	s := *p
	w := 0
	var static []int
	for i, v := range s {
		switch x := v.(type) {
		case *Arguments:
			if i != 0 {
				switch y := s[i-1].(type) {
				case *Global:
					switch l.out[y.Index].(type) {
					case *FunctionDefinition:
						x.FunctionPointer = false
						static = append(static, y.Index)
						s[w-1] = x
						continue
					}
				}
			}

			x.FunctionPointer = true
			static = append(static, -1)
		case *Call:
			panic("TODO")
		case *CallFP:
			n := len(static)
			index := static[n-1]
			static = static[:n-1]
			if index < 0 {
				break
			}

			t := l.typeCache.MustType(x.TypeID).(*PointerType).Element
			v = &Call{Arguments: x.Arguments, Index: index, TypeID: t.ID(), Position: x.Position, Comma: x.Comma}
		}

		s[w] = v
		w++
	}
	*p = s[:w]
}

func (l *linker) defineFunc(e extern, f *FunctionDefinition) (r int) {
	r = len(l.out)
	l.defined[e] = r
	l.out = append(l.out, f)
	unconvert(&f.Body)
	for ip, v := range f.Body {
		switch x := v.(type) {
		case
			*Add,
			*AllocResult,
			*And,
			*Argument,
			*Arguments,
			*BeginScope,
			*Bool,
			*Call,
			*CallFP,
			*Const32,
			*Const64,
			*ConstC128,
			*Convert,
			*Copy,
			*Cpl,
			*Div,
			*Drop,
			*Dup,
			*Element,
			*EndScope,
			*Eq,
			*Field,
			*FieldValue,
			*Geq,
			*Gt,
			*Jmp,
			*JmpP,
			*Jnz,
			*Jz,
			*Label,
			*Leq,
			*Load,
			*Lsh,
			*Lt,
			*Mul,
			*Neg,
			*Neq,
			*Nil,
			*Not,
			*Or,
			*Panic,
			*PostIncrement,
			*PreIncrement,
			*PtrDiff,
			*Rem,
			*Result,
			*Return,
			*Rsh,
			*Store,
			*StringConst,
			*Sub,
			*Switch,
			*Variable,
			*Xor:
			// nop
		case *Const:
			switch v := x.Value.(type) {
			case *AddressValue:
				switch v.Linkage {
				case ExternalLinkage:
					switch ex, ok := l.extern[v.NameID]; {
					case ok:
						v.Index = l.define(ex)
					default:
						panic("ir.linker TODO")
					}
				case InternalLinkage:
					switch ex, ok := l.intern[intern{v.NameID, e.unit}]; {
					case ok:
						v.Index = l.define(extern{unit: e.unit, index: ex})
					default:
						panic("ir.linker TODO")
					}
				default:
					panic("internal error")
				}
			default:
				panic(fmt.Errorf("%s: ir.linker %T", x.Position, v))
			}
		case *Global:
			switch x.Linkage {
			case ExternalLinkage:
				switch ex, ok := l.extern[x.NameID]; {
				case ok:
					x.Index = l.define(ex)
				default:
					var buf buffer.Bytes
					buf.Write(dict.S(idBuiltinPrefix))
					buf.Write(dict.S(int(x.NameID)))
					nm := NameID(dict.ID(buf.Bytes()))
					buf.Close()
					switch ex, ok := l.extern[nm]; {
					case ok:
						x.Index = l.define(ex)
					default:
						panic(fmt.Errorf("%v: ir.linker undefined external global %v", x.Position, x.NameID))
					}
				}
			case InternalLinkage:
				switch ex, ok := l.intern[intern{x.NameID, e.unit}]; {
				case ok:
					x.Index = l.define(extern{e.unit, ex})
				default:
					panic(fmt.Errorf("%v: ir.linker undefined global %v", x.Position, x.NameID))
				}
			default:
				panic("internal error")
			}
		case *VariableDeclaration:
			l.initializer(x, x.Value)
		default:
			panic(fmt.Errorf("ir.linker internal error: %T %s %#05x %v", x, f.NameID, ip, x))
		}
	}
	l.checkCalls(&f.Body)
	return r
}

func (l *linker) defineData(e extern, d *DataDefinition) (r int) {
	r = len(l.out)
	l.defined[e] = r
	l.out = append(l.out, d)
	var f func(Value)
	f = func(v Value) {
		switch x := v.(type) {
		case nil:
		// nop
		case *AddressValue:
			switch x.Linkage {
			case ExternalLinkage:
				switch ex, ok := l.extern[x.NameID]; {
				case ok:
					x.Index = l.define(ex)
				default:
					panic(fmt.Errorf("%s: ir.linker undefined external address %q", d.Position, x.NameID))
				}
			case InternalLinkage:
				switch ex, ok := l.intern[intern{x.NameID, e.unit}]; {
				case ok:
					x.Index = l.define(extern{unit: e.unit, index: ex})
				default:
					switch {
					case Testing:
						for k, v := range l.intern {
							fmt.Printf("%q: %v\n", k.NameID, v)
						}
						fallthrough
					default:
						panic(fmt.Errorf("%s: ir.linker undefined address %q", d.Position, x.NameID))
					}
				}
			default:
				panic("internal error")
			}
		case *CompositeValue:
			for _, v := range x.Values {
				f(v)
			}
		case
			*Complex128Value,
			*Complex64Value,
			*Float32Value,
			*Float64Value,
			*Int32Value,
			*Int64Value,
			*StringValue,
			*WideStringValue:
			// ok, nop.
		default:
			panic(fmt.Errorf("%v.%v: ir.linker internal error: %T", e.unit, e.index, x))
		}
	}
	f(d.Value)
	return r
}

func (l *linker) define(e extern) int {
	if i, ok := l.defined[e]; ok {
		return i
	}

	switch x := l.in[e.unit][e.index].(type) {
	case *DataDefinition:
		return l.defineData(e, x)
	case *FunctionDefinition:
		return l.defineFunc(e, x)
	default:
		panic(fmt.Errorf("ir.linker internal error: %T(%v)", x, x))
	}
}

func (l *linker) linkMain() {
	start, ok := l.extern[NameID(idStart)]
	if !ok {
		panic(fmt.Errorf("ir.linker _start undefined (forgotten crt0?)"))
	}
	l.define(start)
}

func (l *linker) link() {
	var a []int
	for k := range l.extern {
		a = append(a, int(k))
	}
	sort.Ints(a)
	for _, k := range a {
		l.define(l.extern[NameID(k)])
	}
}
