// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
	"math"

	"github.com/cznic/internal/buffer"
)

var (
	_ Type = (*ArrayType)(nil)
	_ Type = (*FunctionType)(nil)
	_ Type = (*PointerType)(nil)
	_ Type = (*StructOrUnionType)(nil)
	_ Type = (*TypeBase)(nil)
)

// Type represents an IR type.
//
// The type specifier syntax is defined using Extended Backus-Naur Form
// (EBNF[0]):
//
//	Type		= ArrayType | FunctionType | PointerType | StructType | TypeName | UnionType .
//	ArrayType	= "[" "0"..."9" { "0"..."9" } "]" Type .
//	FunctionType	= "func" "(" [ TypeList ] [ "..." ] ")" [ Type | "(" TypeList ")" ] .
//	PointerType	= "*" Type .
//	StructType	= "struct" "{" [ TypeList ] "}" .
//	TypeList	= Type { "," Type } .
//	TypeName	= "uint8" | "uint16" | "uint32" | "uint64"
//			| "int8" | "int16" | "int32" | "int64"
//			| "float32" | "float64" | "float128"
//			| "complex64" | "complex128" | complex256
//			| "uint0" | "uint8" | "uint16" | "uint32" | "uint64" .
//	UnionType	= "union" "{" [ TypeList ] "}" .
//
// No whitespace is allowed in type specifiers.
//
//  [0]: https://golang.org/ref/spec#Notation
//
// Type identity
//
// Two types are identical if their type specifiers are equivalent.
type Type interface {
	Equal(Type) bool
	ID() TypeID
	Kind() TypeKind
	Pointer() Type
	Signed() bool
}

// TypeBase collects fields common to all types.
type TypeBase struct {
	TypeKind
	TypeID
}

func (t *TypeBase) setID(id TypeID, p0 []byte, p *[]byte, c TypeCache, u Type) Type {
	if t.TypeKind == 0 {
		return nil
	}

	if t.TypeID != 0 {
		return t
	}

	if id == 0 {
		id = TypeID(dict.ID(p0[:len(p0)-len(*p)]))
	}
	t.TypeID = id
	c[id] = u
	return u
}

// String implements fmt.Stringer.
func (t *TypeBase) String() string { return t.TypeID.String() }

// Pointer implements Type.
func (t *TypeBase) Pointer() Type { return newPointerType(t) }

func newPointerType(t Type) Type {
	var buf buffer.Bytes
	buf.WriteByte('*')
	buf.Write(dict.S(int(t.ID())))
	return &PointerType{
		TypeBase: TypeBase{TypeKind: Pointer, TypeID: TypeID(dict.ID(buf.Bytes()))},
		Element:  t,
	}
}

// TypeID is a numeric identifier of a type specifier as registered in a global
// dictionary[0].
//
//  [0]: https://godoc.org/github.com/cznic/xc#pkg-variables
type TypeID int

// Equal implements Type.
func (t TypeID) Equal(u Type) bool { return t == u.ID() }

// Signed implements Type.
func (t TypeID) Signed() bool {
	switch t {
	case idInt8, idInt16, idInt32, idInt64:
		return true
	}

	return false
}

// ID implements Type.
func (t TypeID) ID() TypeID { return t }

// String implements fmt.Stringer.
func (t TypeID) String() string { return string(dict.S(int(t))) }

// GobDecode implements GobDecoder.
func (t *TypeID) GobDecode(b []byte) error {
	*t = TypeID(dict.ID(b))
	return nil
}

// GobEncode implements GobEncoder.
func (t TypeID) GobEncode() ([]byte, error) {
	return append([]byte(nil), dict.S(int(t))...), nil
}

// ArrayType represents a collection of items that can be selected by index.
type ArrayType struct {
	TypeBase
	Item  Type
	Items int64
}

// Pointer implements Type.
func (t *ArrayType) Pointer() Type { return newPointerType(t) }

// FunctionType represents a function, its possibly variadic, optional
// arguments and results.
type FunctionType struct {
	TypeBase
	Arguments []Type
	Results   []Type
	Variadic  bool // C-variadic.
}

// Pointer implements Type.
func (t *FunctionType) Pointer() Type { return newPointerType(t) }

// PointerType represents a pointer to an element, an instance of another type.
type PointerType struct {
	TypeBase
	Element Type
}

// Pointer implements Type.
func (t *PointerType) Pointer() Type { return newPointerType(t) }

// StructOrUnionType represents a collection of fields that can be selected by
// name.
type StructOrUnionType struct {
	TypeBase
	Fields []Type
}

// Pointer implements Type.
func (t *StructOrUnionType) Pointer() Type { return newPointerType(t) }

// TypeCache maps TypeIDs to  Types. Use TypeCache{} to create a ready to use
// TypeCache value.
type TypeCache map[TypeID]Type

func (c TypeCache) c(p *[]byte) tok {
	s := *p
	if len(s) == 0 {
		return tokEOF
	}

	return tok(s[0])
}

func (c TypeCache) n(p *[]byte) tok {
	s := *p
	if len(s) == 0 {
		return tokEOF
	}

	s = s[1:]
	*p = s
	if len(s) == 0 {
		return tokEOF
	}

	return tok(s[0])
}

func (c TypeCache) lex2(p *[]byte) (tok, int64) {
	t := c.c(p)
	switch t {
	case '*', '(', ')', '{', '}', ',', '[', ']':
		c.n(p)
		return t, 0
	case '.':
		if c.n(p) == '.' && c.n(p) == '.' {
			c.n(p)
			return tokEllipsis, 0
		}
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		n := int64(t - '0')
		for {
			t := c.n(p)
			if t < '0' || t > '9' {
				return tokNumber, n
			}

			n = 10*n + int64(t-'0')
			if n < 0 || n > math.MaxInt64 {
				return tokIllegal, 0
			}
		}
	case 'c':
		if c.n(p) == 'o' && c.n(p) == 'm' && c.n(p) == 'p' && c.n(p) == 'l' && c.n(p) == 'e' && c.n(p) == 'x' {
			switch c.n(p) {
			case '1':
				if c.n(p) == '2' && c.n(p) == '8' {
					c.n(p)
					return tokC128, 0
				}
			case '2':
				if c.n(p) == '5' && c.n(p) == '6' {
					c.n(p)
					return tokC256, 0
				}
			case '6':
				if c.n(p) == '4' {
					c.n(p)
					return tokC64, 0
				}
			}
		}
	case 'f':
		switch c.n(p) {
		case 'l':
			if c.n(p) == 'o' && c.n(p) == 'a' && c.n(p) == 't' {
				switch c.n(p) {
				case '1':
					if c.n(p) == '2' && c.n(p) == '8' {
						c.n(p)
						return tokF128, 0
					}
				case '3':
					if c.n(p) == '2' {
						c.n(p)
						return tokF32, 0
					}
				case '6':
					if c.n(p) == '4' {
						c.n(p)
						return tokF64, 0
					}
				}
			}
		case 'u':
			if c.n(p) == 'n' && c.n(p) == 'c' {
				c.n(p)
				return tokFunc, 0
			}
		}
	case 'i':
		if c.n(p) == 'n' && c.n(p) == 't' {
			switch c.n(p) {
			case '1':
				if c.n(p) == '6' {
					c.n(p)
					return tokI16, 0
				}
			case '3':
				if c.n(p) == '2' {
					c.n(p)
					return tokI32, 0
				}
			case '6':
				if c.n(p) == '4' {
					c.n(p)
					return tokI64, 0
				}
			case '8':
				c.n(p)
				return tokI8, 0
			}
		}
	case 's':
		if c.n(p) == 't' && c.n(p) == 'r' && c.n(p) == 'u' && c.n(p) == 'c' && c.n(p) == 't' {
			c.n(p)
			return tokStruct, 0
		}
	case 'u':
		switch c.n(p) {
		case 'i':
			if c.n(p) == 'n' && c.n(p) == 't' {
				switch c.n(p) {
				case '1':
					if c.n(p) == '6' {
						c.n(p)
						return tokU16, 0
					}
				case '3':
					if c.n(p) == '2' {
						c.n(p)
						return tokU32, 0
					}
				case '6':
					if c.n(p) == '4' {
						c.n(p)
						return tokU64, 0
					}
				case '8':
					c.n(p)
					return tokU8, 0
				}
			}
		case 'n':
			if c.n(p) == 'i' && c.n(p) == 'o' && c.n(p) == 'n' {
				c.n(p)
				return tokUnion, 0
			}
		}
	case tokEOF:
		return t, 0
	}

	c.n(p)
	return tokIllegal, 0
}

func (c TypeCache) lex(p *[]byte) tok {
	t, _ := c.lex2(p)
	return t
}

func (c TypeCache) parseTypeList(p *[]byte) ([]Type, error) {
	var l []Type
	for {
		switch c.c(p) {
		case '}':
			return l, nil
		}

		t, err := c.parse(p, 0)
		if err != nil {
			return nil, err
		}

		l = append(l, t)
		switch c.c(p) {
		case ',':
			c.n(p)
			if c.c(p) == '.' {
				return l, nil
			}
		default:
			return l, nil
		}
	}
}

func (c TypeCache) parseResults(p *[]byte) ([]Type, error) {
	switch c.c(p) {
	case tokEOF, ',', ')', '}':
		return nil, nil
	case '(':
		c.n(p)
		l, err := c.parseTypeList(p)
		if err != nil {
			return nil, err
		}

		if c.lex(p) == ')' {
			return l, nil
		}

		return nil, fmt.Errorf("expected ')'")
	default:
		t, err := c.parse(p, 0)
		if err != nil {
			return nil, err
		}

		return []Type{t}, nil
	}
}

func (c TypeCache) parseFunc(p *[]byte) (*FunctionType, error) {
	if c.lex(p) != '(' {
		return nil, fmt.Errorf("expected '('")
	}

	var arguments []Type
	switch c.c(p) {
	case ')', '.':
		// nop
	default:
		var err error
		if arguments, err = c.parseTypeList(p); err != nil {
			return nil, err
		}
	}

	var variadic bool
more:
	switch tk := c.lex(p); tk {
	case ')':
		results, err := c.parseResults(p)
		if err != nil {
			return nil, err
		}

		return &FunctionType{
			Arguments: arguments,
			Results:   results,
			TypeBase:  TypeBase{TypeKind: Function},
			Variadic:  variadic,
		}, nil
	case tokEllipsis:
		if variadic {
			return nil, fmt.Errorf("unexpected '%s'", tk)
		}

		variadic = true
		goto more
	default:
		return nil, fmt.Errorf("unexpected '%s'", tk)
	}
}

func (c TypeCache) parse(p *[]byte, id TypeID) (Type, error) {
	p0 := *p
	tk := c.lex(p)
	k := Union
	switch tk {
	case tokI8:
		t := &TypeBase{TypeKind: Int8}
		return t.setID(id, p0, p, c, t), nil
	case tokI16:
		t := &TypeBase{TypeKind: Int16}
		return t.setID(id, p0, p, c, t), nil
	case tokI32:
		t := &TypeBase{TypeKind: Int32}
		return t.setID(id, p0, p, c, t), nil
	case tokI64:
		t := &TypeBase{TypeKind: Int64}
		return t.setID(id, p0, p, c, t), nil
	case tokU8:
		t := &TypeBase{TypeKind: Uint8}
		return t.setID(id, p0, p, c, t), nil
	case tokU16:
		t := &TypeBase{TypeKind: Uint16}
		return t.setID(id, p0, p, c, t), nil
	case tokU32:
		t := &TypeBase{TypeKind: Uint32}
		return t.setID(id, p0, p, c, t), nil
	case tokU64:
		t := &TypeBase{TypeKind: Uint64}
		return t.setID(id, p0, p, c, t), nil
	case tokF32:
		t := &TypeBase{TypeKind: Float32}
		return t.setID(id, p0, p, c, t), nil
	case tokF64:
		t := &TypeBase{TypeKind: Float64}
		return t.setID(id, p0, p, c, t), nil
	case tokF128:
		t := &TypeBase{TypeKind: Float128}
		return t.setID(id, p0, p, c, t), nil
	case tokC64:
		t := &TypeBase{TypeKind: Complex64}
		return t.setID(id, p0, p, c, t), nil
	case tokC128:
		t := &TypeBase{TypeKind: Complex128}
		return t.setID(id, p0, p, c, t), nil
	case tokC256:
		t := &TypeBase{TypeKind: Complex256}
		return t.setID(id, p0, p, c, t), nil
	case '*':
		element, err := c.parse(p, 0)
		if err != nil {
			return nil, err
		}

		t := &PointerType{
			Element:  element,
			TypeBase: TypeBase{TypeKind: Pointer},
		}
		return t.setID(id, p0, p, c, t), nil
	case '[':
		if tk, n := c.lex2(p); tk == tokNumber && c.lex(p) == ']' {
			item, err := c.parse(p, 0)
			if err != nil {
				return nil, err
			}

			t := &ArrayType{
				Item:     item,
				Items:    n,
				TypeBase: TypeBase{TypeKind: Array},
			}
			return t.setID(id, p0, p, c, t), nil
		}
	case tokFunc:
		t, err := c.parseFunc(p)
		if err != nil {
			return nil, err
		}

		return t.setID(id, p0, p, c, t), nil
	case tokStruct:
		k = Struct
		fallthrough
	case tokUnion:
		if c.lex(p) != '{' {
			return nil, fmt.Errorf("expected '{'")
		}

		l, err := c.parseTypeList(p)
		if err != nil {
			return nil, err
		}

		if c.lex(p) != '}' {
			return nil, fmt.Errorf("expected '}'")
		}

		t := &StructOrUnionType{TypeBase: TypeBase{TypeKind: k}, Fields: l}
		return t.setID(id, p0, p, c, t), nil
	}
	return nil, fmt.Errorf("unexpected %q (%q)", tk, p0)
}

// Type returns the type identified by id or an error, if any. If the cache has
// already a value for id, it is returned.  Otherwise the type specifier
// denoted by id is parsed.
func (c TypeCache) Type(id TypeID) (Type, error) {
	if t := c[id]; t != nil {
		return t, nil
	}

	b := dict.S(int(id))
	t, err := c.parse(&b, id)
	if err != nil {
		return nil, err
	}

	if tk := c.lex(&b); tk != tokEOF {
		return nil, fmt.Errorf("unexpected token %q", tk)
	}

	c[id] = t
	return t, nil
}

// MustType is like Type but panics on error.
func (c TypeCache) MustType(id TypeID) Type {
	t, err := c.Type(id)
	if err != nil {
		panic(fmt.Errorf("%q: %v", id, err))
	}

	return t
}
