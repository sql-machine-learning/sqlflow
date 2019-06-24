// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import "time"

// DBConnCache caches the DB object
type DBConnCache struct {
	cache      map[string]*DB
	expiration time.Duration
}

// NewDBConnCache returns a new DBConneCache instance.
func NewDBConnCache(exp time.Duration) *DBConnCache {
	return &DBConnCache{cache: make(map[string]*DB), expiration: exp}
}

// Get a cached DB connection object.
func (dc *DBConnCache) Get(key string) (*DB, bool) {
	dbConn, ok := dc.cache[key]
	return dbConn, ok
}

// Set caches a DB connection object.
func (dc *DBConnCache) Set(key string, db *DB) {
	dc.cache[key] = db
}

// RemoveInactiveDB removes the inactive DB connections from the cache pool.
func (dc *DBConnCache) RemoveInactiveDB() {
	// TODO(Yancey1989): delete the inactive DB connectino object.
}
