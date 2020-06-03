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
	"os"
	"strings"

	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/sqlfs"
)

const modelCollTable = "sqlflow_model_zoo.model_repos"
const modelDefTable = "sqlflow_model_zoo.model_definitions"
const trainedModelTable = "sqlflow_model_zoo.models"
const publicModelDB = "sqlflow_public_models"

// TODO(typhoonzero): create tables if these tables are not pre created?
const createTableStmts = `CREATE DATABASE IF NOT EXISTS sqlflow_model_zoo;
DROP TABLE IF EXISTS sqlflow_model_zoo.models;
DROP TABLE IF EXISTS sqlflow_model_zoo.model_definitions;
DROP TABLE IF EXISTS sqlflow_model_zoo.model_repos;

CREATE TABLE sqlflow_model_zoo.model_repos (
    id INT AUTO_INCREMENT,
    name VARCHAR(255),
    version VARCHAR(255),
    PRIMARY KEY (id)
);

CREATE TABLE sqlflow_model_zoo.model_definitions (
    id INT AUTO_INCREMENT,
    model_coll_id INT,
    class_name VARCHAR(255),
    args_desc TEXT,
    PRIMARY KEY (id),
    FOREIGN KEY (model_coll_id) REFERENCES model_repos(id)
);

CREATE TABLE sqlflow_model_zoo.models (
    id INT AUTO_INCREMENT,
    model_def_id INT,
    name VARCHAR(255),
    version VARCHAR(255),
    url VARCHAR(255),
    description TEXT,
    metrics TEXT,
    PRIMARY KEY (id),
    FOREIGN KEY (model_def_id) REFERENCES model_definitions(id)
);`

type modelZooServer struct {
	DB *database.DB
}

func (s *modelZooServer) ListModelRepos(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelRepoResponse, error) {
	// TODO(typhoonzero): join model_repos
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

	responseList := &pb.ListModelRepoResponse{Size: 0}
	for rows.Next() {
		n := ""
		args := ""
		image := ""
		imagetag := ""
		if err := rows.Scan(&n, &args, &image, &imagetag); err != nil {
			return nil, err
		}
		perResp := &pb.ModelRepoResponse{
			ClassName: n,
			ArgDescs:  args,
			ImageUrl:  image,
			Tag:       imagetag,
		}
		responseList.ModelDefList = append(
			responseList.ModelDefList,
			perResp,
		)
	}
	return responseList, nil
}

func (s *modelZooServer) ListModels(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelResponse, error) {
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

	trainedModelList := &pb.ListModelResponse{Size: 0}
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
		perResp := &pb.ModelResponse{
			Name:          n,
			Tag:           v,
			ModelStoreUrl: url,
			Description:   desc,
			Metric:        m,
			ImageUrl:      fmt.Sprintf("%s:%s", imagename, imagetag),
		}
		trainedModelList.ModelList = append(
			trainedModelList.ModelList,
			perResp,
		)
	}

	return trainedModelList, nil
}

func (s *modelZooServer) ReleaseModelRepo(stream pb.ModelZooServer_ReleaseModelRepoServer) error {
	reqName := ""
	reqTag := ""

	fd, err := os.Create("servergot.tar.gz")
	if err != nil {
		return err
	}
	totalSize := 0
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		reqName = req.GetName()
		reqTag = req.GetTag()
		n, err := fd.Write(req.GetContentTar())
		if err != nil {
			fmt.Printf("write content error: %s\n", err)
			log.Printf("get user model source code error %v", err)
		}
		totalSize = totalSize + n
	}
	// close and flush the tar.gz file
	fd.Close()
	if err := checkImageURL(reqName); err != nil {
		return err
	}
	imgExists := imageExistsOnRegistry(reqName, reqTag)
	if imgExists {
		return fmt.Errorf("current image %s:%s already exists on registry", reqName, reqTag)
	}
	if totalSize <= 0 {
		return fmt.Errorf("no model repo content uploaded")
	}
	if err := os.Mkdir("modelrepo", os.ModeDir); err != nil {
		return err
	}
	if err := untarGzDir("servergot.tar.gz", "./modelrepo"); err != nil {
		return err
	}

	defer os.RemoveAll("./modelrepo")
	defer os.Remove("servergot.tar.gz")

	modelDescs, err := getModelClasses("./modelrepo")
	if len(modelDescs) == 0 {
		return fmt.Errorf("no model classes detected")
	}

	// do Docker image build and push
	dryrun := false
	if os.Getenv("SQLFLOW_TEST_DB") != "" {
		// do not push images when testing on CI
		dryrun = true
	}
	if err := buildAndPushImage("./modelrepo", reqName, reqTag, dryrun); err != nil {
		return err
	}

	// get model_repo id, if exists, return already existed error
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' and version='%s';", modelCollTable, reqName, reqTag)
	rows, err := s.DB.Query(sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasNext := rows.Next()
	if hasNext {
		return fmt.Errorf("model repo %s:%s already exists", reqName, reqTag)
	}

	// write model_repo
	sql = fmt.Sprintf("INSERT INTO %s (name, version) VALUES ('%s', '%s');", modelCollTable, reqName, reqTag)
	modelCollInsertRes, err := s.DB.Exec(sql)
	if err != nil {
		return err
	}

	// Write model_definition
	modelCollID, err := modelCollInsertRes.LastInsertId()
	if err != nil {
		return err
	}
	for _, desc := range modelDescs {
		sql = fmt.Sprintf("INSERT INTO %s (model_coll_id, class_name, args_desc) VALUES (%d, '%s', '%s')", modelDefTable, modelCollID, desc.Name, desc.DocString)
		if _, err := s.DB.Exec(sql); err != nil {
			return err
		}
	}

	return stream.SendAndClose(&pb.ReleaseResponse{Success: true, Message: ""})
}

func (s *modelZooServer) DropModelRepo(ctx context.Context, req *pb.ReleaseModelRepoRequest) (*pb.ReleaseResponse, error) {
	// 1. find model repo id
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
		return nil, fmt.Errorf("no model repo %s found", req.GetName())
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
	// 3. delete model repo record
	sql = fmt.Sprintf("DELETE FROM %s WHERE id=%d;", modelCollTable, id)
	if _, err := s.DB.Exec(sql); err != nil {
		return nil, err
	}
	return &pb.ReleaseResponse{Success: true, Message: ""}, nil
}

func (s *modelZooServer) ReleaseModel(stream pb.ModelZooServer_ReleaseModelServer) error {
	var req *pb.ReleaseModelRequest
	var err error
	var sqlf io.WriteCloser

	// sqlf is a sqlfs writer, it will be created when the first stream request arrives.
	// the uploaded model contents into MySQL using package sqlfs.
	sqlf = nil

	// Create database sqlflow_public_models to store public trained models.
	if _, err := s.DB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", publicModelDB)); err != nil {
		return err
	}
	// TODO(typhoonzero): save model to sqlfs with version tag

	for { // read stream request
		// NOTE: other fields in req must be the same in every stream request.
		streamReq, err := stream.Recv()
		if err == io.EOF {
			break
		}
		req = streamReq
		if sqlf == nil {

			modelTableName := fmt.Sprintf("%s.%s", publicModelDB, req.Name)
			// FIXME(typhoonzero): only hive need to pass session
			sqlf, err = sqlfs.Create(s.DB.DB, s.DB.DriverName, modelTableName, nil)
			if err != nil {
				return fmt.Errorf("cannot create sqlfs file %s: %v", modelTableName, err)
			}
			defer sqlf.Close()
		}

		_, err = sqlf.Write(req.GetContentTar())
		if err != nil {
			log.Printf("get user model source code error %v", err)
		}
	}

	if err := checkName(req.Name); err != nil {
		return err
	}

	// Get model_def_id from model_definition table
	imageAndTag := strings.Split(req.ModelRepoImageUrl, ":")
	if len(imageAndTag) != 2 {
		return fmt.Errorf("model repo image should be like you_image_name:version")
	}
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' AND version='%s';", modelCollTable, imageAndTag[0], imageAndTag[1])
	rowsImageID, err := s.DB.Query(sql)
	if err != nil {
		return err
	}
	defer rowsImageID.Close()
	end := rowsImageID.Next()
	if !end {
		return fmt.Errorf("when release model, no model repo %s found", req.ModelRepoImageUrl)
	}
	var modelCollID int
	if err = rowsImageID.Scan(&modelCollID); err != nil {
		return err
	}

	// TODO(typhoonzero): verify req.ModelClassName to avoid SQL injection.
	sql = fmt.Sprintf("SELECT id FROM %s WHERE class_name='%s' AND model_coll_id='%d'", modelDefTable, req.ModelClassName, modelCollID)
	rowsModelDefID, err := s.DB.Query(sql)
	if err != nil {
		return err
	}
	defer rowsModelDefID.Close()
	end = rowsModelDefID.Next()
	if !end {
		return fmt.Errorf("when release model, no model definition %s found", req.GetName())
	}
	var modelDefID int
	if err := rowsModelDefID.Scan(&modelDefID); err != nil {
		return err
	}
	// TODO(typhoonzero): let trained model name + version be unique across the table.
	sql = fmt.Sprintf("INSERT INTO %s (model_def_id, name, version, url, description, metrics) VALUES (%d, '%s', '%s', '%s', '%s', '%s')",
		trainedModelTable, modelDefID, req.Name, req.Tag, req.ContentUrl, req.Description, req.EvaluationMetrics)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.ReleaseResponse{Success: true, Message: ""})
}

func (s *modelZooServer) DropModel(ctx context.Context, req *pb.ReleaseModelRequest) (*pb.ReleaseResponse, error) {
	// TODO(typhoonzero): do not delete rows, set an deletion flag.
	sql := fmt.Sprintf("DELETE FROM %s WHERE name='%s' AND version='%s'", trainedModelTable, req.Name, req.Tag)
	if _, err := s.DB.Exec(sql); err != nil {
		return nil, err
	}
	return &pb.ReleaseResponse{Success: true, Message: ""}, nil
}

func (s *modelZooServer) DownloadModel(req *pb.ReleaseModelRequest, respStream pb.ModelZooServer_DownloadModelServer) error {
	modelName := req.Name
	modelTag := req.Tag
	modelClassName := req.ModelClassName
	modelRepoImageURL := req.ModelRepoImageUrl

	sqlf, err := sqlfs.Open(s.DB.DB, fmt.Sprintf("%s.%s", publicModelDB, modelName))
	if err != nil {
		return err
	}
	defer sqlf.Close()
	// Note that modelBuf is a gob encoded struct
	eof := false
	for {
		buf := make([]byte, 4096)
		n, err := sqlf.Read(buf)
		if err == io.EOF {
			eof = true
		} else if err != nil {
			return err
		}

		if n > 0 {
			err = respStream.Send(&pb.DownloadModelResponse{
				Name:              modelName,
				Tag:               modelTag,
				ModelClassName:    modelClassName,
				ModelRepoImageUrl: modelRepoImageURL,
				ContentTar:        buf,
			})
			if err != nil {
				return err
			}
		}
		// Need to write the last read bytes to streamResp when EOF
		if eof {
			break
		}
	}

	return nil
}
