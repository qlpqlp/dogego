package main
import (
  "fmt"
  "os"
  "path/filepath"
  "dogego/store"
)
func main() {
  root := `C:\dogedata\mainnet`
  rbDir := filepath.Join(root, "rawblocks")
  rb, err := store.OpenRawBlockStore(rbDir, store.DefaultBlockStorageOpts())
  if err != nil { fmt.Println("open", err); os.Exit(1) }
  defer rb.Close()
  for _, h := range []int64{8301,8302,8303,10000,231484,232900} {
    has := rb.HasStoredBody(h)
    b, err := rb.Get(h)
    el := 0
    if b != nil { el = len(b) }
    fmt.Printf("h=%d has=%v get_err=%v len=%d\n", h, has, err, el)
  }
  tip, err := store.ReconcileBundledContiguousTip(rb)
  fmt.Printf("reconcile tip=%d err=%v\n", tip, err)
  if cp, err := store.LoadRawBlockSyncCheckpoint(root); err==nil {
    fmt.Printf("checkpoint contig=%d next=%d\n", cp.ContiguousRawHeight, cp.NextProbeHeight)
  }
}
