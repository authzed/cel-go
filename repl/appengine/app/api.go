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

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/authzed/cel-go/repl"
)

type service struct {
	commandCountLimit int
}

type evaluateRequest struct {
	Commands []string
}

type commandResponse struct {
	ReplOutput string `json:"replOutput"`
	Issue      string `json:"issue"`
	Evaluated  bool   `json:"evaluated"`
}

type evaluateResponse struct {
	Responses []commandResponse `json:"responses"`
	EvalTime  time.Duration     `json:"evalTime"`
}

func unmarshalEvaluationRequest(data []byte) (*evaluateRequest, error) {
	r := evaluateRequest{}
	err := json.Unmarshal(data, &r)
	return &r, err
}

func marshalEvaluationResponse(r *evaluateResponse) ([]byte, error) {
	v, err := json.Marshal(r)
	return v, err
}

func (s *service) evaluate(req *evaluateRequest) (*evaluateResponse, error) {
	evaluator, err := repl.NewEvaluator()
	if err != nil {
		return nil, errors.New("error initilializing evaluator")
	}

	if s.commandCountLimit > 0 && len(req.Commands) > s.commandCountLimit {
		return nil, fmt.Errorf("number of commands (%d) exceeded limit (%d)", len(req.Commands), s.commandCountLimit)
	}
	start := time.Now()
	resp := evaluateResponse{}
	for _, replCmd := range req.Commands {
		cmd, err := repl.Parse(replCmd)
		if err != nil {
			resp.Responses = append(resp.Responses, commandResponse{
				ReplOutput: "",
				Issue:      err.Error(),
			})
			continue
		}
		val, _, err := evaluator.Process(cmd)
		iss := ""
		if err != nil {
			iss = err.Error()
		}
		resp.Responses = append(resp.Responses,
			commandResponse{
				ReplOutput: val,
				Issue:      iss,
				Evaluated:  true,
			})

	}
	elapsed := time.Now().Sub(start)
	resp.EvalTime = elapsed
	return &resp, nil
}

func (s *service) evaluateJSON(data []byte) ([]byte, error) {
	req, err := unmarshalEvaluationRequest(data)
	if err != nil {
		return nil, err
	}

	resp, err := s.evaluate(req)
	if err != nil {
		return nil, err
	}

	return marshalEvaluationResponse(resp)
}

func writeErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(err.Error()))
}

// NewJSONHandler provides an http.HandlerFunc that implements the JSON API
// for the CEL REPL.
//
// The service is stateless -- every request creates a new repl instance and
// applies the list of commands in order.
func NewJSONHandler() http.HandlerFunc {
	s := &service{
		commandCountLimit: 50,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeErr(w, err)
			return
		}
		resp, err := s.evaluateJSON(data)
		if err != nil {
			writeErr(w, err)

		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}

}
