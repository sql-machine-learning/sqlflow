package gohive

import "errors"

type Config struct {
	User             string
	Passwd           string
	Addr             string
        DBName           string
}

// standard dsn format: [user[:password]@]addr/dbname
func ParseDSN(dsn string) (config *Config, err error) {
	var cfg *Config = &Config{} 

	// Find the last '/'
	foundSlash := false
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			foundSlash = true
			var j, k int

			// parse [user[:password]@]addr
			if i > 0 {
				// [user[:password]
				for j = i; j >= 0; j-- {
					if dsn[j] == '@' {
						for k = 0; k < j; k++ {
							if dsn[k] == ':' {
								cfg.Passwd = dsn[k+1 : j]
								break
							}
						}
						cfg.User = dsn[:k]
						break
					}
				}
				// addr
				cfg.Addr = dsn[j+1 : i]
			}
			// parse dbname
			cfg.DBName = dsn[i+1 : len(dsn)]
			break
		}
	}
	if !foundSlash && len(dsn) > 0 {
		return nil, errors.New("valid dsn format is: [user[:password]@]addr/dbname") 
	}
	return cfg, nil
}
