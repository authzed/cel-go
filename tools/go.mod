module github.com/google/cel-go/tools

go 1.23.0

require (
	cel.dev/expr v0.25.1
	github.com/google/cel-go v0.22.0
	github.com/google/cel-go/policy v0.0.0-20250311174852-f5ea07b389a1
	github.com/google/go-cmp v0.7.0
	go.yaml.in/yaml/v3 v3.0.4
	google.golang.org/genproto/googleapis/api v0.0.0-20250311190419-81fb87f6b8bf
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250311190419-81fb87f6b8bf // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/google/cel-go => ../.
