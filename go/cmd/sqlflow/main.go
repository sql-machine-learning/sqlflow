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
	"bufio"
	"bytes"
	"context"
	_ "image/png"
	"time"

	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/step"

	pb "sqlflow.org/sqlflow/go/proto"
	"sqlflow.org/sqlflow/go/sql"
	"sqlflow.org/sqlflow/go/step/tablewriter"

	docopt "github.com/docopt/docopt-go"
)

const tablePageSize = 1000

// current using db, used to emulate SQL session keeping
var currentDB string

// dotEnvFilename is the filename of the .env file
const dotEnvFilename string = ".sqlflow_env"

// if we are on sixel supported platform, assumed to be true for now
var it2Check = true

const usage = `SQLFlow Command-line Tool.

Usage:
    sqlflow [options] [run] [-e <program> -f <file>]
    sqlflow [options] release repo [--force] <repo_dir> <repo_name> <version>
    sqlflow [options] release model [--force] [--local] [--desc=<desc>] <model_name> <version>
    sqlflow [options] get model <model_name>
    sqlflow [options] delete repo <repo_name> <version>
    sqlflow [options] delete model <model_name> <version>
    sqlflow [options] list repo
    sqlflow [options] list model

Options:
    -v, --version                   	print the version and exit
    -h, --help                      	print this screen
    -c, --cert-file=<file>          	cert file to connect SQLFlow or Model Zoo server
        --env-file=<file>           	config file in KEY=VAL format
    -s, --sqlflow-server=<addr>     	SQLFlow server address and port, e.g localhost:50051
    -m, --model-zoo-server=<addr>   	Model Zoo server address and port
    -d, --data-source=<data_source>   data source to use when run or release model
    -u, --user=<user>               	Model Zoo user account
    -p, --password=<password>       	Model Zoo user password

Run Options:
    -e, --execute=<program>           execute given program
    -f, --file=<file>                 execute program in file

Release Options:
        --force                  force overwrite existing model
        --local                  release a model stores in a database that can be connected from local
        --desc=<desc>            description for this model`

type options struct {
	CertFile, EnvFile    string
	SQLFlowServer        string `docopt:"--sqlflow-server"`
	ModelZooServer       string `docopt:"--model-zoo-server"`
	DataSource           string
	Execute              string
	Run                  bool
	File                 string
	Delete, Release, Get bool
	Repo, Model          bool
	Force                bool
	Local                bool
	RepoDir              string `docopt:"<repo_dir>"`
	RepoName             string `docopt:"<repo_name>"`
	ModelName            string `docopt:"<model_name>"`
	Version              string `docopt:"<version>"`
	Description          string `docopt:"--desc"`
	LocalFile            string
	List                 bool
	User                 string
	Password             string
}

func isSpace(c byte) bool {
	return len(bytes.TrimSpace([]byte{c})) == 0
}

// addLineToStmt scans lines into statements, the last four parameters are both input/output.
// A user must initialize `inQuotedString` and `isSingleQuoted` to false and `statements` to []
// at the first call
func addLineToStmt(line string, inQuotedString, isSingleQuoted *bool, statements *[]string) bool {
	if len(*statements) == 0 { // First line of the statements
		*statements = append(*statements, "")
		line = strings.TrimLeft(line, "\t ")
	} else {
		(*statements)[len(*statements)-1] += "\n"
	}
	var isEscape bool // Escaping in quoted string cannot cross lines
	var start, i int

	// note(yancey1989): if the input sql program contain `\n`, bufio.Text() would deal with
	// it as a text string with two character instead of one character "\n".
	// readStmt(bufio.Scanner) should deal with that by replacing `\n` with '\n'
	// TODO(yancey1989): finding a normative way to deal with that.
	replacer := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r")
	line = replacer.Replace(line)

	for i = 0; i < len(line); i++ {
		if isEscape {
			isEscape = false
			continue
		}
		switch line[i] {
		case '\\':
			if *inQuotedString {
				isEscape = true
			}
		case '"', '\'':
			if *inQuotedString {
				if *isSingleQuoted == (line[i] == '\'') {
					*inQuotedString = false // We found the end of a quoted string
				}
			} else { // The start of a quoted string
				*inQuotedString = true
				*isSingleQuoted = (line[i] == '\'')
			}
		case ';':
			if !*inQuotedString { // We found a statement
				// Ignore empty statement that has only a ';'
				if i == 0 && len((*statements)[len(*statements)-1]) != 0 {
					(*statements)[len(*statements)-1] += line[start : i+1]
				} else if i != start {
					(*statements)[len(*statements)-1] += line[start : i+1]
				}
				for i+1 < len(line) && isSpace(line[i+1]) {
					i++ // Ignore leading whitespaces of the next statement
				}
				start = i + 1
				if start == len(line) {
					return true // All done, the last character in the line is the end of a statement
				}
				if len((*statements)[len(*statements)-1]) != 0 {
					// Prepare for searching the next statement: reuse the buffer if the current statement is empty
					*statements = append(*statements, "")
				}
			}
		case '-':
			if !*inQuotedString {
				if i+1 < len(line) && line[i+1] == '-' {
					if i+2 == len(line) || isSpace(line[i+2]) { // We found a line comment
						// Note: `--` comment doesn't interfere with quoted-string and `;`
						(*statements)[len(*statements)-1] += strings.TrimSpace(line[start:i])
						if len(*statements) == 1 && (*statements)[0] == "" {
							*statements = []string{}
							return true // The whole line is an empty statement that has only a `-- comment`,
						}
						return false
					}
				}
			}
		}
	}
	(*statements)[len(*statements)-1] += line[start:]
	return false
}

// readStmt reads a SQL statement from the scanner.  A statement could have
// multiple lines and ends at a semicolon at the end of the last line.
func readStmt(scn *bufio.Scanner) ([]string, error) {
	stmt := []string{}
	var inQuotedString, isSingleQuoted bool
	for scn.Scan() {
		if addLineToStmt(scn.Text(), &inQuotedString, &isSingleQuoted, &stmt) {
			return stmt, nil
		}
	}
	// If the the file doesn't ends with ';', we consider the remaining content as a statement
	if scn.Err() == nil {
		return stmt, io.EOF
	}
	return stmt, scn.Err()
}

func flagPassed(name ...string) bool {
	found := false
	for _, n := range name {
		flag.Visit(func(f *flag.Flag) {
			if f.Name == n {
				found = true
			}
		})
	}
	return found
}

func createRPCConn(opts *options) (*grpc.ClientConn, error) {
	if opts.CertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(opts.CertFile, "")
		if err != nil {
			return nil, err
		}
		return grpc.Dial(opts.SQLFlowServer, grpc.WithTransportCredentials(creds))
	}
	return grpc.Dial(opts.SQLFlowServer, grpc.WithInsecure())
}

func sqlRequest(program string, ds string) *pb.Request {
	se := sql.MakeSessionFromEnv()
	se.DbConnStr = getDataSource(ds, currentDB)
	return &pb.Request{Sql: program, Session: se}
}

func isExitStmt(stmt string) bool {
	separatorIndex := strings.Index(stmt, ";")
	if separatorIndex < 0 {
		separatorIndex = len(stmt)
	}

	firstStmt := stmt[0:separatorIndex]
	firstStmt = strings.ToUpper(strings.TrimSpace(firstStmt))
	return firstStmt == "EXIT" || firstStmt == "QUIT"
}

func runStmt(opts *options, stmt string, isTerminal bool) error {
	if isExitStmt(stmt) {
		fmt.Println("Goodbye!")
		os.Exit(0)
	}

	// special case, process USE to stick SQL session
	parts := strings.Fields(strings.ReplaceAll(stmt, ";", ""))
	if len(parts) == 2 && strings.ToUpper(parts[0]) == "USE" {
		return switchDatabase(opts, parts[1])
	}
	return runStmtOnServer(opts, stmt, isTerminal)
}

func runStmtOnServer(opts *options, stmt string, isTerminal bool) error {
	// render output according to environment
	logFlags := log.Flags()
	log.SetFlags(0)
	defer log.SetFlags(logFlags)

	if !isTerminal {
		fmt.Println("sqlflow>", stmt)
	}

	table, err := tablewriter.Create("ascii", tablePageSize, os.Stdout)
	if err != nil {
		log.Println(err)
		return err
	}
	// connect to sqlflow server and run sql program
	conn, err := createRPCConn(opts)
	if err != nil {
		log.Println(err)
		return err
	}
	defer conn.Close()
	client := pb.NewSQLFlowClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 36000*time.Second)
	defer cancel()
	req := sqlRequest(stmt, opts.DataSource)
	stream, err := client.Run(ctx, req)
	if err != nil {
		log.Println(err) // log the connection failure
		return err
	}

	renderCtx := &renderContext{
		client:     client,
		ctx:        ctx,
		stream:     stream,
		table:      table,
		isTerminal: isTerminal,
		it2Check:   it2Check,
	}
	return renderRPCRespStream(renderCtx)
}

func assertConnectable(opts *options) {
	conn, err := createRPCConn(opts)
	if err != nil {
		log.Fatalf("can't connect to %s: %v", opts.SQLFlowServer, err)
	}
	conn.Close()
}

func repl(opts *options, scanner *bufio.Scanner) {
	for {
		statements, err := readStmt(scanner)
		if err == io.EOF && len(statements) == 0 {
			return
		}
		// The collaborative parsing algorithm requires that each statement ends
		// with a semi-colon, as the definition of `end_of_stmt`
		// in /pkg/parser/extended_syntax_parser.y#L176 .
		n := len(statements)
		if n > 0 && !strings.HasSuffix(strings.TrimSpace(statements[n-1]), ";") {
			statements[len(statements)-1] += ";"
		}
		for _, stmt := range statements {
			if err := runStmt(opts, stmt, false); err != nil {
				log.Fatalf("run SQL statement failed: %v", err)
			}
		}
	}
}

func switchDatabase(opts *options, db string) error {
	stmt := "USE " + db
	out, err := step.GetStdout(func() error {
		return runStmtOnServer(opts, stmt, true)
	})
	if err != nil {
		fmt.Println(out)
		return err
	}
	fmt.Println("Database changed to", db)
	currentDB = db
	return nil
}

// getDataSource generates a data source string that is using database `db` from the original dataSource
func getDataSource(dataSource, db string) string {
	if db == "" {
		return dataSource
	}
	driver, other, e := database.ParseURL(dataSource)
	if e != nil {
		log.Fatalf("unrecognized data source '%s'", dataSource)
	}
	pieces := strings.Split(other, "?")
	switch driver {
	case "maxcompute", "alisa":
		var v url.Values = url.Values{}
		if len(pieces) == 2 {
			v, e = url.ParseQuery(pieces[1])
			if e != nil {
				log.Fatalf("unrecognized data source '%s'", dataSource)
			}
		}
		v["curr_project"] = []string{db}
		return fmt.Sprintf("%s://%s?%s", driver, pieces[0], v.Encode())
	case "mysql":
		fallthrough
	case "hive":
		pieces[0] = strings.Split(pieces[0], "/")[0] + "/" + db
		return fmt.Sprintf("%s://%s", driver, strings.Join(pieces, "?"))
	}
	log.Fatalf("unknown database '%s' in data source'%s'", driver, dataSource)
	return ""
}

// initEnvFromFile initializes environment variables from the .env file
func initEnvFromFile(f string) {
	_ = godotenv.Load(f)
}

func runSQLFlowClient(opts *options) error {
	if opts.SQLFlowServer == "" {
		opts.SQLFlowServer = os.Getenv("SQLFLOW_SERVER")
	}
	if opts.SQLFlowServer == "" {
		return fmt.Errorf("SQLFlow server address is not provided")
	}
	if opts.DataSource == "" {
		opts.DataSource = os.Getenv("SQLFLOW_DATASOURCE")
	}
	if opts.DataSource == "" {
		return fmt.Errorf("data source is not provided")
	}
	if opts.CertFile == "" {
		opts.CertFile = os.Getenv("SQLFLOW_CA_CRT")
	}
	assertConnectable(opts) // Fast fail if we can't connect to the datasource
	var err error
	if currentDB, err = database.GetDatabaseName(opts.DataSource); err != nil {
		return err
	}

	// You might want to use syscall.Stdin instead of 0; however,
	// unfortunately, we cannot.  the syscall standard package has
	// a special implementation for Windows, where the type of
	// syscall.Stdin is not int as in Linux and macOS, but
	// uintptr.
	isTerminal := opts.File == "" && opts.Execute == "" && terminal.IsTerminal(0)
	sqlFile := os.Stdin

	if opts.File != "" && opts.File != "-" {
		if sqlFile, err = os.Open(opts.File); err != nil {
			return err
		}
		defer sqlFile.Close()
	}
	var reader io.Reader = sqlFile
	// Override stdin and file when the `-e|-execute' options are present.
	if opts.Execute != "" {
		reader = strings.NewReader(strings.TrimSpace(opts.Execute))
	}
	scanner := bufio.NewScanner(reader)
	if isTerminal {
		if !it2Check {
			fmt.Println("The terminal doesn't support sixel, explanation statements will show ASCII figures.")
		}
		// TODO(lorylin): get autocomplete dicts for sqlflow_models from sqlflow_server
		runPrompt(func(stmt string) {
			runStmt(opts, stmt, true)
		})
	} else {
		repl(opts, scanner)
	}
	return nil
}

func processOptions(opts *options) {
	var err error
	switch {
	case opts.Run:
		err = runSQLFlowClient(opts)
	case opts.Release && opts.Model:
		if opts.Local {
			err = releaseModelFromLocal(opts)
		} else {
			err = releaseModel(opts)
		}
	case opts.Release && opts.Repo:
		err = releaseRepo(opts)
	case opts.Delete && opts.Model:
		err = deleteModel(opts)
	case opts.Delete && opts.Repo:
		err = deleteRepo(opts)
	case opts.List && opts.Model:
		err = listModels(opts)
	case opts.List && opts.Repo:
		err = listRepos(opts)
	case opts.Get && opts.Model:
		err = downloadModelFromDB(opts)
	default:
		err = runSQLFlowClient(opts)
	}
	if err != nil {
		log.Printf("Failed due to %v", err)
	}
}

func main() {
	opts, err := docopt.ParseArgs(usage, nil, "1.0.0")
	if err != nil {
		log.Fatal(err)
	}
	optionData := &options{}
	if err := opts.Bind(optionData); err != nil {
		log.Fatal(err)
	}
	var envFilePath string
	if optionData.EnvFile != "" {
		envFilePath = optionData.EnvFile
	} else {
		envFilePath = filepath.Join(os.Getenv("HOME"), dotEnvFilename)
	}
	initEnvFromFile(envFilePath)
	processOptions(optionData)
}
