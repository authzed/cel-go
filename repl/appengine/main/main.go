// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// package main provides an entry point for the REPL web server.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/authzed/cel-go/repl/appengine/app"
)

var port string
var serveStatic string

func init() {
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	flag.StringVar(&serveStatic, "serve_static", "", "serve static files from binary using argument as root")
	flag.Parse()
}

type context struct{}

func main() {
	mux := app.NewAppMux(serveStatic)
	log.Printf("Listening on port %s", port)
	http.Handle("/", mux)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
