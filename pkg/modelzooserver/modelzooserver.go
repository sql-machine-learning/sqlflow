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
	"fmt"
	"io"
	"log"

	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

const modelCollTable = "sqlflow_model_zoo.model_collection"
const modelDefTable = "sqlflow_model_zoo.model_definition"
const trainedModelTable = "sqlflow_model_zoo.trained_model"

// TODO(typhoonzero): create tables if these tables are not pre created.
const createTableStmts = `CREATE TABLE sqlflow_model_zoo.model_collection (
    id INT AUTO_INCREMENT,
    name VARCHAR(255),
    version VARCHAR(255),
    PRIMARY KEY (id)
);

CREATE TABLE sqlflow_model_zoo.model_definition (
    id INT AUTO_INCREMENT,
    model_coll_id INT,
    class_name VARCHAR(255),
    args_desc VARCHAR(255),
    PRIMARY KEY (id),
    FOREIGN KEY (model_coll_id) REFERENCES model_collection(id)
);


CREATE TABLE sqlflow_model_zoo.trained_model (
    id INT AUTO_INCREMENT,
    model_def_id INT,
    name VARCHAR(255),
    version VARCHAR(255),
    url VARCHAR(255),
    description TEXT,
    metrics TEXT,
    PRIMARY KEY (id),
    FOREIGN KEY (model_def_id) REFERENCES model_definition(id)
);`

type modelZooServer struct {
	DB *database.DB
}

func (s *modelZooServer) ListModelDefs(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelResponse, error) {
	// TODO(typhoonzero): join model_collection
	var sql string
	if req.Size <= 0 {
		sql = fmt.Sprintf("SELECT class_name, args_desc FROM %s;", modelDefTable)
	} else {
		sql = fmt.Sprintf("SELECT class_name, args_desc FROM %s LIMIT %d OFFSET %d;",
			modelDefTable, req.Size, req.Start)
	}
	rows, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	arglist := []string{}
	for rows.Next() {
		n := ""
		args := ""
		if err := rows.Scan(&n, &args); err != nil {
			return nil, err
		}
		names = append(names, n)
		arglist = append(arglist, args)
	}
	return &pb.ListModelResponse{Names: names, Tags: arglist, Size: int64(len(names))}, nil
}

func (s *modelZooServer) ListTrainedModels(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelResponse, error) {
	return &pb.ListModelResponse{}, nil
}

func (s *modelZooServer) ReleaseModelDef(stream pb.ModelZooServer_ReleaseModelDefServer) error {
	reqName := ""
	reqTag := ""
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		reqName = req.GetName()
		reqTag = req.GetTag()
		if err != nil {
			log.Printf("ReleaseModelDef error %v", err)
		}
	}
	// TODO(typhoonzero): Check the reqName should be of the format:
	// hub.docker.com/group/mymodel
	// group/mymodel
	// mymodel

	// TODO(typhoonzero): validate the uploaded tar contains valid models.

	// write model_collection
	sql := fmt.Sprintf("INSERT INTO %s (name, version) VALUES ('%s', '%s');", modelCollTable, reqName, reqTag)
	modelCollInsertRes, err := s.DB.Exec(sql)
	if err != nil {
		return err
	}
	// TODO(typhoonzero): Get details information from the uploaded tar

	// Write model_definition
	modelCollID, err := modelCollInsertRes.LastInsertId()
	if err != nil {
		return err
	}
	sql = fmt.Sprintf("INSERT INTO %s (model_coll_id, class_name, args_desc) VALUES (%d, 'DNNClassifier', '{aaa,bbb}')", modelDefTable, modelCollID)
	if _, err := s.DB.Exec(sql); err != nil {
		return err
	}

	return stream.SendAndClose(&pb.ModelResponse{Success: true, Message: ""})
}

func (s *modelZooServer) DropModelDef(ctx context.Context, req *pb.ModelDefRequest) (*pb.ModelResponse, error) {
	// 1. find model collection id
	// TODO(typhoonzero): verify request strings to avoid SQL injection
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' and version='%s';",
		modelCollTable, req.GetName(), req.GetTag())
	rows, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var id int
	end := rows.Next()
	if !end {
		return nil, fmt.Errorf("no model collection %s found", req.GetName())
	}
	err = rows.Scan(&id)
	if err != nil {
		return nil, err
	}
	// 2. delete all recoreds in model_definition that have the model_coll_id
	sql = fmt.Sprintf("DELETE FROM %s WHERE model_coll_id=%d;", modelDefTable, id)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return nil, err
	}
	// 3. delete model collection record
	sql = fmt.Sprintf("DELETE FROM %s WHERE id=%d;", modelCollTable, id)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return nil, err
	}
	return &pb.ModelResponse{Success: true, Message: ""}, nil
}

func (s *modelZooServer) ReleaseTrainedModel(stream pb.ModelZooServer_ReleaseTrainedModelServer) error {
	err := stream.SendAndClose(&pb.ModelResponse{})
	return err
}

func (s *modelZooServer) DropTrainedModel(ctx context.Context, req *pb.TrainedModelRequest) (*pb.ModelResponse, error) {
	return &pb.ModelResponse{}, nil
}
