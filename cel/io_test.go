// Copyright 2019 Google LLC
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

package cel

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/authzed/cel-go/checker/decls"
	celast "github.com/authzed/cel-go/common/ast"
	"github.com/authzed/cel-go/common/operators"
	"github.com/authzed/cel-go/common/types"
	"github.com/authzed/cel-go/common/types/ref"

	proto3pb "github.com/authzed/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestRefValueToValueRoundTrip(t *testing.T) {
	tests := []struct {
		value any
	}{
		{value: types.NullValue},
		{value: types.Bool(true)},
		{value: types.String("abc")},
		{value: types.Double(0.0)},
		{value: types.Bytes(make([]byte, 0, 5))},
		{value: types.Int(0)},
		{value: types.Uint(0)},
		{value: types.Duration{Duration: time.Hour}},
		{value: types.Timestamp{Time: time.Unix(0, 0)}},
		{value: types.IntType},
		{value: types.NewOpaqueType("CustomType")},
		{value: map[int64]int64{1: 1}},
		{value: []any{true, "abc"}},
		{value: &proto3pb.TestAllTypes{SingleString: "abc"}},
	}

	env, err := NewEnv(Types(&proto3pb.TestAllTypes{}))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	for i, tst := range tests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]%v", i, tc.value), func(t *testing.T) {
			refVal := env.TypeAdapter().NativeToValue(tc.value)
			val, err := RefValueToValue(refVal)
			if err != nil {
				t.Fatalf("RefValueToValue(%v) failed with error: %v", refVal, err)
			}
			actual, err := ValueToRefValue(env.TypeAdapter(), val)
			if err != nil {
				t.Fatalf("ValueToRefValue() failed: %v", err)
			}
			if refVal.Equal(actual) != types.True {
				t.Errorf("got val %v, wanted %v", actual, refVal)
			}
		})
	}
}

func TestAstToProto(t *testing.T) {
	stdEnv, _ := NewEnv(Declarations(
		decls.NewVar("a", decls.Dyn),
		decls.NewVar("b", decls.Dyn),
	))
	ast, iss := stdEnv.Parse("a + b")
	if iss.Err() != nil {
		t.Fatalf("Parse('a + b') failed: %v", iss.Err())
	}
	parsed, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatalf("AstToParsedExpr() failed: %v", err)
	}
	ast2 := ParsedExprToAst(parsed)
	if !proto.Equal(ast2.Expr(), ast.Expr()) {
		t.Errorf("got expr %v, wanted %v", ast2, ast)
	}
	ast3 := ParsedExprToAstWithSource(parsed, ast.Source())
	if !proto.Equal(ast3.Expr(), ast.Expr()) {
		t.Errorf("got expr %v, wanted %v", ast2, ast)
	}
	if ast3.Source() != ast.Source() {
		t.Errorf("got source %v, wanted %v", ast3.Source(), ast.Source())
	}

	_, err = AstToCheckedExpr(ast)
	if err == nil {
		t.Error("expected error converting unchecked ast")
	}
	ast, iss = stdEnv.Check(ast)
	if iss != nil && iss.Err() != nil {
		t.Fatalf("stdEnv.Check(ast) failed: %v", iss.Err())
	}
	checked, err := AstToCheckedExpr(ast)
	if err != nil {
		t.Fatalf("AstToCheckedExpr(ast) failed: %v", err)
	}
	ast4 := CheckedExprToAst(checked)
	if !proto.Equal(ast4.Expr(), ast.Expr()) {
		t.Fatalf("got ast %v, wanted %v", ast4, ast)
	}
	ast5, err := CheckedExprToAstWithSource(checked, ast.Source())
	if err != nil {
		t.Fatalf("CheckedExprToAstWithSource() failed: %v", err)
	}
	if !proto.Equal(ast5.Expr(), ast.Expr()) {
		t.Errorf("got expr %v, wanted %v", ast5, ast)
	}
	if ast5.Source() != ast.Source() {
		t.Errorf("got source %v, wanted %v", ast5.Source(), ast.Source())
	}
}

func TestAstToString(t *testing.T) {
	stdEnv, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	in := "a + b - (c ? (-d + 4) : e)"
	ast, iss := stdEnv.Parse(in)
	if iss.Err() != nil {
		t.Fatalf("stdEnv.Parse(%q) failed: %v", in, iss.Err())
	}
	expr, err := AstToString(ast)
	if err != nil {
		t.Fatalf("AstToString(ast) failed: %v", err)
	}
	if expr != in {
		t.Errorf("got %v, wanted %v", expr, in)
	}
}

func TestExprToString(t *testing.T) {
	stdEnv, err := NewEnv(EnableMacroCallTracking())
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	in := "[a, b].filter(i, (i > 0) ? (-i + 4) : i)"
	ast, iss := stdEnv.Parse(in)
	if iss.Err() != nil {
		t.Fatalf("stdEnv.Parse(%q) failed: %v", in, iss.Err())
	}
	expr, err := ExprToString(ast.NativeRep().Expr(), ast.NativeRep().SourceInfo())
	if err != nil {
		t.Fatalf("ExprToString(ast) failed: %v", err)
	}
	if expr != in {
		t.Errorf("got %v, wanted %v", expr, in)
	}

	// Test sub-expression unparsing.
	navExpr := celast.NavigateAST(ast.NativeRep())
	condExpr := celast.MatchDescendants(navExpr, celast.FunctionMatcher(operators.Conditional))[0]
	want := `(i > 0) ? (-i + 4) : i`
	expr, err = ExprToString(condExpr, ast.NativeRep().SourceInfo())
	if err != nil {
		t.Fatalf("ExprToString(ast) failed: %v", err)
	}
	if expr != want {
		t.Errorf("got %v, wanted %v", expr, want)
	}

	// Also passes with a nil source info, but only because the sub-expr doesn't contain macro calls.
	expr, err = ExprToString(condExpr, nil)
	if err != nil {
		t.Fatalf("ExprToString(ast) failed: %v", err)
	}
	if expr != want {
		t.Errorf("got %v, wanted %v", expr, want)
	}

	// Fails do to missing macro information.
	_, err = ExprToString(ast.NativeRep().Expr(), nil)
	if err == nil {
		t.Error("ExprToString() succeeded, wanted error")
	}
}

func TestRefValToExprValue(t *testing.T) {
	tests := []struct {
		name        string
		refVal      ref.Val
		expectError bool
	}{
		{
			name:        "unknown value",
			refVal:      types.NewUnknown(1, nil),
			expectError: false,
		},
		{
			name:        "error value",
			refVal:      types.NewErr("test error"),
			expectError: false,
		},
		{
			name:        "bool value",
			refVal:      types.Bool(true),
			expectError: false,
		},
		{
			name:        "string value",
			refVal:      types.String("test"),
			expectError: false,
		},
		{
			name:        "int value",
			refVal:      types.Int(1),
			expectError: false,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			exprVal, err := ExprValueAsProto(tc.refVal)
			if tc.expectError {
				if err == nil {
					t.Errorf("RefValToExprValue(%v) expected error, got %v", tc.refVal, exprVal)
				}
			} else {
				if err != nil {
					t.Errorf("RefValToExprValue(%v) failed with error: %v", tc.refVal, err)
				}
				if exprVal == nil {
					t.Errorf("RefValToExprValue(%v) expected value, got nil", tc.refVal)
				}
			}
		})
	}
}

func TestAstToStringNil(t *testing.T) {
	expr, err := AstToString(nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported expr") {
		t.Errorf("env.AstToString() got (%v, %v) wanted unsupported expr error", expr, err)
	}
}

func TestAstToCheckedExprNil(t *testing.T) {
	expr, err := AstToCheckedExpr(nil)
	if err == nil || !strings.Contains(err.Error(), "cannot convert unchecked ast") {
		t.Errorf("env.AstToCheckedExpr() got (%v, %v) wanted conversion error", expr, err)
	}
}

func TestAstToParsedExprNil(t *testing.T) {
	expr, err := AstToParsedExpr(nil)
	if err != nil {
		t.Errorf("env.AstToParsedExpr() got (%v, %v) wanted conversion error", expr, err)
	}
}

func TestCheckedExprToAstConstantExpr(t *testing.T) {
	stdEnv, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	in := "10"
	ast, iss := stdEnv.Compile(in)
	if iss.Err() != nil {
		t.Fatalf("stdEnv.Compile(%q) failed: %v", in, iss.Err())
	}
	expr, err := AstToCheckedExpr(ast)
	if err != nil {
		t.Fatalf("AstToCheckedExpr(ast) failed: %v", err)
	}
	ast2 := CheckedExprToAst(expr)
	if !proto.Equal(ast2.Expr(), ast.Expr()) {
		t.Fatalf("got ast %v, wanted %v", ast2, ast)
	}
}

func TestCheckedExprToAstMissingInfo(t *testing.T) {
	stdEnv, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	in := "10"
	ast, iss := stdEnv.Parse(in)
	if iss.Err() != nil {
		t.Fatalf("stdEnv.Compile(%q) failed: %v", in, iss.Err())
	}
	if ast.ResultType() != decls.Dyn {
		t.Fatalf("ast.ResultType() got %v, wanted 'dyn'", ast.ResultType())
	}
	expr, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatalf("AstToParsedExpr(ast) failed: %v", err)
	}
	checkedExpr := &exprpb.CheckedExpr{
		TypeMap: map[int64]*exprpb.Type{expr.GetExpr().GetId(): decls.Int},
		Expr:    expr.GetExpr(),
	}
	ast2 := CheckedExprToAst(checkedExpr)
	if !ast2.IsChecked() {
		t.Fatal("CheckedExprToAst() did not produce a 'checked' ast")
	}
	if ast2.ResultType() != decls.Int {
		t.Fatalf("ast2.ResultType() got %v, wanted 'int'", ast.ResultType())
	}
}

func TestRefValueToValue_Error(t *testing.T) {
	_, err := RefValueToValue(types.NewErr("test error"))
	if err == nil {
		t.Error("RefValueToValue(err) should return error")
	}
}

func TestExprValueAsAlphaProto(t *testing.T) {
	// Exercise the ExprValueAsAlphaProto wrapper (0% coverage)
	res, err := ExprValueAsAlphaProto(types.Int(42))
	if err != nil {
		t.Fatalf("ExprValueAsAlphaProto() failed: %v", err)
	}
	if res.GetValue() == nil {
		t.Error("ExprValueAsAlphaProto() returned nil value")
	}
}

func TestRefValToExprValue_Wrappers(t *testing.T) {
	// Exercise the RefValToExprValue wrapper (0% coverage)
	res, err := RefValToExprValue(types.String("hello"))
	if err != nil {
		t.Fatalf("RefValToExprValue() failed: %v", err)
	}
	if res.GetValue() == nil {
		t.Error("RefValToExprValue() returned nil value")
	}
}
