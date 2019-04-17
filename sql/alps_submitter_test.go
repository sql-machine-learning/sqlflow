package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSQLStatement = `select c1, c2, c3 from kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH n_classes = 3 
		COLUMN 
			DENSE(c2, 5, comma), 
			CROSS([BUCKET(NUMERIC(c1, 10), [1, 10]), NUMERIC(c4, 9), c5], 20) 
		LABEL c3 INTO model_table;`

	badSQLStatement = `select c1, c2, c3 from kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH n_classes = 3 
		COLUMN 
			BUCKET(NUMERIC(c1, 10) + 10, [1, 10])
		LABEL c3 INTO model_table;`
)

func getFeatureColumnType(i interface{}) string {
	switch i.(type) {
	case *CrossColumn:
		return "CrossColumn"
	case *NumericColumn:
		return "NumericColumn"
	case *BucketColumn:
		return "BucketColumn"
	case *FeatureSpec:
		return "FeatureSpec"
	}
	return "UNKNOWN"
}

func TestColumnResolve(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testSQLStatement)
	a.NoError(e)

	result, err := resolveTrainColumns(&r.columns)

	a.NoError(err)
	a.Equal("FeatureSpec", getFeatureColumnType(result[0]))
	a.Equal("CrossColumn", getFeatureColumnType(result[1]))
	a.Equal("BucketColumn", getFeatureColumnType(result[1].(*CrossColumn).Keys[0]))
	a.Equal("NumericColumn", getFeatureColumnType(result[1].(*CrossColumn).Keys[1]))
}

func TestColumnResolveFailed(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(badSQLStatement)
	a.NoError(e)

	_, err := resolveTrainColumns(&r.columns)

	a.EqualError(err, "not supported expr in ALPS submitter: +")
}
