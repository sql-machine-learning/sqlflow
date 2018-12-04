package sql

import (
	"fmt"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSelectChurn = `
SELECT *
FROM iris.iris
`
	testTrainSelectChurn = testSelectChurn + `
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_dnn_model
;
`
	testPredictSelectChurn = testSelectChurn + `
PREDICT iris.predict.class
USING my_dnn_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectChurn)
	a.NoError(e)

	fts, e := verify(r, testCfg)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, r, fts, testCfg))
		pw.Close()
	}()

	cmd := tensorflowCmd()
	cmd.Stdin = pr
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}

	a.True(strings.Contains(string(o), "Done training"))
	fmt.Println(string(o))
}

func TestCodeGenPredict(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelectChurn)
	a.NoError(e)
	var tc trainClause
	tc = r.trainClause

	r, e = newParser().Parse(testPredictSelectChurn)
	a.NoError(e)
	r.trainClause = tc

	fts, e := verify(r, testCfg)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, r, fts, testCfg))
		pw.Close()
	}()

	cmd := tensorflowCmd()
	cmd.Stdin = pr
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	a.True(strings.Contains(string(o), "Done predicting"))
}
