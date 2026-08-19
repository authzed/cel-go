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
	"github.com/google/cel-go/common"
)

// ByteSource converts a byte sequence and location description to a model.Source.
func ByteSource(contents []byte, location string) *Source {
	return StringSource(string(contents), location)
}

// StringSource converts a string and location description to a model.Source.
func StringSource(contents, location string) *Source {
	return &Source{
		Source: common.NewStringSource(contents, location),
	}
}

// Source represents the contents of a single source file.
type Source struct {
	common.Source
}

// Relative produces a RelativeSource object for the content provided at the absolute location
// within the parent Source as indicated by the line and column.
func (src *Source) Relative(content string, line, col int) *RelativeSource {
	return &RelativeSource{
		Source:   src.Source,
		localSrc: common.NewStringSource(content, src.Description()),
		absLoc:   common.NewLocation(line, col),
	}
}

// RelativeSource represents an embedded source element within a larger source.
type RelativeSource struct {
	common.Source
	localSrc common.Source
	absLoc   common.Location
}

// Content returns the embedded source snippet.
func (rel *RelativeSource) Content() string {
	return rel.localSrc.Content()
}

// OffsetLocation returns the absolute location given the relative offset, if found.
func (rel *RelativeSource) OffsetLocation(offset int32) (common.Location, bool) {
	absOffset, found := rel.Source.LocationOffset(rel.absLoc)
	if !found {
		return common.NoLocation, false
	}
	return rel.Source.OffsetLocation(absOffset + offset)
}
