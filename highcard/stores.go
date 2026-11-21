//go:build stores

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/linxGnu/grocksdb"
)

// forEachKV streams dir/kv.bin (the same records every other target loads).
func forEachKV(dir string, n uint64, fn func(k, v []byte)) {
	f, err := os.Open(filepath.Join(dir, "kv.bin"))
	must(err, "kv.bin")
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var hdr [2]byte
	buf := make([]byte, 64)
	for i := uint64(0); i < n; i++ {
		_, err := io.ReadFull(r, hdr[:])
		must(err, "hdr")
		kl, vl := int(hdr[0]), int(hdr[1])
		_, err = io.ReadFull(r, buf[:kl+vl])
		must(err, "kv")
		fn(buf[:kl], buf[kl:kl+vl])
	}
}

type lmdbT struct {
	env *lmdb.Env
	dbi lmdb.DBI
	buf []byte
}

// get uses one read transaction per lookup and copies the value out (LMDB values are only valid in the txn).
func (t *lmdbT) get(k []byte) ([]byte, bool) {
	ok := false
	t.env.View(func(tx *lmdb.Txn) error {
		tx.RawRead = true
		if v, err := tx.Get(t.dbi, k); err == nil {
			t.buf = append(t.buf[:0], v...)
			ok = true
		}
		return nil
	})
	return t.buf, ok
}

type rocksT struct {
	db  *grocksdb.DB
	ro  *grocksdb.ReadOptions
	buf []byte
}

func (t *rocksT) get(k []byte) ([]byte, bool) {
	v, err := t.db.Get(t.ro, k)
	if err != nil || !v.Exists() {
		return nil, false
	}
	t.buf = append(t.buf[:0], v.Data()...)
	v.Free()
	return t.buf, true
}

// openStore builds the store from kv.bin on first use (timed, reported), then opens it read-only.
func openStore(name, dir string, n uint64) (target, func()) {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		t0 := time.Now()
		switch name {
		case "lmdb":
			os.MkdirAll(p, 0o755)
			env, _ := lmdb.NewEnv()
			env.SetMapSize(int64(n)*200 + 1<<30)
			must(env.Open(p, 0, 0o644), "lmdb open")
			var k, v [][]byte
			flush := func() {
				must(env.Update(func(tx *lmdb.Txn) error {
					dbi, err := tx.OpenRoot(0)
					if err != nil {
						return err
					}
					for i := range k {
						if err := tx.Put(dbi, k[i], v[i], 0); err != nil {
							return err
						}
					}
					return nil
				}), "lmdb put")
				k, v = k[:0], v[:0]
			}
			forEachKV(dir, n, func(kk, vv []byte) {
				k, v = append(k, append([]byte(nil), kk...)), append(v, append([]byte(nil), vv...))
				if len(k) == 50000 {
					flush()
				}
			})
			flush()
			env.Close()
		case "rocksdb":
			o := grocksdb.NewDefaultOptions()
			o.SetCreateIfMissing(true)
			db, err := grocksdb.OpenDb(o, p)
			must(err, "rocksdb open")
			wo := grocksdb.NewDefaultWriteOptions()
			forEachKV(dir, n, func(kk, vv []byte) { must(db.Put(wo, kk, vv), "rocksdb put") })
			db.Flush(grocksdb.NewDefaultFlushOptions())
			db.CompactRange(grocksdb.Range{})
			db.Close()
		}
		fmt.Printf("RESULT kind=store_build target=%s build_s=%.1f on_disk_bytes=%d\n", name, time.Since(t0).Seconds(), dirBytes2(p))
	}
	to := time.Now()
	defer func() { lastOpen = time.Since(to) }()
	switch name {
	case "lmdb":
		env, _ := lmdb.NewEnv()
		env.SetMapSize(int64(n)*200 + 1<<30)
		must(env.Open(p, lmdb.Readonly, 0o644), "lmdb open ro")
		t := &lmdbT{env: env}
		env.View(func(tx *lmdb.Txn) error { t.dbi, _ = tx.OpenRoot(0); return nil })
		return t, func() { env.Close() }
	default:
		o := grocksdb.NewDefaultOptions()
		bo := grocksdb.NewDefaultBlockBasedTableOptions()
		bo.SetBlockCache(grocksdb.NewLRUCache(512 << 20)) // 512 MB block cache budget
		o.SetBlockBasedTableFactory(bo)
		db, err := grocksdb.OpenDbForReadOnly(o, p, false)
		must(err, "rocksdb open ro")
		return &rocksT{db: db, ro: grocksdb.NewDefaultReadOptions()}, func() { db.Close() }
	}
}
