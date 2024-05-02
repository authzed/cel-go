module github.com/authzed/cel-go/codelab

go 1.18

require (
	github.com/golang/glog v1.0.0
	github.com/authzed/cel-go v0.13.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230803162519-f966b187b2e5
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230803162519-f966b187b2e5 // indirect
)

replace github.com/authzed/cel-go => ../.
