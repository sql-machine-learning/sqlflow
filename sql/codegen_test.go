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
	simpleInferSelect = simpleSelect + `INFER my_dnn_model;`
)

func TestCodeGenTrain(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	a.NoError(e)

	tpl, ok := NewTemplateFiller(&parseResult, fts, testCfg)
	a.Equal(true, ok)

	var text bytes.Buffer
	err := codegen_template.Execute(&text, tpl)
	if err != nil {
		log.Println("executing template:", err)
	}
	a.Equal(err, nil)

	cmd := exec.Command("docker", "run", "--rm", "--network=host", "-i", "tensorflow/tensorflow:1.12.0", "python")
	cmd.Stdin = bytes.NewReader(text.Bytes())
	o, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
	}
	a.True(strings.ContainsAny(string(o), "Done training"))
}
