package main

import (
	pb "github.com/wangkuiyi/sqlflowserver"
)

type sqlFlowServer struct{}

func (s *sqlFlowServer) Run(*pb.RunRequest, pb.SQLFlow_RunServer) error {
	return nil
}
