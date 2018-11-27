package sql

import (
	"bytes"
	"log"
	"os/exec"
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
	a.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	var program bytes.Buffer
	a.NoError(generateTFProgram(&program, &parseResult, fts, testCfg))

	cmd := exec.Command("docker", "run", "--rm", "--network=host", "-i", "sqlflow", "python")
	cmd.Stdin = bytes.NewReader(program.Bytes())

	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	log.Println(string(o))
	a.True(strings.Contains(string(o), "Done training"))
}
