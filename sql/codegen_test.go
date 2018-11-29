package sql

import (
	"io"
	"io/ioutil"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT MonthlyCharges, TotalCharges, tenure
FROM churn.churn
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 73,
  hidden_units = [10, 20]
COLUMN MonthlyCharges, TotalCharges
LABEL tenure
INTO
  my_dnn_model
;
`
	simplePredictSelect = simpleSelect + `
PREDICT churn.predict.tenure
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
		sqlParse(newLexer(simplePredictSelect))
	})


	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	// executor will fill in these field
	parseResult.estimator = "DNNClassifier"
	parseResult.attrs = make(map[string]*expr)
	parseResult.attrs["n_classes"] = &expr{typ: 1, val: "73"}
	parseResult.attrs["hidden_units"] = &expr{typ: 1, val: "[10, 20]"}

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, &parseResult, fts, testCfg))
		pw.Close()
	}()

	b, err := ioutil.ReadAll(pr)
	a.NoError(err)
	println(string(b))


	log.Printf("%#v\n", parseResult)
	log.Printf("%#v\n", fts)


	// Running a prediction job requires

}
