package gohive

import (
	"database/sql"
	"database/sql/driver"
	"errors"

	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/sql-machine-learning/sqlflow/gohive/service-rpc/gen-go/tcliservice"
)

type drv struct{}

func (d drv) Open(dsn string) (driver.Conn, error) {
	cfg, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	transport, err := thrift.NewTSocket(cfg.Addr)
	if err != nil {
		return nil, err
	}

	if err := transport.Open(); err != nil {
		return nil, err
	}

	if transport == nil {
		return nil, errors.New("nil thrift transport")
	}

	protocol := thrift.NewTBinaryProtocolFactoryDefault()
	client := tcliservice.NewTCLIServiceClientFactory(transport, protocol)
	s := tcliservice.NewTOpenSessionReq()
	s.ClientProtocol = 6
	if cfg.User != "" {
		s.Username = &cfg.User
		if cfg.Passwd != "" {
			s.Password = &cfg.Passwd
		}
	}
	session, _ := client.OpenSession(s)
	if err != nil {
		return nil, err
	}

	options := Options{PollIntervalSeconds: 5, BatchSize: 100000}
	conn := &Connection{client, session.SessionHandle, options}
	return conn, nil
}

func init() {
	sql.Register("hive", &drv{})
}
