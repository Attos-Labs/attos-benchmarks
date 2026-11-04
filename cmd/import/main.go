// import moves your existing key/value data into Attos Cloud, the one-step "switch" path.
//
//	export ATTOS_API_KEY=at_live_...
//	go run ./cmd/import -redis localhost:6379 [-match 'user:*']   # every string key in Redis
//	go run ./cmd/import -file data.csv                              # key,value per line
//	go run ./cmd/import -file data.ndjson                           # {"key":"..","value":".."} per line
//
// It prints the dataset id and the three lines of Go that replace your Redis/map lookups. Other
// stores (Postgres, DynamoDB, ...): export to CSV or NDJSON and use -file.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/Attos-Labs/attos-benchmarks/internal/cloud"
)

func main() {
	rd := flag.String("redis", "", "Redis host:port to read every string key from")
	match := flag.String("match", "*", "Redis SCAN pattern")
	file := flag.String("file", "", "CSV (key,value) or NDJSON file")
	name := flag.String("name", "imported", "dataset name")
	flag.Parse()
	c := cloud.FromEnv()
	if c == nil {
		fatal("ATTOS_API_KEY is not set (dashboard: Settings > API Keys)")
	}
	pr, pw := io.Pipe()
	var count int
	go func() {
		w := bufio.NewWriterSize(pw, 1<<20)
		emit := func(k, v string) {
			b, _ := json.Marshal(map[string]string{"key": k, "value": v})
			w.Write(b)
			w.WriteByte('\n')
			count++
		}
		var err error
		switch {
		case *rd != "":
			err = fromRedis(*rd, *match, emit)
		case *file != "":
			err = fromFile(*file, w, emit, &count)
		default:
			err = fmt.Errorf("pass -redis host:port or -file path")
		}
		if err == nil {
			err = w.Flush()
		}
		pw.CloseWithError(err)
	}()
	id, err := c.Upload(context.Background(), *name, pr)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("uploaded %d keys, dataset id %s\n\n", count, id)
	fmt.Printf("Use it:\n  s, _ := attos.NewSynchronizer(ctx, %q, attos.WithBaseURL(%q), attos.WithAPIKey(os.Getenv(\"ATTOS_API_KEY\")))\n  v, err := s.Get([]byte(key)) // replaces your GET\n", id, c.BaseURL)
}

func fromRedis(addr, match string, emit func(k, v string)) error {
	ctx := context.Background()
	cl := redis.NewClient(&redis.Options{Addr: addr})
	defer cl.Close()
	var cur uint64
	for {
		keys, next, err := cl.Scan(ctx, cur, match, 1000).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			vals, err := cl.MGet(ctx, keys...).Result()
			if err != nil {
				return err
			}
			for i, v := range vals {
				if s, ok := v.(string); ok { // non-string types (hash, list, ...) are skipped
					emit(keys[i], s)
				}
			}
		}
		if cur = next; cur == 0 {
			return nil
		}
	}
}

func fromFile(path string, w *bufio.Writer, emit func(k, v string), count *int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case line == "":
		case strings.HasPrefix(line, "{"):
			w.WriteString(line)
			w.WriteByte('\n')
			*count++
		default:
			k, v, ok := strings.Cut(line, ",")
			if !ok {
				return fmt.Errorf("line without a comma: %q", line)
			}
			emit(k, v)
		}
	}
	return sc.Err()
}

func fatal(m string) { fmt.Fprintln(os.Stderr, "import:", m); os.Exit(1) }
