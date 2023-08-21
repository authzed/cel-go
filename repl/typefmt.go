// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repl

import (
	"fmt"
	"strings"

	"github.com/authzed/cel-go/repl/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func formatTypeArgs(ts []*exprpb.Type) string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = UnparseType(t)
	}
	return fmt.Sprintf("(%s)", strings.Join(s, ", "))
}

func formatFn(t *exprpb.Type_FunctionType) string {
	return fmt.Sprintf("%s -> %s", formatTypeArgs(t.GetArgTypes()), UnparseType(t.GetResultType()))
}

func formatPrimitive(t exprpb.Type_PrimitiveType) string {
	switch t {
	case exprpb.Type_BOOL:
		return "bool"
	case exprpb.Type_STRING:
		return "string"
	case exprpb.Type_BYTES:
		return "bytes"
	case exprpb.Type_INT64:
		return "int"
	case exprpb.Type_UINT64:
		return "uint"
	case exprpb.Type_DOUBLE:
		return "double"
	}
	return "<unknown primitive>"
}

func formatWellKnown(t exprpb.Type_WellKnownType) string {
	switch t {
	case exprpb.Type_ANY:
		return "any"
	case exprpb.Type_DURATION:
		return "google.protobuf.Duration"
	case exprpb.Type_TIMESTAMP:
		return "google.protobuf.Timestamp"
	}
	return "<unknown well-known type>"
}

// UnparseType pretty-prints a type for the REPL.
//
// TODO(issue/538): This is slightly different from core CEL's built-in formatter. Should
// converge if possible.
func UnparseType(t *exprpb.Type) string {
	if t == nil {
		return "<unknown type>"
	}
	switch t.TypeKind.(type) {
	case *exprpb.Type_Dyn:
		return "dyn"
	case *exprpb.Type_Null:
		return "null"
	case *exprpb.Type_Primitive:
		return formatPrimitive(t.GetPrimitive())
	case *exprpb.Type_WellKnown:
		return formatWellKnown(t.GetWellKnown())
	case *exprpb.Type_ListType_:
		return fmt.Sprintf("list(%s)", UnparseType(t.GetListType().GetElemType()))
	case *exprpb.Type_MapType_:
		return fmt.Sprintf("map(%s, %s)", UnparseType(t.GetMapType().GetKeyType()), UnparseType(t.GetMapType().GetValueType()))
	case *exprpb.Type_Type:
		return fmt.Sprintf("type(%s)", UnparseType(t.GetType()))
	case *exprpb.Type_Wrapper:
		return fmt.Sprintf("wrapper(%s)", formatPrimitive(t.GetWrapper()))
	case *exprpb.Type_Error:
		return "*error*"
	case *exprpb.Type_MessageType:
		return t.GetMessageType()
	case *exprpb.Type_TypeParam:
		return t.GetTypeParam()
	case *exprpb.Type_Function:
		return formatFn(t.GetFunction())
	case *exprpb.Type_AbstractType_:
		if len(t.GetAbstractType().GetParameterTypes()) > 0 {
			return t.GetAbstractType().GetName() + formatTypeArgs(t.GetAbstractType().GetParameterTypes())
		}
		return t.GetAbstractType().GetName()
	}
	return "<unknown type>"
}

type errorListener struct {
	*antlr.DefaultErrorListener
	errs []error
}

func (l *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int, msg string, e antlr.RecognitionException) {
	l.errs = append(l.errs, fmt.Errorf("type parse error: %s", msg))
	l.DefaultErrorListener.SyntaxError(recognizer, offendingSymbol, line, column, msg, e)
}

type typesVisitor struct {
	parser.BaseCommandsVisitor

	errs []error
}

var _ parser.CommandsVisitor = &typesVisitor{}

type typeParams []*exprpb.Type

func (t *typesVisitor) Visit(tree antlr.ParseTree) any {
	switch ctx := tree.(type) {
	case *parser.StartTypeContext:
		return t.VisitStartType(ctx)
	case *parser.TypeContext:
		return t.VisitType(ctx)
	case *parser.TypeIdContext:
		return t.VisitTypeID(ctx)
	case *parser.TypeParamListContext:
		return t.VisitTypeParamList(ctx)
	default:
		t.errs = append(t.errs, fmt.Errorf("unhandled parse node kind"))
		return nil
	}

}

func (t *typesVisitor) VisitStartType(ctx *parser.StartTypeContext) any {
	return t.Visit(ctx.GetT())
}

func (t *typesVisitor) expectUnparameterized(p typeParams, id string) {
	if p != nil {
		t.errs = append(t.errs, fmt.Errorf("unexpected type params for %s", id))
	}
}

func checkWellKnown(name string) *exprpb.Type {
	switch name {
	case "google.protobuf.Timestamp", ".google.protobuf.Timestamp":
		return &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_TIMESTAMP}}
	case "google.protobuf.Duration", ".google.protobuf.Duration":
		return &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_DURATION}}
	case "any":
		return &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_ANY}}
	}
	return nil
}

func (t *typesVisitor) VisitTypeID(ctx *parser.TypeIdContext) any {
	id := ""
	if ctx.GetLeadingDot() != nil {
		id += "."
	}
	tl := ctx.GetId()
	if tl == nil {
		return nil
	}
	id += tl.GetText()
	for _, tok := range ctx.GetQualifiers() {
		id += "." + tok.GetText()
	}
	return id
}

func (t *typesVisitor) VisitTypeParamList(ctx *parser.TypeParamListContext) any {
	var params typeParams
	for _, ty := range ctx.GetTypes() {
		p := t.Visit(ty)
		params = append(params, p.(*exprpb.Type))
	}
	return params
}

func (t *typesVisitor) VisitType(ctx *parser.TypeContext) any {
	emptyType := &exprpb.Type{}

	r := t.Visit(ctx.GetId())
	if r == nil {
		return emptyType
	}

	typeID := r.(string)

	paramsCtx := ctx.GetParams()

	var params typeParams
	if paramsCtx != nil {
		r = t.Visit(paramsCtx)
		if r == nil {
			return emptyType
		}
		params = r.(typeParams)
	}

	switch typeID {
	case "int":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}}
	case "uint":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_UINT64}}
	case "double":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}}
	case "bytes":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BYTES}}
	case "string":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}}
	case "bool":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BOOL}}
	case "dyn":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Dyn{}}
	case "null":
		t.expectUnparameterized(params, typeID)
		return &exprpb.Type{TypeKind: &exprpb.Type_Null{}}
	case "wrapper":
		if params == nil || len(params) != 1 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly one parameter for wrapper"))
			return emptyType
		}
		p := params[0]
		if p.GetPrimitive() == exprpb.Type_PRIMITIVE_TYPE_UNSPECIFIED {
			t.errs = append(t.errs, fmt.Errorf("expected primitive param for wrapper"))
		}
		return &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: p.GetPrimitive()}}
	case "list":
		if params == nil || len(params) != 1 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly one parameter for list"))
			return emptyType
		}
		p := params[0]
		return &exprpb.Type{TypeKind: &exprpb.Type_ListType_{ListType: &exprpb.Type_ListType{ElemType: p}}}
	case "map":
		if params == nil || len(params) != 2 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly two parameters for map"))
			return emptyType
		}
		k, v := params[0], params[1]
		return &exprpb.Type{TypeKind: &exprpb.Type_MapType_{
			MapType: &exprpb.Type_MapType{
				KeyType:   k,
				ValueType: v,
			}}}
	case "type":
		if params == nil || len(params) != 1 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly one parameter for type"))
			return emptyType
		}
		p := params[0]
		return &exprpb.Type{TypeKind: &exprpb.Type_Type{Type: p}}
	default:
		// TODO(issue/538): need a way to distinguish message from abstract type
		t.expectUnparameterized(params, typeID)
		wkt := checkWellKnown(typeID)
		if wkt != nil {
			return wkt
		}
		return &exprpb.Type{TypeKind: &exprpb.Type_MessageType{MessageType: typeID}}
	}
}

// ParseType parses a human readable type string into the protobuf representation.
// TODO(issue/538): add support for abstract types and validating message types.
func ParseType(t string) (*exprpb.Type, error) {
	var errListener errorListener
	visitor := &typesVisitor{}
	is := antlr.NewInputStream(t)
	lexer := parser.NewCommandsLexer(is)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(&errListener)
	p := parser.NewCommandsParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	p.RemoveErrorListeners()
	p.AddErrorListener(&errListener)

	var result *exprpb.Type
	s := visitor.Visit(p.StartType())
	if s != nil {
		result = s.(*exprpb.Type)
	}

	errs := append(errListener.errs, visitor.errs...)
	var err error = nil

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		err = fmt.Errorf("errors parsing type:\n" + strings.Join(msgs, "\n"))
	}

	return result, err
}
