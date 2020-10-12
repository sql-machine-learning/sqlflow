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
	"github.com/bitly/go-simplejson"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sqlflow.org/sqlflow/go/codegen/experimental"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/log"
	"sqlflow.org/sqlflow/go/model"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sqlfs"
	"sqlflow.org/sqlflow/go/tar"
)

const modelCollTable = "sqlflow_model_zoo.model_repos"
const modelDefTable = "sqlflow_model_zoo.model_definitions"
const trainedModelTable = "sqlflow_model_zoo.models"
const publicModelDB = "sqlflow_public_models"

const createTableStmts = `CREATE DATABASE IF NOT EXISTS sqlflow_model_zoo;

CREATE TABLE IF NOT EXISTS sqlflow_model_zoo.model_repos (
    id INT AUTO_INCREMENT,
    name VARCHAR(255),
    version VARCHAR(255),
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sqlflow_model_zoo.model_definitions (
    id INT AUTO_INCREMENT,
    model_coll_id INT,
    class_name VARCHAR(255),
    args_desc TEXT,
    PRIMARY KEY (id),
    FOREIGN KEY (model_coll_id) REFERENCES model_repos(id)
);

CREATE TABLE IF NOT EXISTS sqlflow_model_zoo.models (
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

// StartModelZooServer start the model zoo grpc server
func StartModelZooServer(port int, dbConnStr string) {
	logger := log.GetDefaultLogger()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	mysqlConn, err := database.OpenAndConnectDB(dbConnStr)
	if err != nil {
		logger.Fatalf("failed to connect to mysql: %v", err)
	}
	defer mysqlConn.Close()
	splitedStmts := strings.Split(createTableStmts, ";")
	for idx, stmt := range splitedStmts {
		if idx == len(splitedStmts)-1 {
			// the last stmt is empty
			break
		}
		_, err = mysqlConn.Exec(stmt)
		if err != nil {
			logger.Fatalf("failed to create model zoo tables: %v", err)
		}
	}
	// Add default model definitions so that models trained using DNNClassifier
	// can be released.
	if err := addDefaultModelDefs(mysqlConn); err != nil {
		logger.Fatalf("failed to add default model definitions: %v", err)
	}

	pb.RegisterModelZooServerServer(grpcServer, &modelZooServer{DB: mysqlConn})

	logger.Infof("SQLFlow Model Zoo started at: %d", port)
	grpcServer.Serve(lis)
}

// modelZooServer is the gRPC model zoo server implementation
type modelZooServer struct {
	DB *database.DB
}

func (s *modelZooServer) ListModelRepos(ctx context.Context, req *pb.ListModelRequest) (*pb.ListModelRepoResponse, error) {
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
		responseList.Size++
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
		trainedModelList.Size++
	}

	return trainedModelList, nil
}

func (s *modelZooServer) ReleaseModelRepo(stream pb.ModelZooServer_ReleaseModelRepoServer) error {
	reqName := ""
	reqTag := ""

	dir, err := ioutil.TempDir("/tmp", "sqlflow-zoo-repo")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	fd, err := os.Create(fmt.Sprintf("%s/servergot.tar.gz", dir))
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
			return err
		}
		totalSize = totalSize + n
	}
	// close and flush the tar.gz file
	fd.Close()
	if err := checkImageURL(reqName); err != nil {
		return err
	}
	if err := checkTag(reqTag); err != nil {
		return err
	}
	imgExists := imageExistsOnRegistry(reqName, reqTag)
	if imgExists {
		return fmt.Errorf("current image %s:%s already exists on registry", reqName, reqTag)
	}
	if totalSize <= 0 {
		return fmt.Errorf("no model repo content uploaded")
	}
	modelExtractDir := fmt.Sprintf("%s/modelrepo", dir)
	if err := os.Mkdir(modelExtractDir, 0755); err != nil {
		return err
	}
	if err := tar.UnzipDir(fmt.Sprintf("%s/servergot.tar.gz", dir), modelExtractDir); err != nil {
		return err
	}

	modelDescs, err := getModelClasses(modelExtractDir)
	if len(modelDescs) == 0 {
		return fmt.Errorf("no model classes detected")
	}

	// do Docker image build and push
	dryrun := false
	if os.Getenv("SQLFLOW_TEST_DB") != "" {
		// do not push images when testing on CI
		dryrun = true
	}
	_, err = os.Stat("/var/run/docker.sock")
	if os.IsNotExist(err) {
		// build image using kaniko if can not run `docker build`
		if err := buildAndPushImageKaniko(modelExtractDir, reqName, reqTag, dryrun); err != nil {
			return err
		}
	} else {
		if err := buildAndPushImage(modelExtractDir, reqName, reqTag, dryrun); err != nil {
			return err
		}
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
	if err := checkImageAndTag(req.GetName(), req.GetTag()); err != nil {
		return nil, err
	}

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

func (s *modelZooServer) ReleaseModel(ctx context.Context, req *pb.ReleaseModelRequest) (*pb.ReleaseResponse, error) {
	if err := checkNameAndTag(req.GetName(), req.GetTag()); err != nil {
		return nil, err
	}
	// Create database sqlflow_public_models to store public trained models.
	if _, err := s.DB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", publicModelDB)); err != nil {
		return nil, err
	}
	// download model from user's model database storage
	db, err := database.OpenDB(req.DbConnStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	dir, err := ioutil.TempDir("/tmp", "upload_model_zoo")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	var modelMeta *model.Model
	var sendFile io.ReadCloser
	if os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") == "true" {
		meta, err := experimental.GetModelMetadataFromDB(req.DbConnStr, req.Name)
		if err != nil {
			return nil, err
		}
		modelMeta = &model.Model{
			Meta: (*simplejson.Json)(meta),
		}
		modelMeta.TrainSelect = modelMeta.GetMetaAsString("original_sql")

		sendFile, err = sqlfs.Open(db.DB, req.Name)
		if err != nil {
			return nil, err
		}
	} else {
		tarFile, err := model.DumpDBModel(db, req.Name, dir)
		if err != nil {
			return nil, err
		}
		modelMeta, err = model.ExtractMetaFromTarball(tarFile, dir)
		if err != nil {
			return nil, err
		}

		sendFile, err = os.Open(tarFile)
		if err != nil {
			return nil, err
		}
	}

	defer sendFile.Close()

	// store modelname:tag as a unique name to the database, format the name like db_table_tag
	modelTableName := fmt.Sprintf("%s.%s_%s", publicModelDB, strings.ReplaceAll(req.Name, ".", "_"), strings.ReplaceAll(req.Tag, ".", "_"))
	modelRepoImage := modelMeta.GetMetaAsString("model_repo_image")
	if modelRepoImage == "" {
		// use a default model repo image: sqlflow/sqlflow:latest
		modelRepoImage = "sqlflow/sqlflow:latest"
	}
	imageAndTag := strings.Split(modelRepoImage, ":")
	if len(imageAndTag) == 2 {
		if err := checkImageURL(imageAndTag[0]); err != nil {
			return nil, err
		}
		if err := checkTag(imageAndTag[1]); err != nil {
			return nil, err
		}
	} else if len(imageAndTag) == 1 {
		if err := checkImageURL(modelRepoImage); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("model repo image should be format of [domain.com/group/]image[:tag]")
	}
	// FIXME(typhoonzero): only hive need to pass session
	sqlf, err := sqlfs.Create(s.DB, modelTableName, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create sqlfs file %s: %v", modelTableName, err)
	}
	defer sqlf.Close()

	buf := make([]byte, 4096)
	eof := false
	for !eof {
		n, err := sendFile.Read(buf)
		if err == io.EOF {
			eof = true
		} else if err != nil {
			return nil, fmt.Errorf("get user model source code error %v", err)
		}

		if n > 0 {
			_, err = sqlf.Write(buf[:n])
			if err != nil {
				return nil, fmt.Errorf("get user model source code error %v", err)
			}
		}
	}

	// Get model_def_id from model_definition table
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' AND version='%s';", modelCollTable, imageAndTag[0], imageAndTag[1])
	rowsImageID, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rowsImageID.Close()
	end := rowsImageID.Next()
	if !end {
		return nil, fmt.Errorf("when release model, no model repo %s found", modelRepoImage)
	}
	var modelCollID int
	if err = rowsImageID.Scan(&modelCollID); err != nil {
		return nil, err
	}

	modelClassName := modelMeta.GetMetaAsString("class_name")
	sql = fmt.Sprintf("SELECT id FROM %s WHERE class_name='%s' AND model_coll_id='%d'", modelDefTable, modelClassName, modelCollID)
	rowsModelDefID, err := s.DB.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rowsModelDefID.Close()
	end = rowsModelDefID.Next()
	if !end {
		return nil, fmt.Errorf("when release model, no model definition %s found", modelClassName)
	}
	var modelDefID int
	if err := rowsModelDefID.Scan(&modelDefID); err != nil {
		return nil, err
	}
	evalMetrics := modelMeta.GetMetaAsString("evaluation")

	// TODO(typhoonzero): let trained model name + version be unique across the table.
	// FIXME(typhoonzero): remove field url
	sql = fmt.Sprintf("INSERT INTO %s (model_def_id, name, version, url, description, metrics) VALUES (%d, '%s', '%s', '%s', '%s', '%s')",
		trainedModelTable, modelDefID, req.Name, req.Tag, "", req.Description, evalMetrics)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return nil, err
	}

	return &pb.ReleaseResponse{Success: true, Message: ""}, nil
}

func (s *modelZooServer) ReleaseModelFromLocal(stream pb.ModelZooServer_ReleaseModelFromLocalServer) error {
	var req *pb.ReleaseModelLocalRequest
	var err error
	var sqlf io.WriteCloser

	// sqlf is a sqlfs writer, it will be created when the first stream request arrives.
	// the uploaded model contents into MySQL using package sqlfs.
	sqlf = nil

	// Create database sqlflow_public_models to store public trained models.
	if _, err := s.DB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", publicModelDB)); err != nil {
		return err
	}

	for { // read stream request
		// NOTE: other fields in req must be the same in every stream request.
		streamReq, err := stream.Recv()
		if err == io.EOF {
			break
		}
		req = streamReq
		if sqlf == nil {
			// store modelname:tag as a unique name to the database, format the name like db_table_tag
			modelTableName := fmt.Sprintf("%s.%s_%s", publicModelDB, strings.ReplaceAll(req.Name, ".", "_"), strings.ReplaceAll(req.Tag, ".", "_"))
			// FIXME(typhoonzero): only hive need to pass session
			sqlf, err = sqlfs.Create(s.DB, modelTableName, nil)
			if err != nil {
				return fmt.Errorf("cannot create sqlfs file %s: %v", modelTableName, err)
			}
			defer sqlf.Close()
		}

		_, err = sqlf.Write(req.GetContentTar())
		if err != nil {
			return fmt.Errorf("get user model source code error %v", err)
		}
	}
	if err := checkNameAndTag(req.GetName(), req.GetTag()); err != nil {
		return err
	}

	// Get model_def_id from model_definition table
	imageAndTag := strings.Split(req.ModelRepoImageUrl, ":")
	if len(imageAndTag) != 2 {
		return fmt.Errorf("model repo image should be like you_image_name:version")
	}
	if err := checkImageURL(imageAndTag[0]); err != nil {
		return err
	}
	if err := checkTag(imageAndTag[1]); err != nil {
		return err
	}
	sql := fmt.Sprintf("SELECT id FROM %s WHERE name='%s' AND version='%s';", modelCollTable, imageAndTag[0], imageAndTag[1])
	rowsImageID, err := s.DB.Query(sql)
	if err != nil {
		return err
	}
	defer rowsImageID.Close()
	end := rowsImageID.Next()
	if !end {
		return fmt.Errorf("when release model local, no model repo %s found", req.ModelRepoImageUrl)
	}
	var modelCollID int
	if err = rowsImageID.Scan(&modelCollID); err != nil {
		return err
	}

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
	// FIXME(typhoonzero): field url is not used.
	sql = fmt.Sprintf("INSERT INTO %s (model_def_id, name, version, url, description, metrics) VALUES (%d, '%s', '%s', '%s', '%s', '%s')",
		trainedModelTable, modelDefID, req.Name, req.Tag, "", req.Description, req.EvaluationMetrics)
	_, err = s.DB.Exec(sql)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.ReleaseResponse{Success: true, Message: ""})
}

func (s *modelZooServer) DropModel(ctx context.Context, req *pb.ReleaseModelRequest) (*pb.ReleaseResponse, error) {
	// TODO(typhoonzero): do not delete rows, set an deletion flag.
	// TODO(typhoonzero): do we need to also delete the model table?
	if err := checkNameAndTag(req.GetName(), req.GetTag()); err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("DELETE FROM %s WHERE name='%s' AND version='%s'", trainedModelTable, req.Name, req.Tag)
	if _, err := s.DB.Exec(sql); err != nil {
		return nil, err
	}
	return &pb.ReleaseResponse{Success: true, Message: ""}, nil
}

// DownloadModel downloads the model from modelzoo.
// If the model is not present in the modelzoo storage,
// try download the model directly from the database, so that previously trained model can also be exported.
func (s *modelZooServer) DownloadModel(req *pb.ReleaseModelRequest, respStream pb.ModelZooServer_DownloadModelServer) error {
	modelName := req.Name
	modelTableName := strings.ReplaceAll(modelName, ".", "_")
	modelTag := req.Tag

	if err := checkNameAndTag(modelName, modelTag); err != nil {
		return err
	}

	sqlf, err := sqlfs.Open(s.DB.DB, fmt.Sprintf("%s.%s_%s", publicModelDB, modelTableName, strings.ReplaceAll(modelTag, ".", "_")))
	if err != nil {
		return err
	}
	defer sqlf.Close()

	if os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") == "true" {
		lengthHexStr := make([]byte, 10)
		n, err := sqlf.Read(lengthHexStr)
		if err != nil || n != 10 {
			return fmt.Errorf("read length head error: %v", err)
		}
		metaLength, err := strconv.ParseInt(string(lengthHexStr), 0, 64)
		if err != nil {
			return fmt.Errorf("convert length head error: %v", err)
		}

		metaHead := make([]byte, metaLength)
		metaN, err := sqlf.Read(metaHead)
		if err != nil || int64(metaN) != metaLength {
			return fmt.Errorf("read meta data error: %v", err)
		}
	}

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
				ModelClassName:    "", // FIXME(typhoonzero): not used.
				ModelRepoImageUrl: "",
				ContentTar:        buf[0:n],
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

// TODO(typhoonzero): Note that the current user may not have access to modelTableName,
// it may be trained by other users. We need to store the metadata in the model zoo table
// and get it directly.
func (s *modelZooServer) GetModelMeta(ctx context.Context, req *pb.ReleaseModelRequest) (*pb.GetModelMetaResponse, error) {
	if os.Getenv("SQLFLOW_USE_EXPERIMENTAL_CODEGEN") != "true" {
		return nil, fmt.Errorf("only support when SQLFLOW_USE_EXPERIMENTAL_CODEGEN=true")
	}

	if err := checkNameAndTag(req.Name, req.Tag); err != nil {
		return nil, err
	}

	modelTableName := fmt.Sprintf("%s.%s_%s", publicModelDB, strings.ReplaceAll(req.Name, ".", "_"), strings.ReplaceAll(req.Tag, ".", "_"))
	meta, err := experimental.GetModelMetadataFromDB(s.DB.URL(), modelTableName)
	if err != nil {
		return nil, err
	}

	encoded, err := (*simplejson.Json)(meta).Encode()
	if err != nil {
		return nil, err
	}

	return &pb.GetModelMetaResponse{
		Meta: string(encoded),
	}, nil
}
