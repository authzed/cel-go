// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/ext"
)

// FromConfig configures a CEL policy environment from a config file.
//
// This option supports all extensions supported by policies, whereas the cel.FromConfig supports
// a set of configuration ConfigOptionFactory values to handle extensions and other config features
// which may be defined outside of the `cel` package.
func FromConfig(config *env.Config) cel.EnvOption {
	return cel.FromConfig(config, ext.ExtensionOptionFactory)
}
