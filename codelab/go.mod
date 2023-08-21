module github.com/authzed/cel-go/codelab

go 1.18

require (
	github.com/golang/glog v1.0.0
	github.com/authzed/cel-go v0.13.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230525234035-dd9d682886f9 // indirect
)

replace github.com/authzed/cel-go => ../.
