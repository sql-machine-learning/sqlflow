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
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	var pResult extendedSelect
	a.NotPanics(func() {
		pResult = Parse(simpleTrainSelect)
	})

	fts, e := verify(&pResult, testCfg)
	a.NoError(e)

	pr, pw := io.Pipe()
	go func() {
		a.NoError(generateTFProgram(pw, &pResult, fts, testCfg))
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
