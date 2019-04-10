package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRunSelect(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT * FROM churn.train LIMIT 10;`)
	a.NoError(e)
	a.Nil(dryRunSelect(r, testDB))
}

func TestDescribeTables(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT * FROM churn.train LIMIT 10;`)
	a.NoError(e)
	fts, e := describeTables(r, testDB)
	a.NoError(e)
	a.Equal(21, len(fts))

	r, e = newParser().Parse(`SELECT Churn, churn.train.Partner,TotalCharges FROM churn.train LIMIT 10;`)
	a.NoError(e)
	fts, e = describeTables(r, testDB)
	a.NoError(e)
	a.Equal(3, len(fts))
	a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, fts["Churn"]["churn.train"])
	a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, fts["Partner"]["churn.train"])
	a.Equal("FLOAT", fts["TotalCharges"]["churn.train"])
}

func TestIndexSelectFields(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT * FROM churn.train LIMIT 10;`)
	a.NoError(e)
	f := indexSelectFields(r)
	a.Equal(0, len(f))

	r, e = newParser().Parse(`SELECT f FROM churn.train LIMIT 10;`)
	a.NoError(e)
	f = indexSelectFields(r)
	a.Equal(1, len(f))
	a.Equal(map[string]string{}, f["f"])

	r, e = newParser().Parse(`SELECT t1.f, t2.f, g FROM churn.train LIMIT 10;`)
	a.NoError(e)
	f = indexSelectFields(r)
	a.Equal(2, len(f))
	a.Equal(map[string]string{}, f["g"])
	a.Equal("", f["f"]["t1"])
	a.Equal("", f["f"]["t2"])
}

func TestVerify(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT Churn, churn.train.Partner FROM churn.train LIMIT 10;`)
	a.NoError(e)
	fts, e := verify(r, testDB)
	a.NoError(e)
	a.Equal(2, len(fts))
	typ, ok := fts.get("Churn")
	a.Equal(true, ok)
	a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, typ)

	typ, ok = fts.get("churn.train.Partner")
	a.Equal(true, ok)
	a.Contains([]string{"VARCHAR(255)", "VARCHAR"}, typ)

	_, ok = fts.get("churn.train.gender")
	a.Equal(false, ok)

	_, ok = fts.get("gender")
	a.Equal(false, ok)
}

func TestVerifyColumnNameAndType(t *testing.T) {
	a := assert.New(t)
	trainParse, e := newParser().Parse(`SELECT gender, tenure, TotalCharges
FROM churn.train LIMIT 10
TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN gender, tenure, TotalCharges
LABEL class
INTO sqlflow_models.my_dnn_model;`)
	a.NoError(e)

	predParse, e := newParser().Parse(`SELECT gender, tenure, TotalCharges
FROM churn.train LIMIT 10
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`)
	a.NoError(e)
	a.NoError(verifyColumnNameAndType(trainParse, predParse, testDB))

	predParse, e = newParser().Parse(`SELECT gender, tenure
FROM churn.train LIMIT 10
PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;`)
	a.NoError(e)
	a.EqualError(verifyColumnNameAndType(trainParse, predParse, testDB),
		"predFields doesn't contain column TotalCharges")
}

func TestDescribeEmptyTables(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT * FROM iris.iris_empty LIMIT 10;`)
	a.NoError(e)
	_, e = describeTables(r, testDB)
	a.EqualError(e, "table is Empty. table name: iris.iris_empty")
}
