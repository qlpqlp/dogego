package main

import (
	"encoding/json"
	"fmt"
	"os"

	"dogego/config"
)

func main() {
	path := os.Getenv("APPDATA") + `\DogeGo\dogecoinconf.json`
	b, err := os.ReadFile(path)
	fmt.Println("read", path, "err", err, "len", len(b))
	var c config.File
	err = json.Unmarshal(b, &c)
	fmt.Println("unmarshal err", err)
	fmt.Printf("TLS=%v CA=%v DataDir=%q MaxOut=%d Workers=%d Layout=%q\n", c.WebUITLSLocal, c.LocalTLSTrustCA, c.DataDir, c.MaxOutbound, c.BlockSyncWorkers, c.BlockStorageLayout)
	f, p := config.LoadFirst()
	fmt.Println("LoadFirst path", p, "DataDir", f.DataDir, "TLS", f.WebUITLSLocal)
	for i, sp := range config.SearchPaths() {
		_, e := config.LoadFile(sp)
		fmt.Printf("search[%d] %s err=%v\n", i, sp, e)
	}
}
