// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/token"
)

var (
	_ Operation = (*Arguments)(nil)
	_ Operation = (*BeginScope)(nil)
	_ Operation = (*EndScope)(nil)
	_ Operation = (*Result)(nil)
	_ Operation = (*Variable)(nil)
)

// Operation is a unit of execution.
type Operation interface {
	opcode() opcode
}

type Arguments struct {
	token.Position
}

func (*Arguments) opcode() opcode { return arguments }

type BeginScope struct {
	token.Position
}

func (*BeginScope) opcode() opcode { return beginScope }

type EndScope struct {
	token.Position
}

func (*EndScope) opcode() opcode { return beginScope }

type Result struct {
	TypeID
	TypeName NameID
	token.Position
}

func (*Result) opcode() opcode { return result }

type Variable struct {
	Index int // 0-based function index.
	NameID
	TypeID
	TypeName NameID
	Value
	token.Position
}

func (*Variable) opcode() opcode { return variable }
