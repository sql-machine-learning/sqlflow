package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testSQLStatement = `select c1, c2, c3 from kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH 
			estimator.hidden_units = [10, 20]
		COLUMN 
			DENSE(c2, 5, comma), 
			cross([BUCKET(NUMERIC(c1, 10), [1, 10]), c5], 20),
			NUMERIC(c1, 10)
		LABEL c3 INTO model_table;`

	badSQLStatement = `select c1, c2, c3 from kaggle_credit_fraud_training_data 
		TRAIN DNNClassifier 
		WITH n_classes = 3 
		COLUMN 
			BUCKET(NUMERIC(c1, 10) + 10, [1, 10])
		LABEL c3 INTO model_table;`

	featureColumnCode = `[tf.feature_column.crossed_column([tf.feature_column.bucketized_column(tf.feature_column.numeric_column("c1", shape=(10,)), boundaries=[1,10]),"c5"], hash_bucket_size=20),tf.feature_column.numeric_column("c1", shape=(10,))]`

	estimatorCode = `tf.estimator.DNNClassifier(hidden_units=[10,20])`
)

func getFeatureColumnType(i interface{}) string {
	switch i.(type) {
	case *crossColumn:
		return "crossColumn"
	case *numericColumn:
		return "numericColumn"
	case *bucketColumn:
		return "bucketColumn"
	case *featureSpec:
		return "featureSpec"
	case string:
		return i.(string)
	}
	return "UNKNOWN"
}

func TestAlpsColumnResolve(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testSQLStatement)
	a.NoError(e)

	fcList, fsMap, err := resolveTrainColumns(&r.columns)

	a.NoError(err)

	fs := fsMap["c2"]
	fc := fcList[0]

	a.Equal("featureSpec", getFeatureColumnType(fs))
	a.Equal("c2", fs.FeatureName)
	a.Equal(5, fs.Shape[0])
	a.Equal(",", fs.Delimiter)

	a.Equal("crossColumn", getFeatureColumnType(fc))
	cl := fc.(*crossColumn)
	a.Equal(20, cl.HashBucketSize)

	a.Equal("bucketColumn", getFeatureColumnType(cl.Keys[0]))
	bl := cl.Keys[0].(*bucketColumn)
	nl2 := bl.SourceColumn
	a.Equal("c1", nl2.Key)
	a.Equal(10, nl2.Shape)
	a.Equal([]int{1, 10}, bl.Boundaries)

	a.Equal("c5", getFeatureColumnType(cl.Keys[1]))
}

func TestAlpsColumnResolveFailed(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(badSQLStatement)
	a.NoError(e)

	_, _, err := resolveTrainColumns(&r.columns)

	a.EqualError(err, "not supported expr in ALPS submitter: +")
}

func TestAlpsFeatureColumnCodeGenerate(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testSQLStatement)
	a.NoError(e)

	fcList, _, err := resolveTrainColumns(&r.columns)
	a.NoError(err)

	code, err := generateFeatureColumnCode(fcList)
	a.NoError(err)

	a.Equal(featureColumnCode, code)
}

func TestAlpsEstimatorCodeGenerate(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testSQLStatement)
	a.NoError(e)

	attrs, err := resolveTrainAttribute(&r.attrs)
	a.NoError(err)

	code, err := generateEstimatorCreator(r.estimator, filter(attrs, estimator))

	a.Equal(estimatorCode, code)
}
