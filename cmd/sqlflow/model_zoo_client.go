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

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/model"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/tar"
)

func getModelZooServerConn(opts *options) (*grpc.ClientConn, error) {
	if opts.ModelZooServer == "" {
		return nil, fmt.Errorf("Model Zoo server is not specified")
	}
	conn, err := grpc.Dial(opts.ModelZooServer, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create client error: %v", err)
	}
	return conn, nil
}

// tarRepo tar the repoDir and return the tar file path
func tarRepo(repoDir, tarName string) (string, error) {
	curDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(curDir)
	os.Chdir(repoDir)
	err = tar.ZipDir(".", tarName)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoDir, tarName), nil
}

func releaseModel(opts *options) error {
	if opts.DataSource == "" {
		return fmt.Errorf("You should specify a datasource with -d")
	}
	db, err := database.OpenDB(opts.DataSource)
	if err != nil {
		return err
	}
	defer db.Close()
	buf, err := model.LoadToBuffer(db, opts.ModelName)
	if err != nil {
		return err
	}
	modelData := buf.Bytes()
	// (TODO:lhw) decode the model from the buffer when
	// more metadata is stored in database, client should
	// not parse the original model.TrainSelect directly,
	// because parser should exist only on SQLFlow server

	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	stream, err := client.ReleaseModel(context.Background())
	if err != nil {
		return err
	}
	nameParts := strings.Split(opts.ModelName, ".")
	request := &pb.ReleaseModelRequest{
		Name: nameParts[len(nameParts)-1],
		Tag:  opts.Version,
		// (TODO: lhw) add following fields from model
		Description:       "",
		EvaluationMetrics: "",
		ModelClassName:    "MyDNNClassifier",
		// (FIXME: lhw) should extract from the model metadata once
		// we store it in the database
		ModelRepoImageUrl: "test/my_repo:v1.0",
		ContentTar:        modelData,
		ContentUrl:        "",
	}
	stream.Send(request)
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Message)
	}
	return nil
}

func deleteModel(opts *options) error {
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	// if user give a db.table format, we just use the table name
	nameParts := strings.Split(opts.ModelName, ".")
	req := &pb.ReleaseModelRequest{
		Name: nameParts[len(nameParts)-1],
		Tag:  opts.Version,
	}
	resp, err := client.DropModel(context.Background(), req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Message)
	}
	return nil
}

func checkModelZooParam(opts *options) error {
	// model zoo server is an optional param, so check it
	if opts.ModelZooServer == "" {
		return fmt.Errorf("you need to specify a model zoo server")
	}
	return nil
}

func releaseRepo(opts *options) error {
	if err := checkModelZooParam(opts); err != nil {
		return err
	}
	tarFile, err := tarRepo(opts.RepoDir,
		filepath.Base(opts.RepoName)+".tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tarFile)

	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	stream, err := client.ReleaseModelRepo(context.Background())
	if err != nil {
		return err
	}

	file, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 1024*32)
	for {
		size, e := file.Read(buf)
		if e == io.EOF {
			break
		} else if e != nil {
			return e
		}
		req := &pb.ReleaseModelRepoRequest{
			Name:       opts.RepoName,
			Tag:        opts.Version,
			ContentTar: buf[:size],
		}
		stream.Send(req)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Message)
	}
	return nil
}

func deleteRepo(opts *options) error {
	if err := checkModelZooParam(opts); err != nil {
		return err
	}
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	req := &pb.ReleaseModelRepoRequest{
		Name: opts.RepoName,
		Tag:  opts.Version,
	}
	resp, err := client.DropModelRepo(context.Background(), req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Message)
	}
	return nil
}
