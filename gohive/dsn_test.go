package gohive

import "testing"


func TestParseDSN(t *testing.T) {
        dsn := "root:root@127.0.0.1/test"
        cfg, err := ParseDSN(dsn)
        if err != nil {
                t.Error(err.Error())
        }
        if "root" != cfg.User {
                t.Errorf("expected username is root but real value is %s", cfg.User)
        }
        if "root" != cfg.Passwd {
                t.Errorf("expected password is root but real value is %s", cfg.Passwd)
        }
        if "127.0.0.1" != cfg.Addr {
                t.Errorf("expected address is 127.0.0.1 but real value is %s", cfg.Addr)
        }
        if "test" != cfg.DBName {
                t.Errorf("expected address is test but real value is %s", cfg.DBName)
        }
}
