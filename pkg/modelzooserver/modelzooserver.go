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

package modelzooserver

import (
	"context"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

type modelZooServer struct {
}

func (s *modelZooServer) ListModelDefs(ctx context.Context, req *pb.Empty) (*pb.ListModelResponse, error) {
	return &pb.ListModelResponse{}, nil
}

func (s *modelZooServer) ListTrainedModels(ctx context.Context, req *pb.Empty) (*pb.ListModelResponse, error) {
	return &pb.ListModelResponse{}, nil
}

func (s *modelZooServer) ReleaseModelDef(stream pb.ModelZooServer_ReleaseModelDefServer) error {
	err := stream.SendAndClose(&pb.ModelResponse{})
	return err
}

func (s *modelZooServer) DropModelDef(ctx context.Context, req *pb.ModelRequest) (*pb.ModelResponse, error) {
	return &pb.ModelResponse{}, nil
}

func (s *modelZooServer) ReleaseTrainedModel(stream pb.ModelZooServer_ReleaseTrainedModelServer) error {
	err := stream.SendAndClose(&pb.ModelResponse{})
	return err
}

func (s *modelZooServer) DropTrainedModel(ctx context.Context, req *pb.ModelRequest) (*pb.ModelResponse, error) {
	return &pb.ModelResponse{}, nil
}
