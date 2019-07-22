package sql

import "io"

type xgboostTemplate struct {}

func (*xgboostTemplate) execute(w io.Writer, r *filler) error {
	panic("xgboostTemplate has not been implemented!")
}
