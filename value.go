// Copyright 2017 The IR Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

var (
	_ Value = (*Int32Value)(nil)
	_ Value = (*StringValue)(nil)
)

type valuer struct{}

func (valuer) value() {}

// Value represents a constant expression used for initializing static data or
// function variables.
type Value interface {
	value()
}

// Int32Value is a declaration initializer constant of type int32.
type Int32Value struct {
	valuer
	Value int32
}

// StringValue is a declaration initializer constant of type string.
type StringValue struct {
	valuer
	StringID
}
