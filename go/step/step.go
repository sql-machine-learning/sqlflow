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

package step

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	"sqlflow.org/sqlflow/go/executor"
	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql"
	"sqlflow.org/sqlflow/go/step/tablewriter"
)

// RunSQLProgramAndPrintResult execute SQL statement and print the logs and select result
// TODO(yancey1989): more meanful argument isTerminal and it2Check
func RunSQLProgramAndPrintResult(sqlStmt string, modelDir string, session *pb.Session, table tablewriter.TableWriter, isTerminal, it2Check bool) error {
	startTime := time.Now().UnixNano()
	defer log.Printf("(%.2f sec)\n", float64(time.Now().UnixNano()-startTime)/1e9)
	// note(yancey1989): if the input sql program contain `\n`, bufio.Text() would deal with
	// it as a text string with two character instead of one character "\n".
	// readStmt(bufio.Scanner) should deal with that by replacing `\n` with '\n'
	// TODO(yancey1989): finding a normative way to deal with that.
	replacer := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r")
	sqlStmt = replacer.Replace(sqlStmt)

	log.SetFlags(0)
	stream := sql.RunSQLProgram(sqlStmt, modelDir, session)
	for res := range stream.ReadAll() {
		if e := Render(res, table, isTerminal, it2Check); e != nil {
			return e
		}
	}
	return table.Flush()
}

// Render output according to calling environment
func Render(rsp interface{}, table tablewriter.TableWriter, isTerminal, it2Check bool) error {
	switch s := rsp.(type) {
	case map[string]interface{}: // table header
		return table.SetHeader(s)
	case []interface{}: // row
		return table.AppendRow(s)
	case error:
		if isTerminal {
			log.Printf("ERROR: %v\n", s)
		} else {
			table.FlushWithError(s)
			log.Fatalf("workflow step failed: %v", s)
		}
	case sql.EndOfExecution:
	case executor.Figures:
		if isHTMLCode(s.Image) {
			if !isTerminal {
				printAsDataURL(s.Image)
				break
			}
			if image, e := getBase64EncodedImage(s.Image); e != nil {
				printAsDataURL(s.Image)
			} else if !it2Check {
				printAsDataURL(s.Image)
				log.Println("Or use iTerm2 as your terminal to view images.")
				log.Println(s.Text)
			} else if e = imageCat(image); e != nil {
				log.New(os.Stderr, "", 0).Printf("ERROR: %v\n", e)
				printAsDataURL(s.Image)
				log.Println(s.Text)
			}
		} else {
			log.Println(s)
		}
	case string:
		log.Println(s)
	case nil:
		return nil
	default:
		if isTerminal {
			log.Printf("unrecognized response type: %v", s)
		} else {
			log.Fatalf("unrecognized response type: %v", s)
		}
	}
	return nil
}

func isHTMLCode(code string) bool {
	//TODO(yancey1989): support more lines HTML code e.g.
	//<div>
	//  ...
	//</div>
	re := regexp.MustCompile(`<div.*?>.*</div>`)
	return re.MatchString(code)
}

func printAsDataURL(s string) {
	log.Println("data:text/html,", s)
	log.Println()
	log.Println("To view the content, paste the above data url to a web browser.")
}

func getBase64EncodedImage(s string) ([]byte, error) {
	match := regexp.MustCompile(`base64,(.*)'`).FindStringSubmatch(s)
	if len(match) == 2 {
		return base64.StdEncoding.DecodeString(match[1])
	}
	return []byte{}, fmt.Errorf("no images in the HTML")
}

func imageCat(imageBytes []byte) error {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return err
	}
	err = sixel.NewEncoder(os.Stdout).Encode(img)
	if err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// GetStdout hooks stdout and stderr, it collects stderr, stdout, and logs
// been generated during func f's execution, and returns them along with f's error
func GetStdout(f func() error) (out string, e error) {
	logOut, oldStdout, oldStderr := log.Writer(), os.Stdout, os.Stderr // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	log.SetOutput(w)
	outC := make(chan string)
	// first start read, then write, in case the pipe is immediately full and get blocked
	go func() { // copy the output in a separate goroutine so printing can't block indefinitely
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	e = f()                                     // f prints to stdout
	w.Close()                                   // Cancel redirection
	os.Stdout, os.Stderr = oldStdout, oldStderr // restoring the real stdout and stderr
	log.SetOutput(logOut)
	out = <-outC
	return
}
