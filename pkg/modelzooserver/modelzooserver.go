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
	"strings"

	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

const modelCollTable = "sqlflow_model_zoo.model_collection"
const modelDefTable = "sqlflow_model_zoo.model_definition"
const trainedModelTable = "sqlflow_model_zoo.trained_model"

// TODO(typhoonzero): create tables if these tables are not pre created?
const createTableStmts = `CREATE DATABASE IF NOT EXISTS sqlflow_model_zoo;
DROP TABLE IF EXISTS sqlflow_model_zoo.trained_model;
DROP TABLE IF EXISTS sqlflow_model_zoo.model_definition;
DROP TABLE IF EXISTS sqlflow_model_zoo.model_collection;

CREATE TABLE sqlflow_model_zoo.model_collection (
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

func (s *modelZooServer) ListModelDefs(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelDefResponse, error) {
	// TODO(typhoonzero): join model_collection
	var sql string
	if req.Size <= 0 {
		sql = fmt.Sprintf("SELECT class_name, args_desc, b.name, b.version FROM %s LEFT JOIN %s AS b ON %s.model_coll_id=b.id;",
			modelDefTable, modelCollTable, modelDefTable)
	} else {
		sql = fmt.Sprintf("SELECT class_name, args_desc, b.name, b.version FROM %s LEFT JOIN %s AS b ON %s.model_coll_id=b.id LIMIT %d OFFSET %d;",
			modelDefTable, modelCollTable, modelDefTable, req.Size, req.Start)
	}
	rows, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	arglist := []string{}
	images := []string{}
	imagetags := []string{}

	for rows.Next() {
		n := ""
		args := ""
		image := ""
		imagetag := ""
		if err := rows.Scan(&n, &args, &image, &imagetag); err != nil {
			return nil, err
		}
		names = append(names, n)
		arglist = append(arglist, args)
		images = append(images, image)
		imagetags = append(imagetags, imagetag)
	}
	return &pb.ListModelDefResponse{
			ClassNames: names,
			Tags:       imagetags,
			ImageUrls:  images,
			ArgDescs:   arglist,
			Size:       int64(len(names))},
		nil
}

func (s *modelZooServer) ListTrainedModels(ctx context.Context, req *pb.ListModelRequest) (*pb.ListTrainedModelResponse, error) {
	var sql string
	if req.Size <= 0 {
		sql = fmt.Sprintf(`SELECT a.name, a.version, a.url, a.description, a.metrics, c.name, c.version FROM %s AS a
LEFT JOIN %s AS b ON a.model_def_id=b.id
LEFT JOIN %s AS c ON b.model_coll_id=c.id;`,
			trainedModelTable, modelDefTable, modelCollTable)
	} else {
		sql = fmt.Sprintf(`SELECT a.name, a.version, a.url, a.description, a.metrics, c.name, c.version FROM %s AS a
LEFT JOIN %s AS b ON a.model_def_id=b.id
LEFT JOIN %s AS c ON b.model_coll_id=c.id LIMIT %d OFFSET %d;`,
			trainedModelTable, modelDefTable, modelCollTable, req.Size, req.Start)
	}
	rows, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []string{}
	modelVersions := []string{}
	urls := []string{}
	descs := []string{}
	metrics := []string{}
	imageurls := []string{}

	for rows.Next() {
		n := ""
		v := ""
		url := ""
		desc := ""
		m := ""
		imagename := ""
		imagetag := ""
		if err := rows.Scan(&n, &v, &url, &desc, &m, &imagename, &imagetag); err != nil {
			return nil, err
		}
		names = append(names, n)
		modelVersions = append(modelVersions, v)
		urls = append(urls, url)
		descs = append(descs, desc)
		metrics = append(metrics, m)
		imageURL := fmt.Sprintf("%s:%s", imagename, imagetag)
		imageurls = append(imageurls, imageURL)
	}

	return &pb.ListTrainedModelResponse{
		Names:          names,
		Tags:           modelVersions,
		ModelStoreUrls: urls,
		ImageUrls:      imageurls,
		Descriptions:   descs,
		Metrics:        metrics,
		Size:           int64(len(names)),
	}, nil
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
	// TODO(typhoonzero): replace DNNClassifier with the real name.
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

func (s *modelZooServer) ReleaseTrainedModel(ctx context.Context, req *pb.TrainedModelRequest) (*pb.ModelResponse, error) {
	// Get model_def_id from model_definition table
	imageAndTag := strings.Split(req.ModelCollectionImageUrl, ":")
	if len(imageAndTag) != 2 {
		return nil, fmt.Errorf("model collection image should be like you_image_name:version")
	}
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' AND version='%s';", modelCollTable, imageAndTag[0], imageAndTag[1])
	rowsImageID, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rowsImageID.Close()
	end := rowsImageID.Next()
	if !end {
		return nil, fmt.Errorf("no model collection %s found", req.GetName())
	}
	var modelCollID int
	if err = rowsImageID.Scan(&modelCollID); err != nil {
		return nil, err
	}

	// TODO(typhoonzero): verify req.ModelClassName to avoid SQL injection.
	sql = fmt.Sprintf("SELECT id FROM %s WHERE class_name='%s' AND model_coll_id='%d'", modelDefTable, req.ModelClassName, modelCollID)
	rowsModelDefID, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rowsModelDefID.Close()
	end = rowsModelDefID.Next()
	if !end {
		return nil, fmt.Errorf("no model collection %s found", req.GetName())
	}
	var modelDefID int
	if err = rowsModelDefID.Scan(&modelDefID); err != nil {
		return nil, err
	}
	// TODO(typhoonzero): let trained model name + version be unique across the table.
	sql = fmt.Sprintf("INSERT INTO %s (model_def_id, name, version, url, description, metrics) VALUES (%d, '%s', '%s', '%s', '%s', '%s')",
		trainedModelTable, modelDefID, req.Name, req.Tag, req.ContentUrl, req.Description, req.EvaluationMetrics)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return nil, err
	}
	return &pb.ModelResponse{Success: true, Message: ""}, nil
}

func (s *modelZooServer) DropTrainedModel(ctx context.Context, req *pb.TrainedModelRequest) (*pb.ModelResponse, error) {
	sql := fmt.Sprintf("DELETE FROM %s WHERE name='%s' AND version='%s'", trainedModelTable, req.Name, req.Tag)
	_, err := s.DB.Exec(sql)
	if err != nil {
		return nil, err
	}
	return &pb.ModelResponse{Success: true, Message: ""}, nil
}
