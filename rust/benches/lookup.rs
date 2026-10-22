//! Attos Rust SDK lookup latency vs a std HashMap, on the same keys and values as the Go suite.
//!
//!     cargo bench                       # default 1M keys, reads ../.data/n1000000/data.attos
//!     ATTOS_FILE=/path/data.attos ATTOS_KEYS=1000000 cargo bench
//!
//! The .attos file is built by the Go side (`make data`), so both languages read identical bytes.
//! Std-only harness: one timed call at a time, timer floor reported and included.

use attos_client::AttosMap;
use std::collections::HashMap;
use std::hint::black_box;
use std::time::Instant;

fn mix(mut x: u64) -> u64 {
    x ^= x >> 32;
    x = x.wrapping_mul(0xD6E8FEB86659FD93);
    x ^= x >> 32;
    x = x.wrapping_mul(0xD6E8FEB86659FD93);
    x ^= x >> 32;
    x
}
fn key(i: u64) -> String { format!("user:{:016x}", mix(i + 1)) }
fn value(i: u64) -> String { format!("{:064x}", mix(i * 3 + 7)) }

fn queries(keys: u64, n: usize, hot: bool) -> Vec<String> {
    let pool = if hot { keys.min(5000) } else { keys };
    let mut r: u64 = 12345;
    (0..n).map(|_| { r = r.wrapping_add(0x9E3779B97F4A7C15); key(mix(r) % pool) }).collect()
}

fn report(name: &str, mode: &str, floor: u64, mut lat: Vec<u32>, total_ns: u128) {
    lat.sort_unstable();
    let p = |f: f64| lat[((lat.len() - 1) as f64 * f) as usize];
    println!("| {name} | {mode} | {} | {} | {} | {} | {:.1} |", p(0.5), p(0.99), p(0.999), floor,
             total_ns as f64 / lat.len() as f64);
}

fn timer_floor() -> u64 {
    let mut t = 0u128;
    for _ in 0..200_000 { let a = Instant::now(); t += a.elapsed().as_nanos(); }
    (t / 200_000) as u64
}

fn main() {
    let keys: u64 = std::env::var("ATTOS_KEYS").ok().and_then(|v| v.parse().ok()).unwrap_or(1_000_000);
    let path = std::env::var("ATTOS_FILE").unwrap_or_else(|_| format!("../.data/n{keys}/data.attos"));
    let map = AttosMap::open(&path).unwrap_or_else(|e| panic!("open {path}: {e:?} (run `make data` first)"));
    for i in (0..keys).step_by((keys / 1000).max(1) as usize) {
        assert_eq!(map.get_string(&key(i)).unwrap(), value(i), "wrong value for key {i}");
    }
    let mut hm: HashMap<String, String> = HashMap::with_capacity(keys as usize);
    for i in 0..keys { hm.insert(key(i), value(i)); }

    let floor = timer_floor();
    println!("| system | mode | p50 ns | p99 ns | p99.9 ns | timer floor ns | mean ns (loop) |");
    println!("|---|---|---|---|---|---|---|");
    for (mode, hot) in [("hot", true), ("cold", false)] {
        let qs = queries(keys, 1_000_000, hot);
        for q in &qs[..200_000] { black_box(map.get_string(q).map(|v| v.len()).unwrap_or(0)); }
        let mut lat = Vec::with_capacity(qs.len());
        let t0 = Instant::now();
        for q in &qs {
            let a = Instant::now();
            black_box(map.get_string(q).map(|v| v.len()).unwrap_or(0));
            lat.push(a.elapsed().as_nanos() as u32);
        }
        report("attos (rust)", mode, floor, lat, t0.elapsed().as_nanos());

        for q in &qs[..200_000] { black_box(hm.get(q).map(|v| v.len())); }
        let mut lat = Vec::with_capacity(qs.len());
        let t0 = Instant::now();
        for q in &qs {
            let a = Instant::now();
            black_box(hm.get(q).map(|v| v.len()));
            lat.push(a.elapsed().as_nanos() as u32);
        }
        report("std HashMap", mode, floor, lat, t0.elapsed().as_nanos());
    }
}
