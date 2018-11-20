package sql

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	simpleSelect = `
SELECT sepal_length, sepal_width, petal_length, petal_width, species
FROM irisis
`
	simpleTrainSelect = simpleSelect + `
TRAIN DNNClassifier
WITH 
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL species
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

var cts = columnTypes{
	Column:   []columnType{
			{Name: "sepal_length", Type: "numeric_column"},
			{Name: "sepal_width", Type: "numeric_column"},
			{Name: "petal_length", Type: "numeric_column"},
			{Name: "petal_width", Type: "numeric_column"}},
	Label:	columnType{Name: "species", Type: "numeric_column"}}

func TestCodeGenTrain(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(simpleTrainSelect))
	})

	tpl := NewTemplateFiller(&parseResult, cts, cfg)
	var text bytes.Buffer
	err := codegen_template.Execute(&text, tpl)
	if err != nil {
		log.Println("executing template:", err)
	}
	assert.Equal(err, nil)
	fmt.Println(text.String())
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
