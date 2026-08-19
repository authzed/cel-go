module github.com/google/cel-go/conformance

go 1.23.0

require (
	cel.dev/expr v0.25.1
	github.com/bazelbuild/rules_go v0.49.0
	github.com/google/cel-go v0.26.1
	github.com/google/go-cmp v0.7.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
)

replace github.com/google/cel-go => ./..
