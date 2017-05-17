// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"

	"github.com/cznic/internal/buffer"
)

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
				err = fmt.Errorf("%v", x)
			}
		}()
	}
	l := newLinker(translationUnits)
	l.linkMain()
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
								panic("internal error")
							}

							if x.Value != nil && def.Value == nil {
								def.Value = x.Value
							}
						default:
							panic(fmt.Errorf("internal error %T", def))
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
					panic("internal error")
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

							if len(def.Body) == 1 {
								if _, ok := def.Body[0].(*Panic); ok {
									l.extern[x.NameID] = extern{unit: unit, index: i}
									break
								}
							}

							panic(fmt.Errorf("%s: internal error %s", x.Position, x.NameID))
						default:
							panic(fmt.Errorf("internal error %T", def))
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
					panic("internal error")
				}
			default:
				panic(fmt.Errorf("internal error: %T(%v)", x, x))
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
				panic(fmt.Errorf("%s: undefined extern %s", op.Position, x.NameID))
			}

			x.Index = l.define(e)
		default:
			panic(fmt.Errorf("internal error %s", x.Linkage))
		}
	case *CompositeValue:
		for _, v := range x.Values {
			l.initializer(op, v)
		}
	default:
		panic(fmt.Errorf("internal error: %T %v", x, op))
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
						panic("TODO")
					}
				case InternalLinkage:
					switch ex, ok := l.intern[intern{v.NameID, e.unit}]; {
					case ok:
						v.Index = l.define(extern{unit: e.unit, index: ex})
					default:
						panic("TODO")
					}
				default:
					panic("internal error")
				}
			default:
				panic(fmt.Errorf("%s: %T", x.Position, v))
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
						panic(fmt.Errorf("%v: undefined %v", x.Position, x.NameID))
					}
				}
			case InternalLinkage:
				switch ex, ok := l.intern[intern{x.NameID, e.unit}]; {
				case ok:
					x.Index = l.define(extern{e.unit, ex})
				default:
					panic(fmt.Errorf("%v: undefined %v", x.Position, x.NameID))
				}
			default:
				panic("internal error")
			}
		case *VariableDeclaration:
			l.initializer(x, x.Value)
		default:
			panic(fmt.Errorf("internal error: %T %s %#05x %v", x, f.NameID, ip, x))
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
					panic(fmt.Errorf("%s: undefined %q", d.Position, x.NameID))
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
						panic(fmt.Errorf("%s: undefined %q", d.Position, x.NameID))
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
			panic(fmt.Errorf("%v.%v: internal error: %T", e.unit, e.index, x))
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
		panic(fmt.Errorf("internal error: %T(%v)", x, x))
	}
}

func (l *linker) linkMain() {
	start, ok := l.extern[NameID(idStart)]
	if !ok {
		panic(fmt.Errorf("_start undefined (forgotten crt0?)"))
	}
	l.define(start)
}
