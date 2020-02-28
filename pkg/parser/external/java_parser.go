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

package external

import (
	"context"
	"fmt"
	"sqlflow.org/sqlflow/pkg/proto"
)

type javaParser struct {
	typ string
}

// typ should be either "hive" or "calcite".
func newJavaParser(typ string) *javaParser {
	return &javaParser{typ: typ}
}

func (p *javaParser) Parse(program string) ([]string, int, error) {
	c, err := connectToServer()
	if err != nil {
		return nil, -1, err
	}

	r, err := proto.NewParserClient(c).Parse(context.Background(), &proto.ParserRequest{Dialect: p.typ, SqlProgram: program})
	if err != nil {
		return nil, -1, err
	}
	if r.Error != "" {
		return nil, -1, fmt.Errorf(r.Error)
	}
	return r.SqlStatements, int(r.Index), nil
}
