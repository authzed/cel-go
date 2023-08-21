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
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/authzed/cel-go/checker"
	"github.com/authzed/cel-go/common"
	"github.com/authzed/cel-go/common/ast"
	"github.com/authzed/cel-go/common/containers"
	"github.com/authzed/cel-go/common/decls"
	"github.com/authzed/cel-go/common/stdlib"
	"github.com/authzed/cel-go/common/types"
	"github.com/authzed/cel-go/parser"

	"github.com/authzed/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestNavigateExpr(t *testing.T) {
	tests := []struct {
		expr           string
		descedantCount int
		callCount      int
	}{
		{
			expr:           `'a' == 'b'`,
			descedantCount: 3,
			callCount:      1,
		},
		{
			expr:           `'a'.size()`,
			descedantCount: 2,
			callCount:      1,
		},
		{
			expr:           `[1, 2, 3]`,
			descedantCount: 4,
			callCount:      0,
		},
		{
			expr:           `[1, 2, 3][0]`,
			descedantCount: 6,
			callCount:      1,
		},
		{
			expr:           `{1u: 'hello'}`,
			descedantCount: 3,
			callCount:      0,
		},
		{
			expr:           `{'hello': 'world'}.hello`,
			descedantCount: 4,
			callCount:      0,
		},
		{
			expr:           `type(1) == int`,
			descedantCount: 4,
			callCount:      2,
		},
		{
			expr:           `google.expr.proto3.test.TestAllTypes{single_int32: 1}`,
			descedantCount: 2,
			callCount:      0,
		},
		{
			expr:           `[true].exists(i, i)`,
			descedantCount: 11, // 2 for iter range, 1 for accu init, 4 for loop condition, 3 for loop step, 1 for result
			callCount:      3,  // @not_strictly_false(!result), accu_init || i
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.expr, func(t *testing.T) {
			checked := mustTypeCheck(t, tc.expr)
			nav := ast.NavigateCheckedAST(checked)
			descendants := ast.MatchDescendants(nav, ast.AllMatcher())
			if len(descendants) != tc.descedantCount {
				t.Errorf("ast.MatchDescendants(%v) got %d descendants, wanted %d", checked.Expr, len(descendants), tc.descedantCount)
			}
			calls := ast.MatchSubset(descendants, ast.KindMatcher(ast.CallKind))
			if len(calls) != tc.callCount {
				t.Errorf("ast.MatchSubset(%v) got %d calls, wanted %d", checked.Expr, len(calls), tc.callCount)
			}
		})
	}
}

func TestNavigateExprNilSafety(t *testing.T) {
	tests := []struct {
		name string
		e    ast.NavigableExpr
	}{
		{
			name: "nil expr",
			e:    ast.NavigateCheckedAST(&ast.CheckedAST{TypeMap: map[int64]*types.Type{}}),
		},
		{
			name: "empty expr",
			e:    ast.NavigateCheckedAST(&ast.CheckedAST{Expr: &exprpb.Expr{}, TypeMap: map[int64]*types.Type{}}),
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e := tc.e
			if e.Kind() != ast.UnspecifiedKind {
				t.Errorf("Kind() got %v, wanted unspecified kind", e.Kind())
			}
			if e.ID() != 0 {
				t.Errorf("ID() got %d, wanted 0", e.ID())
			}
			if e.Type() != types.DynType {
				t.Errorf("Type() got %v, wanted types.DynType", e.Type())
			}
			if p, found := e.Parent(); found {
				t.Errorf("Parent() got %v, waned not found", p)
			}
			if len(e.Children()) != 0 {
				t.Errorf("Children() got %v, wanted none", e.Children())
			}
			if e.AsLiteral() != nil {
				t.Errorf("AsLiteral() got %v, wanted nil", e.AsLiteral())
			}
		})
	}
}

func TestNavigableCallExpr_Member(t *testing.T) {
	checked := mustTypeCheck(t, `'hello'.size()`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.CallKind {
		t.Errorf("Kind() got %v, wanted CallKind", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("FunctionName() got %s, wanted size", call.FunctionName())
	}
	if call.Target() == nil {
		t.Fatalf("Target() got nil, wanted non-nil")
	}
	if len(call.Args()) != 0 {
		t.Errorf("Args() got %v, wanted 0", call.Args())
	}
	target := call.Target()
	if target.Kind() != ast.LiteralKind {
		t.Errorf("Kind() got %v, wanted literal", target.Kind())
	}
	if target.AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("AsLiteral() got %v, wanted 'hello'", target.AsLiteral())
	}
	if call.ReturnType() != expr.Type() {
		t.Errorf("ReturnType() got %v, wanted %v", call.ReturnType(), expr.Type())
	}
	if call.ReturnType() != types.IntType {
		t.Errorf("ReturnType() got %v, wanted int", call.ReturnType())
	}
	if p, found := target.Parent(); !found || p != expr {
		t.Errorf("Parent() got %v, wanted %v", p, expr)
	}
	sizeFn := ast.MatchDescendants(expr, ast.FunctionMatcher("size"))
	if len(sizeFn) != 1 {
		t.Errorf("ast.MatchDescendants() size function returned %v, wanted 1", sizeFn)
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 1 {
		t.Fatalf("ast.MatchDescendants() constant values returned %v, wanted 1 value", constantValues)
	}
	if constantValues[0].AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("constantValues[0] got %v, wanted 'hello'", constantValues[0])
	}
}

func TestNavigableCallExpr_Global(t *testing.T) {
	checked := mustTypeCheck(t, `size('hello')`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.CallKind {
		t.Errorf("Kind() got %v, wanted CallKind", expr.Kind())
	}
	call := expr.AsCall()
	if call.FunctionName() != "size" {
		t.Errorf("FunctionName() got %s, wanted size", call.FunctionName())
	}
	if call.Target() != nil {
		t.Fatalf("Target() got non-nil, wanted nil")
	}
	if len(call.Args()) != 1 {
		t.Errorf("Args() got %v, wanted 1", call.Args())
	}
	arg := call.Args()[0]
	if arg.Kind() != ast.LiteralKind {
		t.Errorf("Kind() got %v, wanted literal", arg.Kind())
	}
	if arg.AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("AsLiteral() got %v, wanted 'hello'", arg.AsLiteral())
	}
	if call.ReturnType() != expr.Type() {
		t.Errorf("ReturnType() got %v, wanted %v", call.ReturnType(), expr.Type())
	}
	if call.ReturnType() != types.IntType {
		t.Errorf("ReturnType() got %v, wanted int", call.ReturnType())
	}
	if p, found := arg.Parent(); !found || p != expr {
		t.Errorf("Parent() got %v, wanted %v", p, expr)
	}
	sizeFn := ast.MatchDescendants(expr, ast.FunctionMatcher("size"))
	if len(sizeFn) != 1 {
		t.Errorf("ast.MatchDescendants() size function returned %v, wanted 1", sizeFn)
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 1 {
		t.Fatalf("ast.MatchDescendants() constant values returned %v, wanted 1 value", constantValues)
	}
	if constantValues[0].AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("constantValues[0] got %v, wanted 'hello'", constantValues[0])
	}
}

func TestNavigableListExpr(t *testing.T) {
	checked := mustTypeCheck(t, `[[1], [2]]`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.ListKind {
		t.Errorf("Kind() got %v, wanted ListKind", expr.Kind())
	}
	list := expr.AsList()
	if list.Size() != 2 {
		t.Errorf("Size() got %d, wanted 2", list.Size())
	}
	if len(list.OptionalIndices()) != 0 {
		t.Errorf("OptionalIndicies() returned %v, wanted none", list.OptionalIndices())
	}
	if len(list.Elements()) != 2 {
		t.Errorf("Elements() returned %v, wanted 2", list.Elements())
	}
	constantValues := ast.MatchDescendants(expr, ast.ConstantValueMatcher())
	if len(constantValues) != 5 {
		t.Errorf("ast.MatchDescendants() constant values returned %v, wanted 5", constantValues)
	}
	constantLists := ast.MatchSubset(constantValues, ast.KindMatcher(ast.ListKind))
	if len(constantLists) != 3 {
		t.Errorf("ast.MatchSubset() constant lists returned %v, wanted 3", constantLists)
	}
	literals := ast.MatchSubset(constantValues, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 2 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 2", literals)
	}
}

func TestNavigableMapExpr(t *testing.T) {
	checked := mustTypeCheck(t, `{'hello': 1}`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.MapKind {
		t.Errorf("Kind() got %v, wanted MapKind", expr.Kind())
	}
	m := expr.AsMap()
	if m.Size() != 1 {
		t.Errorf("Size() got %d, wanted 1", m.Size())
	}
	if len(m.Entries()) != 1 {
		t.Errorf("Entries() returned %v, wanted 1", m.Entries())
	}
	entry := m.Entries()[0]
	if entry.IsOptional() {
		t.Error("IsOptional() returned true, wanted false")
	}
	if entry.Key().AsLiteral().Equal(types.String("hello")) != types.True {
		t.Errorf("Key() returned %v, wanted 'hello'", entry.Key().AsLiteral())
	}
	if entry.Value().AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value() returned %v, wanted 1", entry.Value().AsLiteral())
	}
	descendants := ast.MatchDescendants(expr, ast.AllMatcher())
	if len(descendants) != 3 {
		t.Errorf("ast.MatchDescendants() returned %v, wanted 3", descendants)
	}
	literals := ast.MatchSubset(descendants, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 2 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 2", literals)
	}
}

func TestNavigableStructExpr(t *testing.T) {
	checked := mustTypeCheck(t, `google.expr.proto3.test.TestAllTypes{single_int32: 1}`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.StructKind {
		t.Errorf("Kind() got %v, wanted StructKind", expr.Kind())
	}
	s := expr.AsStruct()
	if s.TypeName() != "google.expr.proto3.test.TestAllTypes" {
		t.Errorf("TypeName() got %s, wanted TestAllTypes", s.TypeName())
	}
	if len(s.Fields()) != 1 {
		t.Errorf("Fields() returned %v, wanted 1", s.Fields())
	}
	field := s.Fields()[0]
	if field.IsOptional() {
		t.Error("IsOptional() returned true, wanted false")
	}
	if field.FieldName() != "single_int32" {
		t.Errorf("FieldName() returned %s, wanted 'single_int32'", field.FieldName())
	}
	if field.Value().AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value() returned %v, wanted 1", field.Value().AsLiteral())
	}
	descendants := ast.MatchDescendants(expr, ast.AllMatcher())
	if len(descendants) != 2 {
		t.Errorf("ast.MatchDescendants() returned %v, wanted 2", descendants)
	}
	literals := ast.MatchSubset(descendants, ast.KindMatcher(ast.LiteralKind))
	if len(literals) != 1 {
		t.Errorf("ast.MatchSubset() literals returned %v, wanted 1", literals)
	}
	if literals[0].AsLiteral().Equal(types.Int(1)) != types.True {
		t.Errorf("Value().AsLiteral() got %v, wanted 1", literals[0].AsLiteral())
	}
}

func TestNavigableComprehensionExpr(t *testing.T) {
	checked := mustTypeCheck(t, `[true].exists(i, i)`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.ComprehensionKind {
		t.Errorf("Kind() got %v, wanted ComprehensionKind", expr.Kind())
	}
	comp := expr.AsComprehension()
	iterRange := comp.IterRange()
	if len(ast.MatchSubset([]ast.NavigableExpr{iterRange}, ast.ConstantValueMatcher())) != 1 {
		t.Errorf("IterRange() returned a non-constant list")
	}
	if comp.IterVar() != "i" {
		t.Errorf("IterVar() got %s, wanted 'i'", comp.IterVar())
	}
	if comp.AccuVar() != "__result__" {
		t.Errorf("AccuVar() got %s, wanted '__result__'", comp.AccuVar())
	}
	if comp.AccuInit().AsLiteral() != types.False {
		t.Errorf("AccuInit() returned %v, wanted false", comp.AccuInit().AsLiteral())
	}
	if comp.Result().Kind() != ast.IdentKind {
		t.Errorf("Result() returned %v, wanted ident", comp.Result())
	}
	if comp.LoopCondition().Kind() != ast.CallKind {
		t.Errorf("LoopCondition() returned %v, wanted call", comp.LoopCondition())
	}
	if comp.LoopStep().Kind() != ast.CallKind {
		t.Errorf("LoopStep() returned %v, wanted call", comp.LoopStep())
	}
	if comp.Result().AsIdent() != "__result__" {
		t.Errorf("AsIdent() returned %v, wanted __result__", comp.Result().AsIdent())
	}
}

func TestNavigableSelectExpr(t *testing.T) {
	checked := mustTypeCheck(t, `msg.single_int32`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.SelectKind {
		t.Errorf("Kind() got %v, wanted SelectKind", expr.Kind())
	}
	sel := expr.AsSelect()
	if sel.FieldName() != "single_int32" {
		t.Errorf("FieldName() got %s, wanted single_int32", sel.FieldName())
	}
	if sel.Operand().Kind() != ast.IdentKind {
		t.Fatalf("Operand() kind got %v, wanted ident", sel.Operand().Kind())
	}
	if sel.Operand().AsIdent() != "msg" {
		t.Errorf("Operand() got %v, wanted ident 'msg'", sel.Operand())
	}
	if sel.IsTestOnly() {
		t.Error("IsTestOnly() got true, wanted false")
	}
}

func TestNavigableSelectExpr_TestOnly(t *testing.T) {
	checked := mustTypeCheck(t, `has(msg.single_int32)`)
	expr := ast.NavigateCheckedAST(checked)
	if expr.Kind() != ast.SelectKind {
		t.Errorf("Kind() got %v, wanted SelectKind", expr.Kind())
	}
	sel := expr.AsSelect()
	if sel.FieldName() != "single_int32" {
		t.Errorf("FieldName() got %s, wanted single_int32", sel.FieldName())
	}
	if sel.Operand().Kind() != ast.IdentKind {
		t.Fatalf("Operand() kind got %v, wanted ident", sel.Operand().Kind())
	}
	if sel.Operand().AsIdent() != "msg" {
		t.Errorf("Operand() got %v, wanted ident 'msg'", sel.Operand())
	}
	if !sel.IsTestOnly() {
		t.Error("IsTestOnly() got false, wanted true")
	}
}

func mustTypeCheck(t testing.TB, expr string) *ast.CheckedAST {
	t.Helper()
	p, err := parser.NewParser(parser.Macros(parser.AllMacros...), parser.EnableOptionalSyntax(true))
	if err != nil {
		t.Fatalf("parser.NewParser() failed: %v", err)
	}
	exprSrc := common.NewTextSource(expr)
	parsed, iss := p.Parse(exprSrc)
	if len(iss.GetErrors()) != 0 {
		t.Fatalf("Parse(%s) failed: %s", expr, iss.ToDisplayString())
	}
	reg := newTestRegistry(t, &proto3pb.TestAllTypes{})
	env := newTestEnv(t, containers.DefaultContainer, reg)
	checked, iss := checker.Check(parsed, exprSrc, env)
	if len(iss.GetErrors()) != 0 {
		t.Fatalf("Check(%s) failed: %s", expr, iss.ToDisplayString())
	}
	return checked
}

func newTestRegistry(t testing.TB, msgs ...proto.Message) *types.Registry {
	t.Helper()
	reg, err := types.NewRegistry(msgs...)
	if err != nil {
		t.Fatalf("types.NewRegistry(%v) failed: %v", msgs, err)
	}
	return reg
}

func newTestEnv(t testing.TB, cont *containers.Container, reg *types.Registry) *checker.Env {
	t.Helper()
	env, err := checker.NewEnv(cont, reg, checker.CrossTypeNumericComparisons(true))
	if err != nil {
		t.Fatalf("checker.NewEnv(%v, %v) failed: %v", cont, reg, err)
	}
	err = env.AddIdents(stdlib.Types()...)
	if err != nil {
		t.Fatalf("env.Add(stdlib.Types()...) failed: %v", err)
	}
	err = env.AddFunctions(stdlib.Functions()...)
	if err != nil {
		t.Fatalf("env.Add(stdlib.Functions()...) failed: %v", err)
	}
	env.AddIdents(decls.NewVariable("msg", types.NewObjectType("google.expr.proto3.test.TestAllTypes")))
	return env
}
