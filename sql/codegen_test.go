package sql

import (
	"bytes"
	"log"
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
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	fts, e := verify(&parseResult, testCfg)
	assert.Nil(e)

	tpl, ok := NewTemplateFiller(&parseResult, fts, testCfg)
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
