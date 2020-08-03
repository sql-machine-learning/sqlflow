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
	"io/ioutil"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/model"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/step/tablewriter"
	"sqlflow.org/sqlflow/go/tar"
)

func getModelZooServerConn(opts *options) (*grpc.ClientConn, error) {
	if opts.ModelZooServer == "" {
		return nil, fmt.Errorf("Model Zoo server is not specified")
	}

	var err error
	var conn *grpc.ClientConn
	if opts.CertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(opts.CertFile, "")
		if err != nil {
			return nil, err
		}
		conn, err = grpc.Dial(opts.ModelZooServer, grpc.WithTransportCredentials(creds))
	} else {
		conn, err = grpc.Dial(opts.ModelZooServer, grpc.WithInsecure())
	}
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
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	request := &pb.ReleaseModelRequest{
		Name:        opts.ModelName,
		Tag:         opts.Version,
		Description: opts.Description,
		DbConnStr:   opts.DataSource,
	}

	resp, err := client.ReleaseModel(context.Background(), request)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf(resp.Message)
	}
	return nil
}

func releaseModelFromLocal(opts *options) error {
	if opts.DataSource == "" {
		return fmt.Errorf("You should specify a datasource with -d")
	}
	db, err := database.OpenDB(opts.DataSource)
	if err != nil {
		return err
	}
	defer db.Close()
	dir, err := ioutil.TempDir("/tmp", "upload_model_zoo")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tarFile, err := model.DumpDBModel(db, opts.ModelName, dir)
	if err != nil {
		return err
	}
	model, err := model.ExtractMetaFromTarball(tarFile, dir)
	if err != nil {
		return err
	}
	sendFile, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer sendFile.Close()

	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	stream, err := client.ReleaseModelFromLocal(context.Background())
	if err != nil {
		return err
	}
	modelRepoImage := model.GetMetaAsString("model_repo_image")
	if modelRepoImage == "" {
		// use a default model repo image sqlflow/sqlflow:latest
		modelRepoImage = "sqlflow/sqlflow:latest"
	}

	request := &pb.ReleaseModelLocalRequest{
		Name:              opts.ModelName,
		Tag:               opts.Version,
		Description:       opts.Description,
		EvaluationMetrics: model.GetMetaAsString("evaluation"),
		ModelClassName:    model.GetMetaAsString("class_name"),
		ModelRepoImageUrl: modelRepoImage,
		ContentTar:        nil,
	}
	buf := make([]byte, 1024*10)
	for {
		if _, e := sendFile.Read(buf); e == io.EOF {
			break
		}
		request.ContentTar = buf
		stream.Send(request)
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

func deleteModel(opts *options) error {
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewModelZooServerClient(conn)
	req := &pb.ReleaseModelRequest{
		Name: opts.ModelName,
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

func listModels(opts *options) error {
	if err := checkModelZooParam(opts); err != nil {
		return err
	}
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()

	writer, err := tablewriter.Create("ascii", 1024, os.Stdout)
	if err != nil {
		return err
	}
	writer.SetHeader(map[string]interface{}{
		"columnNames": []string{
			"Name", "Tag", "ModelStoreUrl", "ImageUrl", "Description", "Metric"},
	})

	start := int64(0)
	client := pb.NewModelZooServerClient(conn)
	for {
		req := &pb.ListModelRequest{
			// (TODO: lhw) add authentication information in request
			Start: start,
			Size:  100,
		}
		resp, err := client.ListModels(context.Background(), req)
		if err != nil {
			return err
		}
		if resp.Size <= 0 {
			break
		}
		for _, m := range resp.ModelList {
			writer.AppendRow([]interface{}{
				m.Name, m.Tag, m.ModelStoreUrl, m.ImageUrl, m.Description, m.Metric,
			})
		}
		start += resp.Size
	}
	writer.Flush()
	return nil
}

func listRepos(opts *options) error {
	if err := checkModelZooParam(opts); err != nil {
		return err
	}
	conn, err := getModelZooServerConn(opts)
	if err != nil {
		return err
	}
	defer conn.Close()

	writer, err := tablewriter.Create("ascii", 1024, os.Stdout)
	if err != nil {
		return err
	}
	writer.SetHeader(map[string]interface{}{
		"columnNames": []string{
			"ClassName", "ImageUrl", "Tag", "ArgDescs"},
	})

	start := int64(0)
	client := pb.NewModelZooServerClient(conn)
	for {
		req := &pb.ListModelRequest{
			// (TODO: lhw) add authentication information in request
			Start: start,
			Size:  100,
		}
		resp, err := client.ListModelRepos(context.Background(), req)
		if err != nil {
			return err
		}
		if resp.Size <= 0 {
			break
		}
		for _, r := range resp.ModelDefList {
			writer.AppendRow([]interface{}{
				r.ClassName, r.ImageUrl, r.Tag, r.ArgDescs,
			})
		}
		start += int64(resp.Size)
	}
	writer.Flush()
	return nil
}
