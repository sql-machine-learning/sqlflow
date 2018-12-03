package sql

import (
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT *
FROM iris.iris
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO my_dnn_model
;
`
	simplePredictSelect = simpleSelect + `
PREDICT iris.predict.class
USING my_dnn_model;
`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, &parseResult, fts, testCfg))
		pw.Close()
	}()

	cmd := tensorflowCmd()
	cmd.Stdin = pr
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}

	a.True(strings.Contains(string(o), "Done training"))
}

func TestCodeGenPredict(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})
	var tc trainClause
	tc = parseResult.trainClause

	a.NotPanics(func() {
		sqlParse(newLexer(simplePredictSelect))
	})
	parseResult.trainClause = tc

	log.Printf("%#v\n", tc.attrs["n_classes"])
	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, &parseResult, fts, testCfg))
		pw.Close()
	}()

	cmd := tensorflowCmd()
	cmd.Stdin = pr
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	log.Println(string(o))
	a.True(strings.Contains(string(o), "Done predicting"))
}
