// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
)

// LinkMain returns all objects transitively referenced from function _start or
// an error, if any. Linking mutates the passed objects.
//
// LinkMain panics when passed no data.
//
// Note: Caller is responsible to ensure that all translationUnits were
// produced using the same memory model.
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
	defined map[extern]int    // unit, unit index: out index
	extern  map[NameID]extern // name: unit, unit index
	in      [][]Object
	intern  map[intern]int // name, unit: unit index
	out     []Object
}

func newLinker(in [][]Object) *linker {
	l := &linker{
		defined: map[extern]int{},
		extern:  map[NameID]extern{},
		in:      in,
		intern:  map[intern]int{},
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
					switch _, ok := l.extern[x.NameID]; {
					case ok:
						panic(fmt.Errorf("TODO: %T(%v)", x, x))
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
					switch _, ok := l.extern[x.NameID]; {
					case ok:
						panic("TODO")
					default:
						l.extern[x.NameID] = extern{unit: unit, index: i}
					}
				case InternalLinkage:
					panic("TODO")
				default:
					panic("internal error")
				}
			default:
				panic(fmt.Errorf("internal error: %T(%v)", x, x))
			}
		}
	}
}

func (l *linker) initializer(op *VariableDeclaration) {
	switch x := op.Value.(type) {
	case
		*Int32Value,
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
	default:
		panic(fmt.Errorf("internal error: %T", x))
	}
}

func (l *linker) defineFunc(e extern, f *FunctionDefinition) (r int) {
	r = len(l.out)
	l.defined[e] = r
	l.out = append(l.out, f)
	for ip, v := range f.Body {
		switch x := v.(type) {
		case
			*Add,
			*AllocResult,
			*Argument,
			*Arguments,
			*BeginScope,
			*Call,
			*Drop,
			*Dup,
			*Element,
			*EndScope,
			*Eq,
			*Field,
			*Int32Const,
			*Jmp,
			*Jnz,
			*Jz,
			*Label,
			*Leq,
			*Load,
			*Lt,
			*Mul,
			*Panic,
			*PostIncrement,
			*Result,
			*Return,
			*Store,
			*StringConst,
			*Sub,
			*Variable:
			// nop
		case *Extern:
			switch ex, ok := l.extern[x.NameID]; {
			case ok:
				x.Index = l.define(ex)
			default:
				panic("TODO")
			}
		case *VariableDeclaration:
			l.initializer(x)
		default:
			panic(fmt.Errorf("internal error: %T %s %#05x %v", x, f.NameID, ip, x))
		}
	}
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
		default:
			panic(fmt.Errorf("internal error: %T", x))
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
