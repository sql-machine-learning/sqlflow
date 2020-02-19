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

package pai

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ExplainRender used for PAI
type ExplainRender struct {
	Code         string
	PaiCmd       string
	Requirements string
	// the explain result saved on OSS
	endpoint string
	ak       string
	sk       string
	bucket   string
	key      string
}

func newExplainRender(userID string) *ExplainRender {
	k := fmt.Sprintf("explain_images/%s/%s", userID, time.Now().Format("20060102150405"))
	return &ExplainRender{
		endpoint: os.Getenv("SQLFLOW_OSS_ALISA_ENDPOINT"),
		ak:       os.Getenv("SQLFLOW_OSS_AK"),
		sk:       os.Getenv("SQLFLOW_OSS_SK"),
		bucket:   os.Getenv("SQLFLOW_OSS_ALISA_BUCKET"),
		key:      k}
}

// Draw returns the explain result(png) as HTML
func (expn *ExplainRender) Draw() (string, error) {
	osscli, e := oss.New(expn.endpoint, expn.ak, expn.sk)
	if e != nil {
		return "", e
	}
	bucket, e := osscli.Bucket(expn.bucket)
	if e != nil {
		return "", e
	}
	r, e := bucket.GetObject(expn.key)
	if e != nil {
		return "", e
	}
	defer r.Close()
	return readAsHTML(r)
}

func readAsHTML(r io.Reader) (string, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	img := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("<div align='center'><img src='data:image/png;base64,%s' /></div>", img), nil
}
