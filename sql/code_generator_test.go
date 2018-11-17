package sql

import (
    "testing"

	"fmt"
	"os"
    "reflect"
    "log"
	// "text/template"

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
COLUMN *
INTO
  my_dnn_model
;
`
    simpleinferSelect = simpleSelect + `INFER my_dnn_model;`
)

func TestCodeGenTrain(t *testing.T) {
    assert := assert.New(t)
	assert.NotPanics(func() {
        sqlParse(newLexer(simpleTrainSelect))
    })
    fmt.Println(parseResult.Extended)
    fmt.Println(parseResult.Train)
    fmt.Println(parseResult.StandardSelect.String())
    fmt.Println(parseResult.TrainClause.Estimator)
    fmt.Println(parseResult.TrainClause.Attrs["n_classes"])
    fmt.Println(parseResult.TrainClause.Attrs["hidden_units"])
    fmt.Println(parseResult.TrainClause.Save)

    fmt.Println(reflect.TypeOf(parseResult))
    f, _ := os.Create("./train.py")
    defer f.Close()

    err := codegen_template.Execute(f, parseResult)
    if err != nil {
        log.Println("executing template:", err)
    }
}

func TestCodeGenInfer(t *testing.T) {
    assert := assert.New(t)
	assert.NotPanics(func() {
        sqlParse(newLexer(simpleinferSelect))
    })
    fmt.Println(parseResult.Extended)
    fmt.Println(parseResult.Train)
    fmt.Println(parseResult.StandardSelect.String())

    fmt.Println(reflect.TypeOf(parseResult))
    f, _ := os.Create("./infer.py")
    defer f.Close()

    err := codegen_template.Execute(f, parseResult)
    if err != nil {
        log.Println("executing template:", err)
    }
}
