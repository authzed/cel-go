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
	"testing"

	"github.com/google/cel-go/cel"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestUnparseType(t *testing.T) {
	var testCases = []struct {
		exprType *exprpb.Type
		wantFmt  string
	}{
		{
			exprType: &exprpb.Type{},
			wantFmt:  "*unknown type kind*",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Dyn{}},
			wantFmt:  "dyn",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Null{}},
			wantFmt:  "null_type",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BOOL}},
			wantFmt:  "bool",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}},
			wantFmt:  "int",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_UINT64}},
			wantFmt:  "uint",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
			wantFmt:  "double",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
			wantFmt:  "string",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BYTES}},
			wantFmt:  "bytes",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_PRIMITIVE_TYPE_UNSPECIFIED}},
			wantFmt:  "*unknown type kind*",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_DURATION}},
			wantFmt:  "google.protobuf.Duration",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_TIMESTAMP}},
			wantFmt:  "google.protobuf.Timestamp",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_ANY}},
			wantFmt:  "google.protobuf.Any",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_WELL_KNOWN_TYPE_UNSPECIFIED}},
			wantFmt:  "*unknown type kind*",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_MapType_{MapType: &exprpb.Type_MapType{
				KeyType:   &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
				ValueType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_TIMESTAMP}},
			}}},
			wantFmt: "map<string, google.protobuf.Timestamp>",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_ListType_{
				ListType: &exprpb.Type_ListType{ElemType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}}}}},
			wantFmt: "list<double>",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Type{
				Type: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}}}},
			wantFmt: "type<double>",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{
				Wrapper: exprpb.Type_UINT64}},
			wantFmt: "google.protobuf.UInt64Value",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Error{}},
			wantFmt:  "*error*",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_MessageType{
				MessageType: "com.example.Message",
			}},
			wantFmt: "com.example.Message",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_TypeParam{
				TypeParam: "T",
			}},
			wantFmt: "~T",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_Function{
				Function: &exprpb.Type_FunctionType{
					ResultType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
					ArgTypes: []*exprpb.Type{
						{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
						{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
					},
				},
			}},
			wantFmt: "*unknown type kind*",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_AbstractType_{
				AbstractType: &exprpb.Type_AbstractType{
					Name: "MyAbstractParamType",
					ParameterTypes: []*exprpb.Type{
						{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
						{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
					},
				},
			}},
			wantFmt: "MyAbstractParamType<double, string>",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_AbstractType_{
				AbstractType: &exprpb.Type_AbstractType{
					Name: "MyAbstractType",
				},
			}},
			wantFmt: "MyAbstractType",
		},
		{
			exprType: &exprpb.Type{TypeKind: &exprpb.Type_MapType_{MapType: &exprpb.Type_MapType{
				KeyType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
				ValueType: &exprpb.Type{TypeKind: &exprpb.Type_ListType_{
					ListType: &exprpb.Type_ListType{ElemType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}}}}},
			}}},
			wantFmt: "map<string, list<double>>",
		},
	}

	for _, tc := range testCases {
		fmt := UnparseExprType(tc.exprType)
		if fmt != tc.wantFmt {
			t.Errorf("UnparseExprType(%v) = %q, wanted %q", tc.exprType, fmt, tc.wantFmt)
		}
	}

}

func TestParseType(t *testing.T) {
	var testCases = []struct {
		fmt          string
		wantExprType *exprpb.Type
	}{
		{
			fmt:          "int",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}},
		},
		{
			fmt:          "uint",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_UINT64}},
		},
		{
			fmt:          "double",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_DOUBLE}},
		},
		{
			fmt:          "string",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
		},
		{
			fmt:          "bytes",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BYTES}},
		},
		{
			fmt:          "bool",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_BOOL}},
		},
		{
			fmt:          "int_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_INT64}},
		},
		{
			fmt:          "uint_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_UINT64}},
		},
		{
			fmt:          "double_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_DOUBLE}},
		},
		{
			fmt:          "string_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_STRING}},
		},
		{
			fmt:          "bytes_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_BYTES}},
		},
		{
			fmt:          "bool_wrapper",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_BOOL}},
		},
		{
			fmt:          "dyn",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Dyn{}},
		},
		{
			fmt:          "null",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Null{}},
		},
		{
			fmt:          "google.protobuf.Any",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_ANY}},
		},
		{
			fmt:          "google.protobuf.Timestamp",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_TIMESTAMP}},
		},
		{
			fmt:          "google.protobuf.Duration",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_DURATION}},
		},
		{
			fmt:          "any",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_ANY}},
		},
		{
			fmt:          "timestamp",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_TIMESTAMP}},
		},
		{
			fmt:          "duration",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_WellKnown{WellKnown: exprpb.Type_DURATION}},
		},
		{
			fmt: "map<string, int>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_MapType_{
				MapType: &exprpb.Type_MapType{
					KeyType:   &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
					ValueType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}},
				}}},
		},
		{
			fmt: "map<string, map<string, int>>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_MapType_{
				MapType: &exprpb.Type_MapType{
					KeyType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
					ValueType: &exprpb.Type{TypeKind: &exprpb.Type_MapType_{
						MapType: &exprpb.Type_MapType{
							KeyType:   &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
							ValueType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}},
						}}},
				}}},
		},
		{
			fmt: "list<int>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_ListType_{
				ListType: &exprpb.Type_ListType{ElemType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}}}}},
		},
		{
			fmt: "list<list<int>>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_ListType_{
				ListType: &exprpb.Type_ListType{ElemType: &exprpb.Type{TypeKind: &exprpb.Type_ListType_{
					ListType: &exprpb.Type_ListType{ElemType: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}}}}}}}},
		},
		{
			fmt: "type<int>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Type{
				Type: &exprpb.Type{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_INT64}}}},
		},
		{
			fmt: "type<type<int_wrapper>>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_Type{
				Type: &exprpb.Type{TypeKind: &exprpb.Type_Type{
					Type: &exprpb.Type{TypeKind: &exprpb.Type_Wrapper{Wrapper: exprpb.Type_INT64}}}}}},
		},
		{
			fmt: "optional_type",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_AbstractType_{
				AbstractType: &exprpb.Type_AbstractType{
					Name: "optional_type",
					ParameterTypes: []*exprpb.Type{
						{TypeKind: &exprpb.Type_Dyn{}},
					}}}},
		},
		{
			fmt: "optional_type<string>",
			wantExprType: &exprpb.Type{TypeKind: &exprpb.Type_AbstractType_{
				AbstractType: &exprpb.Type_AbstractType{
					Name: "optional_type",
					ParameterTypes: []*exprpb.Type{
						{TypeKind: &exprpb.Type_Primitive{Primitive: exprpb.Type_STRING}},
					}}}},
		},
	}
	env, _ := cel.NewEnv()
	for _, tc := range testCases {
		t.Run(tc.fmt, func(t *testing.T) {
			confType, err := ParseType(tc.fmt)
			if err != nil {
				t.Fatalf("ParseType(%s) failed: %v", tc.fmt, err)
			}
			celType, err := confType.AsCELType(env.CELTypeProvider())
			if err != nil {
				t.Fatalf("could not resolve parsed type %s", confType.SpecifierFormat())
			}
			gotFmt := UnparseType(celType)
			wantFmt := UnparseExprType(tc.wantExprType)
			if gotFmt != wantFmt {
				t.Errorf("ParseType(%s) got %s, wanted %s", tc.fmt, gotFmt, wantFmt)
			}
		})
	}
}

func TestParseTypeErrors(t *testing.T) {
	var testCases = []struct {
		fmt string
	}{{
		fmt: "list<>",
	},
		{
			fmt: "list<int",
		},
		{
			fmt: "list",
		},
		{
			fmt: "list<int, int>",
		},
		{
			fmt: "in",
		},
		{
			fmt: "x?",
		},
		{
			fmt: "map<int>",
		},
		{
			fmt: "map",
		},
		{
			fmt: "map<string, >",
		},
		{
			fmt: "map<string, int",
		},
	}

	for _, tc := range testCases {
		exprType, err := ParseType(tc.fmt)
		if err == nil {
			t.Errorf("ParseType(%s) got %s, wanted error", tc.fmt, exprType)
		}
	}

}
