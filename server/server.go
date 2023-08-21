// Copyright 2018 Google LLC
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

// Package server defines the gRPC conformance test server for CEL Go.
package server

import (
	"context"
	"fmt"

	"github.com/authzed/cel-go/cel"
	"github.com/authzed/cel-go/common/types"
	"github.com/authzed/cel-go/common/types/ref"

	test2pb "github.com/google/cel-spec/proto/test/v1/proto2/test_all_types"
	test3pb "github.com/google/cel-spec/proto/test/v1/proto3/test_all_types"
	confpb "google.golang.org/genproto/googleapis/api/expr/conformance/v1alpha1"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	codepb "google.golang.org/genproto/googleapis/rpc/code"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

// ConformanceServer contains the server state.
type ConformanceServer struct{}

// Parse implements ConformanceService.Parse.
func (s *ConformanceServer) Parse(ctx context.Context, in *confpb.ParseRequest) (*confpb.ParseResponse, error) {
	if in.CelSource == "" {
		return nil, invalidArgument("No source code.")
	}
	// NOTE: syntax_version isn't currently used
	var parseOptions []cel.EnvOption
	if in.DisableMacros {
		parseOptions = append(parseOptions, cel.ClearMacros())
	}
	env, _ := cel.NewEnv(parseOptions...)
	past, iss := env.Parse(in.CelSource)
	resp := confpb.ParseResponse{}
	if iss == nil || iss.Err() == nil {
		// Success
		resp.ParsedExpr, _ = cel.AstToParsedExpr(past)
	} else {
		// Failure
		appendErrors(iss.Errors(), &resp.Issues)
	}
	return &resp, nil
}

// Check implements ConformanceService.Check.
func (s *ConformanceServer) Check(ctx context.Context, in *confpb.CheckRequest) (*confpb.CheckResponse, error) {
	if in.ParsedExpr == nil {
		return nil, invalidArgument("No parsed expression.")
	}
	if in.ParsedExpr.SourceInfo == nil {
		return nil, invalidArgument("No source info.")
	}
	// Build the environment.
	var checkOptions []cel.EnvOption = []cel.EnvOption{cel.StdLib()}
	if in.NoStdEnv {
		checkOptions = []cel.EnvOption{}
	}
	checkOptions = append(checkOptions, cel.Container(in.Container))
	checkOptions = append(checkOptions, cel.Declarations(in.TypeEnv...))
	checkOptions = append(checkOptions, cel.Types(&test2pb.TestAllTypes{}))
	checkOptions = append(checkOptions, cel.Types(&test3pb.TestAllTypes{}))
	env, _ := cel.NewCustomEnv(checkOptions...)

	// Check the expression.
	cast, iss := env.Check(cel.ParsedExprToAst(in.ParsedExpr))
	resp := confpb.CheckResponse{}
	if iss == nil || iss.Err() == nil {
		// Success
		resp.CheckedExpr, _ = cel.AstToCheckedExpr(cast)
	} else {
		// Failure
		appendErrors(iss.Errors(), &resp.Issues)
	}
	return &resp, nil
}

// Eval implements ConformanceService.Eval.
func (s *ConformanceServer) Eval(ctx context.Context, in *confpb.EvalRequest) (*confpb.EvalResponse, error) {
	env, _ := evalEnv.Extend(cel.Container(in.Container))
	var prg cel.Program
	var err error
	switch in.GetExprKind().(type) {
	case *confpb.EvalRequest_ParsedExpr:
		ast := cel.ParsedExprToAst(in.GetParsedExpr())
		prg, err = env.Program(ast)
		if err != nil {
			return nil, err
		}
	case *confpb.EvalRequest_CheckedExpr:
		ast := cel.CheckedExprToAst(in.GetCheckedExpr())
		prg, err = env.Program(ast)
		if err != nil {
			return nil, err
		}
	default:
		return nil, invalidArgument("No expression.")
	}
	args := make(map[string]any)
	for name, exprValue := range in.Bindings {
		refVal, err := ExprValueToRefValue(env.TypeAdapter(), exprValue)
		if err != nil {
			return nil, fmt.Errorf("can't convert binding %s: %s", name, err)
		}
		args[name] = refVal
	}
	// NOTE: the EvalState is currently discarded
	res, _, err := prg.Eval(args)
	resultExprVal, err := RefValueToExprValue(res, err)
	if err != nil {
		return nil, fmt.Errorf("con't convert result: %s", err)
	}
	return &confpb.EvalResponse{Result: resultExprVal}, nil
}

// appendErrors converts the errors from errs to Status messages
// and appends them to the list of issues.
func appendErrors(errs []*cel.Error, issues *[]*statuspb.Status) {
	for _, e := range errs {
		status := ErrToStatus(e, confpb.IssueDetails_ERROR)
		*issues = append(*issues, status)
	}
}

// ErrToStatus converts an Error to a Status message with the given severity.
func ErrToStatus(e *cel.Error, severity confpb.IssueDetails_Severity) *statuspb.Status {
	detail := &confpb.IssueDetails{
		Severity: severity,
		Position: &confpb.SourcePosition{
			Line:   int32(e.Location.Line()),
			Column: int32(e.Location.Column()),
		},
	}
	s := errToStatus(invalidArgument(e.Message))
	packed, err := anypb.New(detail)
	if err != nil {
		return s
	}
	s.Details = append(s.Details, packed)
	return s
}

// TODO(jimlarson): The following conversion code should be moved to
// common/types/provider.go and consolidated/refactored as appropriate.
// In particular, make judicious use of types.NativeToValue().

// RefValueToExprValue converts between ref.Val and exprpb.ExprValue.
func RefValueToExprValue(res ref.Val, err error) (*exprpb.ExprValue, error) {
	if err != nil {
		s := errToStatus(err)
		return &exprpb.ExprValue{
			Kind: &exprpb.ExprValue_Error{
				Error: &exprpb.ErrorSet{
					Errors: []*statuspb.Status{s},
				},
			},
		}, nil
	}
	if types.IsUnknown(res) {
		return &exprpb.ExprValue{
			Kind: &exprpb.ExprValue_Unknown{
				Unknown: &exprpb.UnknownSet{
					Exprs: res.Value().([]int64),
				},
			}}, nil
	}
	v, err := cel.RefValueToValue(res)
	if err != nil {
		return nil, err
	}
	return &exprpb.ExprValue{
		Kind: &exprpb.ExprValue_Value{Value: v}}, nil
}

// ExprValueToRefValue converts between exprpb.ExprValue and ref.Val.
func ExprValueToRefValue(adapter types.Adapter, ev *exprpb.ExprValue) (ref.Val, error) {
	switch ev.Kind.(type) {
	case *exprpb.ExprValue_Value:
		return cel.ValueToRefValue(adapter, ev.GetValue())
	case *exprpb.ExprValue_Error:
		// An error ExprValue is a repeated set of statuspb.Status
		// messages, with no convention for the status details.
		// To convert this to a types.Err, we need to convert
		// these Status messages to a single string, and be
		// able to decompose that string on output so we can
		// round-trip arbitrary ExprValue messages.
		// TODO(jimlarson) make a convention for this.
		return types.NewErr("XXX add details later"), nil
	case *exprpb.ExprValue_Unknown:
		var unk *types.Unknown
		for _, id := range ev.GetUnknown().GetExprs() {
			if unk == nil {
				unk = types.NewUnknown(id, nil)
			}
			unk = types.MergeUnknowns(types.NewUnknown(id, nil), unk)
		}
		return unk, nil
	}
	return nil, invalidArgument("unknown ExprValue kind")
}

func errToStatus(err error) *statuspb.Status {
	re, ok := err.(invalidArgErr)
	if ok {
		return &statuspb.Status{
			Code:    int32(codepb.Code_INVALID_ARGUMENT),
			Message: re.msg,
		}
	}
	return &statuspb.Status{
		Code:    int32(codepb.Code_UNKNOWN),
		Message: err.Error(),
	}
}

func invalidArgument(msg string) error {
	return invalidArgErr{msg: msg}
}

type invalidArgErr struct {
	msg string
}

func (e invalidArgErr) Error() string {
	return e.String()
}

func (e invalidArgErr) String() string {
	return fmt.Sprintf("rpc error: code = InvalidArgument desc = %s", e.msg)
}

func (e invalidArgErr) Is(other error) bool {
	otherErr, ok := other.(invalidArgErr)
	return ok && e.msg == otherErr.msg
}

var evalEnv *cel.Env

func init() {
	evalEnv, _ = cel.NewEnv(
		cel.Types(&test2pb.TestAllTypes{}, &test3pb.TestAllTypes{}),
		cel.EagerlyValidateDeclarations(true))
}
