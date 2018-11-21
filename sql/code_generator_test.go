package sql

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT MonthlyCharges, TotalCharges
FROM churn.churn
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN MonthlyCharges
LABEL TotalCharges
INTO
  my_dnn_model
;
`
	simpleInferSelect = simpleSelect + `INFER my_dnn_model;`
)

var cfg = connectionConfig{
	User:     "root",
	Password: "root",
	Host:     "localhost",
	Database: "yang",
	WorkDir:  "/tmp/"}

func TestCodeGenTrain(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	assert.Nil(e,
		"Make sure you are running the MySQL server in example/churn.")

	tpl, ok := NewTemplateFiller(&parseResult, fts, cfg)
	assert.Equal(true, ok)

	var text bytes.Buffer
	err := codegen_template.Execute(&text, tpl)
	if err != nil {
		log.Println("executing template:", err)
	}
	assert.Equal(err, nil)
}

// func TestCodeGenInfer(t *testing.T) {
// 	assert := assert.New(t)
// 	assert.NotPanics(func() {
// 		sqlParse(newLexer(simpleInferSelect))
// 	})
//
// 	// tpl = NewTemplateFiller(
// 	var text bytes.Buffer
// 	err := codegen_template.Execute(&text, parseResult)
// 	if err != nil {
// 		log.Println("executing template:", err)
// 	}
// 	assert.Equal(text.String(), ``)
// }
