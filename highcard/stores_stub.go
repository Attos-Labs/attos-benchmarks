//go:build !stores

package main

import "fmt"

// LMDB and RocksDB need cgo and system libraries; build with `-tags stores` (see README).
func openStore(name, dir string, n uint64) (target, func()) {
	must(fmt.Errorf("%s needs a build with -tags stores (cgo, liblmdb-dev and librocksdb-dev)", name), "store")
	return nil, nil
}
