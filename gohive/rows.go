package gohive

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/wangkuiyi/sqlflow/gohive/service-rpc/gen-go/tcliservice"
)

// rowSet implements the interface database/sql/driver.Rows.
type rowSet struct {
	thrift    *tcliservice.TCLIServiceClient
	operation *tcliservice.TOperationHandle
	options   Options

	columns    []*tcliservice.TColumnDesc
	columnStrs []string

	offset    int
	rowSet    *tcliservice.TRowSet
	hasMore   bool
	resultSet [][]interface{}
	nextRow   []interface{}
	status    *Status
}

type Status struct {
	state *tcliservice.TOperationState
}

func (r *rowSet) Next(dest []driver.Value) error {
	if r.status == nil || !r.status.IsStopped() {
		err := r.wait()
		if err != nil {
			return nil
		}
	}
	if r.status == nil {
		return fmt.Errorf("could not get job status.")
	}
	if !r.status.IsFinished() {
		return fmt.Errorf("job failed.")
	}
	if r.resultSet == nil || r.offset >= len(r.resultSet[0]) {
		if r.hasMore {
			r.batchFetch()
		} else {
			return io.EOF
		}
	}
	if len(r.resultSet) <= 0 {
		return fmt.Errorf("the length of resultSet is not greater than zero.")
	}
	r.nextRow = make([]interface{}, 0)
	for i, v := range r.resultSet {
		r.nextRow = append(r.nextRow, v[r.offset])
		dest[i] = v[r.offset]
	}
	r.offset++
	return nil
}

func (r *rowSet) Scan(dest ...interface{}) error {
	if r.nextRow == nil {
		return errors.New("No row to scan! Did you call Next() first?")
	}
	if len(dest) != len(r.nextRow) {
		return fmt.Errorf("Can't scan into %d arguments with input of "+
			"length %d", len(dest), len(r.nextRow))
	}
	for i, val := range r.nextRow {
		d := dest[i]
		switch dt := d.(type) {
		case *string:
			switch st := val.(type) {
			case string:
				*dt = st
			default:
				*dt = fmt.Sprintf("%v", val)
			}
		case *[]byte:
			*dt = []byte(val.(string))
		case *int:
			*dt = int(val.(int32))
		case *int64:
			*dt = val.(int64)
		case *int32:
			*dt = val.(int32)
		case *int16:
			*dt = val.(int16)
		case *float64:
			*dt = val.(float64)
		case *bool:
			*dt = val.(bool)
		default:
			return fmt.Errorf("Can't scan value of type %T with value %v", dt, val)
		}
	}
	return nil
}

// Returns the names of the columns for the given operation,
// blocking if necessary until the information is available.
func (r *rowSet) Columns() []string {
	if r.columnStrs == nil {
		if r.status == nil || !r.status.IsStopped() {
			err := r.wait()
			if err != nil {
				return nil
			}
		}
		if r.status == nil || !r.status.IsFinished() {
			return nil
		}
		ret := make([]string, len(r.columns))
		for i, col := range r.columns {
			ret[i] = col.ColumnName
		}

		r.columnStrs = ret
	}
	return r.columnStrs
}

func (r *rowSet) Close() (err error) {
	return nil
}

// Issue a thrift call to check for the job's current status.
func (r *rowSet) poll() error {
	req := tcliservice.NewTGetOperationStatusReq()
	req.OperationHandle = r.operation

	resp, err := r.thrift.GetOperationStatus(req)
	if err != nil {
		return fmt.Errorf("Error getting status: %+v, %v", resp, err)
	}
	if !isSuccessStatus(resp.Status) {
		return fmt.Errorf("GetStatus call failed: %s", resp.Status.String())
	}
	if resp.OperationState == nil {
		return errors.New("No error from GetStatus, but nil status!")
	}
	r.status = &Status{resp.OperationState}
	return nil
}

func (r *rowSet) wait() error {
	for {
		err := r.poll()
		if err != nil {
			return err
		}
		if r.status.IsStopped() {
			if r.status.IsFinished() {
				metadataReq := tcliservice.NewTGetResultSetMetadataReq()
				metadataReq.OperationHandle = r.operation

				metadataResp, err := r.thrift.GetResultSetMetadata(metadataReq)
				if err != nil {
					return err
				}
				if !isSuccessStatus(metadataResp.Status) {
					return fmt.Errorf("GetResultSetMetadata failed: %s",
						metadataResp.Status.String())
				}
				r.columns = metadataResp.Schema.Columns
				return nil
			} else {
				return fmt.Errorf("Query failed execution: %s", r.status.state.String())
			}
		}
		time.Sleep(time.Duration(r.options.PollIntervalSeconds) * time.Second)
	}
}

func (r *rowSet) batchFetch() error {
	r.offset = 0
	fetchReq := tcliservice.NewTFetchResultsReq()
	fetchReq.OperationHandle = r.operation
	fetchReq.Orientation = tcliservice.TFetchOrientation_FETCH_NEXT
	fetchReq.MaxRows = r.options.BatchSize

	resp, err := r.thrift.FetchResults(fetchReq)
	if err != nil {
		return err
	}
	if !isSuccessStatus(resp.Status) {
		return fmt.Errorf("FetchResults failed: %s\n", resp.Status.String())
	}
	r.offset = 0
	r.rowSet = resp.GetResults()
	r.hasMore = *resp.HasMoreRows

	rs := r.rowSet.Columns
	colLen := len(rs)
	r.resultSet = make([][]interface{}, colLen)

	for i := 0; i < colLen; i++ {
		v, length := convertColumn(rs[i])
		c := make([]interface{}, length)
		for j := 0; j < length; j++ {
			c[j] = reflect.ValueOf(v).Index(j).Interface()
		}
		r.resultSet[i] = c
	}
	return nil
}

func convertColumn(col *tcliservice.TColumn) (colValues interface{}, length int) {
	switch {
	case col.IsSetStringVal():
		return col.GetStringVal().GetValues(), len(col.GetStringVal().GetValues())
	case col.IsSetBoolVal():
		return col.GetBoolVal().GetValues(), len(col.GetBoolVal().GetValues())
	case col.IsSetByteVal():
		return col.GetByteVal().GetValues(), len(col.GetByteVal().GetValues())
	case col.IsSetI16Val():
		return col.GetI16Val().GetValues(), len(col.GetI16Val().GetValues())
	case col.IsSetI32Val():
		return col.GetI32Val().GetValues(), len(col.GetI32Val().GetValues())
	case col.IsSetI64Val():
		return col.GetI64Val().GetValues(), len(col.GetI64Val().GetValues())
	case col.IsSetDoubleVal():
		return col.GetDoubleVal().GetValues(), len(col.GetDoubleVal().GetValues())
	default:
		return nil, 0
	}
}

func (s Status) IsStopped() bool {
	if s.state == nil {
		return false
	}
	switch *s.state {
	case tcliservice.TOperationState_FINISHED_STATE,
		tcliservice.TOperationState_CANCELED_STATE,
		tcliservice.TOperationState_CLOSED_STATE,
		tcliservice.TOperationState_ERROR_STATE:
		return true
	}
	return false
}

func (s Status) IsFinished() bool {
	return s.state != nil && *s.state == tcliservice.TOperationState_FINISHED_STATE
}

func newRowSet(thrift *tcliservice.TCLIServiceClient,
	operation *tcliservice.TOperationHandle,
	options Options) driver.Rows {
	return &rowSet{thrift, operation, options, nil, nil,
		0, nil, true, nil, nil, nil}
}
