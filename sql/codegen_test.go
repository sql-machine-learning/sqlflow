package sql

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSelectIris = `
SELECT *
FROM iris.iris
`
	testTrainSelectIris = testSelectIris + `
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_dnn_model
;
`
	testPredictSelectIris = testSelectIris + `
predict iris.predict.class
USING my_dnn_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)

	fts, e := verify(r, testDB)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(genTF(pw, r, fts, testCfg))
		pw.Close()
	}()

	// NOTE: the temporary directory must be in a host directory
	// which can be mounted to Docker containers.  If I don't
	// specify the "/tmp" prefix, ioutil.TempDir would by default
	// generate a directory in /private/tmp for macOS, which
	// cannot be mounted by Docker into the container.  For more
	// detailed, please refer to
	// https://docs.docker.com/docker-for-mac/osxfs/#namespaces.
	cwd, e := ioutil.TempDir("/tmp", "sqlflow-codegen_test")
	a.NoError(e)
	defer os.RemoveAll(cwd)

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = pr
	o, e := cmd.CombinedOutput()
	a.NoError(e)
	a.True(strings.Contains(string(o), "Done training"))
}

func TestCodeGenPredict(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectIris)
	a.NoError(e)
	tc := r.trainClause

	r, e = newParser().Parse(testPredictSelectIris)
	a.NoError(e)
	r.trainClause = tc

	fts, e := verify(r, testDB)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(genTF(pw, r, fts, testCfg))
		pw.Close()
	}()

	cwd, e := ioutil.TempDir("/tmp", "sqlflow-codegen_test")
	a.NoError(e)
	defer os.RemoveAll(cwd)

	cmd := tensorflowCmd(cwd)
	cmd.Stdin = pr
	o, e := cmd.CombinedOutput()
	a.NoError(e)
	a.True(strings.Contains(string(o), "Done predicting"))
}
