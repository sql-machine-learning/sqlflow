// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package response

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

// CompoundResponses Compounds response
type CompoundResponses struct {
	stepCnt   int
	stepIdx   int
	responses []*pb.Response
}

// New returns CompoundResponses with step index
func New(stepCnt, stepIdx int) *CompoundResponses {
	return &CompoundResponses{
		stepCnt:   stepCnt,
		stepIdx:   stepIdx,
		responses: []*pb.Response{},
	}
}

// AppendMessageWithStepStatus append message with Step Index as prefix
func (r *CompoundResponses) AppendMessageWithStepStatus(message string) error {
	res, e := pb.EncodeMessage(fmt.Sprintf("SQLFlow Step: [%d/%d] %s", r.stepIdx, r.stepCnt, message))
	if e != nil {
		return e
	}
	r.responses = append(r.responses, res)
	return nil
}

// AppendEoe appends eoe response
func (r *CompoundResponses) AppendEoe() {
	eoe := &pb.Response{Response: &pb.Response_Eoe{Eoe: &pb.EndOfExecution{}}}
	r.responses = append(r.responses, eoe)
}

// AppendProtoMessages appends the message with protobuf message format
func (r *CompoundResponses) AppendProtoMessages(messages []string) error {
	// unmarshal pb.Response from proto message with text format
	res, e := unMarshalProtoMessages(messages)
	if e != nil {
		return e
	}
	r.responses = append(r.responses, res...)
	return nil
}

// Messages returns Message type response as string slice
func (r *CompoundResponses) Messages() []string {
	messages := []string{}
	for _, res := range r.responses {
		if res.GetMessage() != nil {
			messages = append(messages, res.GetMessage().Message)
		}
	}
	return messages
}

// Response returns the compounded Response
func (r *CompoundResponses) Response(jobID, stepID, stepPhase string, eof bool) *pb.FetchResponse {
	return NewFetchResponse(NewFetchRequest(jobID, stepID, stepPhase), eof, r.responses)
}

// ResponseWithStepComplete returns the compounded Response at the end of step
func (r *CompoundResponses) ResponseWithStepComplete(jobID, stepID, stepPhase string, eof bool) *pb.FetchResponse {
	r.AppendEoe()
	return r.Response(jobID, stepID, stepPhase, eof)
}

func unMarshalProtoMessages(messages []string) ([]*pb.Response, error) {
	responses := []*pb.Response{}
	for _, msg := range messages {
		msg = strings.TrimSpace(msg)
		if isHTMLCode(msg) {
			r, e := pb.EncodeMessage(msg)
			if e != nil {
				return nil, e
			}
			responses = append(responses, r)
		}
		response := &pb.Response{}
		if e := proto.UnmarshalText(msg, response); e != nil {
			// skip this line if it's not protobuf message
			continue
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func isHTMLCode(code string) bool {
	//TODO(yancey1989): support more lines HTML code e.g.
	//<div>
	//  ...
	//</div>
	re := regexp.MustCompile(`<div.*?>.*</div>`)
	return re.MatchString(code)
}

// NewFetchRequest returns a FetchRequest
func NewFetchRequest(workflowID, stepID, stepPhase string) *pb.FetchRequest {
	return &pb.FetchRequest{
		Job: &pb.Job{
			Id: workflowID,
		},
		StepId:    stepID,
		StepPhase: stepPhase,
	}
}

// NewFetchResponse returns a FetchResponse
func NewFetchResponse(newReq *pb.FetchRequest, eof bool, responses []*pb.Response) *pb.FetchResponse {
	return &pb.FetchResponse{
		UpdatedFetchSince: newReq,
		Eof:               eof,
		Responses: &pb.FetchResponse_Responses{
			Response: responses,
		},
	}
}
