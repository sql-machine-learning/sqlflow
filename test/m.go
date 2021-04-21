package main

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ClickHouse/clickhouse-go"
)

func main() {
	// ds := "clickhouse://tcp(192.168.31.114:9000)/?database=iris"
	// fmt.Println(ds)
	// driver, other, e := database.ParseURL(ds)
	// fmt.Println(driver)
	// fmt.Println(other)
	// r := strings.Replace(other, "tcp(", "tcp://", -1)
	// r = strings.Replace(r, ")/", "/", 1)
	// url := r
	// fmt.Println(r)
	// db, err := database.OpenDB(url)
	// if err != nil {
	// 	fmt.Errorf("failed to open database: %v", err)
	// }
	// if err := db.Ping(); err != nil {
	// 	fmt.Errorf("failed to ping database: %v %v", url, err)
	// }
	// fmt.Println(e)

	p := findMetaPath("/tmp/sqlflow_models364238697", "model_meta.json")
	fmt.Println(p)
}

func findMetaPath(dst, target string) string {
	ret := filepath.Join(dst, target)
	f, e := os.Stat(dst)
	if e != nil {
		fmt.Printf("%v", e)
		return ret
	}
	if f.IsDir() {
		filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == target {
				ret = path
			}
			return nil
		})
	}
	return ret
}
