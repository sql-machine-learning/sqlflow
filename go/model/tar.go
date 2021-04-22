// Copyright 2021 The SQLFlow Authors. All rights reserved.
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

package model

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func tarFile(filesource, prefix string, sfileInfo os.FileInfo, tarwriter *tar.Writer) error {
	sfile, err := os.Open(filesource)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer sfile.Close()
	header, err := tar.FileInfoHeader(sfileInfo, "")
	if err != nil {
		fmt.Println(err)
		return err
	}
	name := strings.TrimPrefix(strings.TrimPrefix(sfile.Name(), prefix), "/")
	header.Name = name
	err = tarwriter.WriteHeader(header)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if _, err = io.Copy(tarwriter, sfile); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func tarFolder(directory, prefix string, tarwriter *tar.Writer) error {

	return filepath.Walk(directory, func(targetpath string, file os.FileInfo, err error) error {
		//read the file failure
		if file == nil {
			return err
		}
		if file.IsDir() {
			if directory == targetpath {
				return nil
			}
			header, err := tar.FileInfoHeader(file, "")
			if err != nil {
				return err
			}
			name := strings.TrimPrefix(strings.TrimPrefix(targetpath, prefix), "/")
			header.Name = name
			if err = tarwriter.WriteHeader(header); err != nil {
				return err
			}
			os.Mkdir(strings.TrimPrefix(directory, file.Name()), os.ModeDir)
			//如果压缩的目录里面还有目录，则递归压缩
			return tarFolder(targetpath, prefix, tarwriter)
		}
		return tarFile(targetpath, prefix, file, tarwriter)
	})
}
