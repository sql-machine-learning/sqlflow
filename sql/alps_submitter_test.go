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
			cross([BUCKET(NUMERIC(c1, 10), [1, 10]), NUMERIC(c4, 9), c5], 20) 
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
	case *featureSpec:
		return "featureSpec"
	case string:
		return i.(string)
	}
	return "UNKNOWN"
}

func TestColumnResolve(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testSQLStatement)
	a.NoError(e)

	result, err := resolveTrainColumns(&r.columns)

	a.NoError(err)

	a.Equal("featureSpec", getFeatureColumnType(result[0]))
	a.Equal("c2", result[0].(*featureSpec).FeatureName)
	a.Equal(5, result[0].(*featureSpec).Shape[0])
	a.Equal("comma", result[0].(*featureSpec).Delimiter)

	a.Equal("CrossColumn", getFeatureColumnType(result[1]))
	cl := result[1].(*CrossColumn)
	a.Equal(20, cl.HashBucketSize)

	a.Equal("BucketColumn", getFeatureColumnType(cl.Keys[0]))
	bl := cl.Keys[0].(*BucketColumn)
	nl2 := bl.SourceColumn
	a.Equal("c1", nl2.Key)
	a.Equal(10, nl2.Shape)
	a.Equal([]int{1, 10}, bl.Boundaries)

	a.Equal("NumericColumn", getFeatureColumnType(cl.Keys[1]))
	nl := cl.Keys[1].(*NumericColumn)
	a.Equal("c4", nl.Key)
	a.Equal(9, nl.Shape)

	a.Equal("c5", getFeatureColumnType(cl.Keys[2]))

}

func TestColumnResolveFailed(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(badSQLStatement)
	a.NoError(e)

	_, err := resolveTrainColumns(&r.columns)

	a.EqualError(err, "not supported expr in ALPS submitter: +")
}