// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bench defines benchmark test cases and utilities for CEL parsers.
package bench

import (
	"strings"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"
)

// ParseResult indicates whether a parse is expected to succeed or fail.
type ParseResult int

const (
	// ParseResultSuccess indicates an expression that is expected to parse without error.
	ParseResultSuccess ParseResult = iota
	// ParseResultError indicates an expression that is expected to produce parse error(s).
	ParseResultError
)

// TestCase represents an expression to parse and the expected parse result.
type TestCase struct {
	Expr   string
	Result ParseResult
}

// ErrorCase returns a TestCase expecting a parse error.
func ErrorCase(expr string) TestCase {
	return TestCase{
		Expr:   expr,
		Result: ParseResultError,
	}
}

// SuccessCase returns a TestCase expecting successful parsing.
func SuccessCase(expr string) TestCase {
	return TestCase{
		Expr:   expr,
		Result: ParseResultSuccess,
	}
}

// Category represents a named group of test cases for benchmarking and verification.
type Category struct {
	Name  string
	Cases []TestCase
}

// OptMapMacro expands `m.optMap(v, f)` into a conditional comprehension.
var OptMapMacro = parser.NewReceiverMacro("optMap", 2, optMapExpander)

func optMapExpander(meh parser.ExprHelper, target ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	varIdent := args[0]
	varName := ""
	switch varIdent.Kind() {
	case ast.IdentKind:
		varName = varIdent.AsIdent()
	default:
		return nil, meh.NewError(varIdent.ID(), "optMap() variable name must be a simple identifier")
	}
	mapExpr := args[1]
	return meh.NewCall(
		operators.Conditional,
		meh.NewMemberCall("hasValue", target),
		meh.NewCall("optional.of",
			meh.NewComprehension(
				meh.NewList(),
				"#unused",
				varName,
				meh.NewMemberCall("value", meh.Copy(target)),
				meh.NewLiteral(types.False),
				meh.NewIdent(varName),
				mapExpr,
			),
		),
		meh.NewCall("optional.none"),
	), nil
}

// GetCategories returns benchmark and correctness test cases organized by category.
func GetCategories() []Category {
	return categories
}

// GetTestCases returns benchmark and correctness test cases flattened across all categories.
func GetTestCases() []TestCase {
	var allCases []TestCase
	for _, cat := range categories {
		allCases = append(allCases, cat.Cases...)
	}
	return allCases
}

var categories = []Category{
	// Simple: common, representative CEL expressions covering basic syntax, operators, calls, and literals
	{
		Name: "Simple",
		Cases: []TestCase{
			SuccessCase("x * 2 + y / 3"),
			SuccessCase(`foo.bar.baz(1, 2, "abc")`),
			SuccessCase(`a > 5 && b < 10 || c == "xyz"`),
			SuccessCase("x ? y : z"),
			SuccessCase(`{"foo": 1, "bar": [2, 3]}`),
			SuccessCase("a[b]"),
			SuccessCase("a.b.c"),
			SuccessCase("a.`b-c`"),
			SuccessCase("\"\\a\\b\\f\\n\\r\\t\\v'\\\"\\\\ Legal escapes \\u2764\""),
		},
	},

	// Complex: expressions with deep chaining, nesting, precedence, and complex structures
	{
		Name: "Complex",
		Cases: []TestCase{
			SuccessCase("a" + strings.Repeat(" + a", 49)),
			SuccessCase("a" + strings.Repeat(" || a", 49)),
			SuccessCase("a" + strings.Repeat(".f", 49)),
			SuccessCase(strings.Repeat("(", 20) + "a" + strings.Repeat(")", 20)),
			SuccessCase(`SomeMessage{foo: 5, bar: "xyz"}`),
			SuccessCase("1 + 2 * 3 - 1 / 2 == 6 % 1"),
			SuccessCase("[] + [1, 2, 3] + [4]"),
		},
	},

	// Macros: standard and receiver comprehension macros, optional syntax traversal
	{
		Name: "Macros",
		Cases: []TestCase{
			SuccessCase("has(m.f)"),
			SuccessCase("[1, 2, 3].all(x, x > 0)"),
			SuccessCase("m.map(v, v * 2)"),
			SuccessCase("m.filter(v, v > 0)"),
			SuccessCase("m.exists_one(v, v == 1)"),
			SuccessCase("x.filter(y, y.exists(z, has(z.a)))"),
			SuccessCase("a.?b[?0] && a[?c]"),
			SuccessCase("m.optMap(v, v + 1)"),
		},
	},

	// Errors: representative syntax errors, invalid tokens, keywords, and unclosed delimiters
	{
		Name: "Errors",
		Cases: []TestCase{
			ErrorCase("x * 2 + y /"),
			ErrorCase(`foo.bar.baz(1, 2, "abc"`),
			ErrorCase("a > 5 && && b < 10"),
			ErrorCase(`{"foo": 1, "bar": [2, 3`),
			ErrorCase("1 + $"),
			ErrorCase("break"),
			ErrorCase(`"\xFh"`),
			ErrorCase("a" + strings.Repeat(" + a", 49) + " +"),
			ErrorCase(strings.Repeat("(", 20) + "a"),
			ErrorCase("f(*" + strings.Repeat(", *", 9) + ")"),
		},
	},
}
