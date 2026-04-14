package cairn

import (
	"path/filepath"
	"testing"
)

// Bench payload: 128 bytes — representative of a small structured event.
// Not so small that we're measuring pure overhead, not so large that we're
// measuring raw I/O throughput.
var benchPayload = make([]byte, 128)

func benchStore(b *testing.B) *Store {
	b.Helper()
	path := filepath.Join(b.TempDir(), "bench.db")
	s, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

// BenchmarkAppendSingle — one event per op.
// Reports events/sec as ops/sec (1 event == 1 op).
// This is the fsync-bound path: every Append commits, so throughput is
// roughly 1/fsync_latency on the target disk.
func BenchmarkAppendSingle(b *testing.B) {
	s := benchStore(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Append("bench.topic", benchPayload); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "events/sec")
}

// BenchmarkAppendBatch100 — 100 events per op, committed in one transaction.
// Reports events/sec (ops/sec * 100). One fsync per batch, so this is much
// faster than single Append on spinning disks / SD cards.
func BenchmarkAppendBatch100(b *testing.B) {
	s := benchStore(b)
	batch := make([]BatchEvent, 100)
	for i := range batch {
		batch[i] = BatchEvent{Topic: "bench.topic", Payload: benchPayload}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.AppendBatch(batch); err != nil {
			b.Fatal(err)
		}
	}
	events := float64(b.N) * 100
	b.ReportMetric(events/b.Elapsed().Seconds(), "events/sec")
}

// BenchmarkQuery100k — pre-seeds 100k events in a single topic, then runs
// Query over the full range per op. Reports events scanned/sec.
// Dominated by row scanning + Go slice allocation, not fsync.
func BenchmarkQuery100k(b *testing.B) {
	s := benchStore(b)

	// Seed via large batches — single Append on SD would take minutes.
	const totalEvents = 100_000
	const batchSize = 1000
	batch := make([]BatchEvent, batchSize)
	for i := range batch {
		batch[i] = BatchEvent{Topic: "bench.topic", Payload: benchPayload}
	}
	for seeded := 0; seeded < totalEvents; seeded += batchSize {
		if _, err := s.AppendBatch(batch); err != nil {
			b.Fatalf("seed: %v", err)
		}
	}

	b.ResetTimer()
	var totalScanned int64
	for i := 0; i < b.N; i++ {
		events, err := s.Query("bench.topic", 0, 1<<62)
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != totalEvents {
			b.Fatalf("expected %d events, got %d", totalEvents, len(events))
		}
		totalScanned += int64(len(events))
	}
	b.ReportMetric(float64(totalScanned)/b.Elapsed().Seconds(), "events/sec")
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "queries/sec")
}
