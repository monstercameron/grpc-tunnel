# Performance Optimization Notes

This file records measured optimization changes and their evidence.

## 2026-03-27: Bidirectional Stream Benchmark Allocation Reduction

Change:

- In `benchmarks/comparison_test.go`, `BenchmarkGRPC_BidirectionalStream_100Messages` now reuses one immutable `proto.SyncRequest` instead of allocating a new nested request object for every streamed message.

Command used for before and after measurements:

```bash
go test ./benchmarks -bench "Benchmark(GRPC_BidirectionalStream_100Messages|REST_Bidirectional_100Messages)$" -run "^$" -benchmem -benchtime=700ms -count=3
```

Median-of-3 evidence (gRPC benchmark):

| Metric | Before | After | Delta |
| --- | ---: | ---: | ---: |
| `ns/op` | 6,173,737 | 5,454,517 | `-11.7%` |
| `B/op` | 212,221 | 195,787 | `-7.7%` |
| `allocs/op` | 5,727 | 5,424 | `-5.3%` |

Notes:

- This optimization targets benchmark client-request construction overhead.
- REST comparison benchmark remained in the same overall range during the same runs.
