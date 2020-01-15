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

// To run this program:
//	go generate .. && go run main.go
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/server"
	sf "sqlflow.org/sqlflow/pkg/sql"
)

func newServer(caCrt, caKey string) (*grpc.Server, error) {
	var s *grpc.Server
	if caCrt != "" && caKey != "" {
		creds, err := credentials.NewServerTLSFromFile(caCrt, caKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA crt/key files: %s, %s, %v", caCrt, caKey, err)
		}
		s = grpc.NewServer(grpc.Creds(creds))
		log.Println("Launch server with SSL/TLS certification.")
	} else {
		s = grpc.NewServer()
		log.Println("Launch server with insecure mode.")
	}
	return s, nil
}

func start(modelDir, caCrt, caKey string, port int, isArgoMode bool) {
	s, err := newServer(caCrt, caKey)
	if err != nil {
		log.Fatalf("failed to create new gRPC Server: %v", err)
	}

	if modelDir != "" {
		if _, derr := os.Stat(modelDir); derr != nil {
			os.Mkdir(modelDir, os.ModePerm)
		}
	}
	if isArgoMode {
		proto.RegisterSQLFlowServer(s, server.NewServer(sf.SubmitWorkflow, modelDir))
	} else {
		proto.RegisterSQLFlowServer(s, server.NewServer(sf.RunSQLProgram, modelDir))
	}

	listenString := fmt.Sprintf(":%d", port)

	lis, err := net.Listen("tcp", listenString)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server Started at", listenString)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	modelDir := flag.String("model_dir", "", "model would be saved on the local dir, otherwise upload to the table.")
	caCrt := flag.String("ca-crt", "", "CA certificate file.")
	caKey := flag.String("ca-key", "", "CA private key file.")
	port := flag.Int("port", 50051, "TCP port to listen on.")
	isArgoMode := flag.Bool("argo-mode", false, "Enable Argo workflow model.")
	flag.Parse()
	start(*modelDir, *caCrt, *caKey, *port, *isArgoMode)
}
