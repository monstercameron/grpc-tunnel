# Release Performance Notes Template

Use this template in release notes for every tag.

## Performance Summary

- Baseline snapshot file: `benchmarks/quality_baseline.json`
- Trend summary file: `bin/quality/trend.json`
- Benchmark command:

```bash
go run ./tools/runner.go quality-trend
```

## Benchmark Delta Highlights

| Benchmark | `ns/op` delta | `B/op` delta | `allocs/op` delta | Status |
| --- | ---: | ---: | ---: | --- |
| `BenchmarkGRPC_PayloadSize_1000Items` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |
| `BenchmarkGRPC_BidirectionalStream_100Messages` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |
| `BenchmarkGRPC_StreamLargeDataset_1000Items` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |
| `BenchmarkREST_PayloadSize_1000Items` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |
| `BenchmarkREST_Bidirectional_100Messages` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |
| `BenchmarkREST_LargeDataset_1000Items` | `<fill>` | `<fill>` | `<fill>` | `<improved|regressed|flat>` |

## Regressions and Mitigation

- Regressions detected: `<yes|no>`
- Affected benchmark(s): `<fill>`
- Suspected cause: `<fill>`
- Mitigation plan:
  - `<owner>`
  - `<issue link>`
  - `<target release>`

## Release Decision

- Performance gate status: `<pass|fail>`
- Release owner sign-off: `<name/date>`
- Performance owner sign-off: `<name/date>`
