// Copyright 2023 Google LLC
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

package ast_test

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	chkdecls "github.com/authzed/cel-go/checker/decls"
	"github.com/authzed/cel-go/common/ast"
	"github.com/authzed/cel-go/common/overloads"
	"github.com/authzed/cel-go/common/types"
	"github.com/authzed/cel-go/common/types/ref"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestConvertAST(t *testing.T) {
	goAST := &ast.CheckedAST{
		Expr:       &exprpb.Expr{},
		SourceInfo: &exprpb.SourceInfo{},
		TypeMap: map[int64]*types.Type{
			1: types.BoolType,
			2: types.DynType,
		},
		ReferenceMap: map[int64]*ast.ReferenceInfo{
			1: ast.NewFunctionReference(overloads.LogicalNot),
			2: ast.NewIdentReference("TRUE", types.True),
		},
	}

	exprAST := &exprpb.CheckedExpr{
		Expr:       &exprpb.Expr{},
		SourceInfo: &exprpb.SourceInfo{},
		TypeMap: map[int64]*exprpb.Type{
			1: chkdecls.Bool,
			2: chkdecls.Dyn,
		},
		ReferenceMap: map[int64]*exprpb.Reference{
			1: {OverloadId: []string{overloads.LogicalNot}},
			2: {
				Name: "TRUE",
				Value: &exprpb.Constant{
					ConstantKind: &exprpb.Constant_BoolValue{BoolValue: true},
				},
			},
		},
	}

	checkedAST, err := ast.CheckedExprToCheckedAST(exprAST)
	if err != nil {
		t.Fatalf("CheckedExprToCheckedAST() failed: %v", err)
	}
	if !reflect.DeepEqual(checkedAST.ReferenceMap, goAST.ReferenceMap) ||
		!reflect.DeepEqual(checkedAST.TypeMap, goAST.TypeMap) {
		t.Errorf("conversion to AST did not produce identical results: got %v, wanted %v", checkedAST, goAST)
	}
	if !checkedAST.ReferenceMap[1].Equals(goAST.ReferenceMap[1]) ||
		!checkedAST.ReferenceMap[2].Equals(goAST.ReferenceMap[2]) {
		t.Error("converted reference info values not equal")
	}
	checkedExpr, err := ast.CheckedASTToCheckedExpr(goAST)
	if err != nil {
		t.Fatalf("CheckedASTToCheckedExpr() failed: %v", err)
	}
	if !proto.Equal(checkedExpr, exprAST) {
		t.Errorf("conversion to protobuf did not produce identical results: got %v, wanted %v", checkedExpr, exprAST)
	}
}

func TestReferenceInfoEquals(t *testing.T) {
	tests := []struct {
		name  string
		a     *ast.ReferenceInfo
		b     *ast.ReferenceInfo
		equal bool
	}{
		{
			name:  "single overload equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddBytes),
			equal: true,
		},
		{
			name:  "single overload not equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddDouble),
			equal: false,
		},
		{
			name:  "single and multiple overload not equal",
			a:     ast.NewFunctionReference(overloads.AddBytes),
			b:     ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble),
			equal: false,
		},
		{
			name:  "multiple overloads equal",
			a:     ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble),
			b:     ast.NewFunctionReference(overloads.AddDouble, overloads.AddBytes),
			equal: true,
		},
		{
			name:  "identifier reference equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("BYTES", nil),
			equal: true,
		},
		{
			name:  "identifier reference not equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("TRUE", nil),
			equal: false,
		},
		{
			name:  "identifier and constant reference not equal",
			a:     ast.NewIdentReference("BYTES", nil),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			equal: false,
		},
		{
			name:  "constant references equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			equal: true,
		},
		{
			name:  "constant references not equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewIdentReference("BYTES", types.Bytes("bytes-other")),
			equal: false,
		},
		{
			name:  "constant and overload reference not equal",
			a:     ast.NewIdentReference("BYTES", types.Bytes("bytes")),
			b:     ast.NewFunctionReference(overloads.AddDouble, overloads.AddBytes),
			equal: false,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.Equals(tc.b)
			if out != tc.equal {
				t.Errorf("%v.Equals(%v) got %v, wanted %v", tc.a, tc.b, out, tc.equal)
			}
		})
	}
}

func TestReferenceInfoAddOverload(t *testing.T) {
	add := ast.NewFunctionReference(overloads.AddBytes)
	add.AddOverload(overloads.AddDouble)
	if !add.Equals(ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble)) {
		t.Error("AddOverload() did not produce equal references")
	}
	add.AddOverload(overloads.AddDouble)
	if !add.Equals(ast.NewFunctionReference(overloads.AddBytes, overloads.AddDouble)) {
		t.Error("repeated AddOverload() did not produce equal references")
	}
}

func TestReferenceInfoToReferenceExprError(t *testing.T) {
	out, err := ast.ReferenceInfoToReferenceExpr(
		ast.NewIdentReference("SECOND", types.Duration{Duration: time.Duration(1) * time.Second}))
	if err == nil {
		t.Errorf("ReferenceInfoToReferenceExpr() got %v, wanted error", out)
	}
}

func TestReferenceExprToReferenceInfoError(t *testing.T) {
	out, err := ast.ReferenceExprToReferenceInfo(&exprpb.Reference{Value: &exprpb.Constant{}})
	if err == nil {
		t.Errorf("ReferenceExprToReferenceInfo() got %v, wanted error", out)
	}
}

func TestConvertVal(t *testing.T) {
	tests := []ref.Val{
		types.True,
		types.Bytes("bytes"),
		types.Double(3.2),
		types.Int(-1),
		types.NullValue,
		types.String("string"),
		types.Uint(27),
	}
	for _, tst := range tests {
		c, err := ast.ValToConstant(tst)
		if err != nil {
			t.Errorf("ValToConstant(%v) failed: %v", tst, err)
		}
		v, err := ast.ConstantToVal(c)
		if err != nil {
			t.Errorf("ValToConstant(%v) failed: %v", c, err)
		}
		if tst.Equal(v) != types.True {
			t.Errorf("roundtrip from %v to %v and back did not produce equal results, got %v, wanted %v", tst, c, v, tst)
		}
	}
}

func TestValToConstantError(t *testing.T) {
	out, err := ast.ValToConstant(types.Duration{Duration: time.Duration(10)})
	if err == nil {
		t.Errorf("ValToConstant() got %v, wanted error", out)
	}
}

func TestConstantToValError(t *testing.T) {
	out, err := ast.ConstantToVal(&exprpb.Constant{})
	if err == nil {
		t.Errorf("ConstantToVal() got %v, wanted error", out)
	}
}
