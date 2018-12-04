package sql

import (
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testTrainSelectChurn = `
SELECT MonthlyCharges, TotalCharges, tenure
FROM churn.churn
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
}
