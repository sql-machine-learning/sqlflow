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
// limitations under the License

package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"sqlflow.org/sqlflow/go/executor"
	"sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/step"
	"sqlflow.org/sqlflow/go/step/tablewriter"
)

type renderContext struct {
	client     proto.SQLFlowClient
	ctx        context.Context
	stream     proto.SQLFlow_RunClient
	table      tablewriter.TableWriter
	isTerminal bool
	it2Check   bool
	output     *os.File
}

func render(ctx *renderContext, obj interface{}) error {
	var renderObj interface{}
	switch r := obj.(type) {
	case error:
		renderObj = obj
	case *proto.Response_Head:
		renderObj = map[string]interface{}{"columnNames": r.Head.ColumnNames}
	case *proto.Response_Row:
		var arr = make([]interface{}, 0, len(r.Row.Data))
		for _, any := range r.Row.Data {
			val, _ := proto.DecodePODType(any)
			arr = append(arr, val)
		}
		renderObj = arr
	case *proto.Response_Eoe:
	case *proto.Response_Job:
	case *proto.Response_Message:
		re := regexp.MustCompile(`<div.*?>.*</div>`)
		if re.MatchString(r.Message.Message) {
			renderObj = executor.Figures{
				Image: r.Message.Message,
			}
		} else {
			renderObj = r.Message.Message
		}
	default:
		renderObj = fmt.Sprintf("get un-recognized message type: %v", r)
	}
	return step.Render(renderObj, ctx.table, ctx.isTerminal, ctx.it2Check)
}

// renderRPCRespStream read all output from rpc
// and render them according to calling environment
func renderRPCRespStream(ctx *renderContext) (err error) {
	for {
		var resp *proto.Response
		resp, err = ctx.stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				err = nil
			} else {
				render(ctx, err)
			}
			break
		}
		render(ctx, resp.Response)
		// the last response type is Job for the workflow mode, so break the loop here
		if r, ok := resp.Response.(*proto.Response_Job); ok {
			req := &proto.FetchRequest{Job: &proto.Job{Id: r.Job.Id}}
			err = renderPRCFetchResp(ctx, req)
			if err != nil {
				render(ctx, err)
			}
			break
		}
	}
	ctx.table.Flush()
	return
}

func renderPRCFetchResp(ctx *renderContext, fetchReq *proto.FetchRequest) error {
	for {
		resp, err := ctx.client.Fetch(ctx.ctx, fetchReq)
		if err != nil {
			return err
		}
		for _, r := range resp.Responses.Response {
			render(ctx, r.Response)
		}
		if resp.Eof {
			return nil
		}
		fetchReq = resp.UpdatedFetchSince
		time.Sleep(time.Second)
	}
}
