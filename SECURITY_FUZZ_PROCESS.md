# Security Fuzz Process

Use this runbook when doing deep security fuzz verification before release or after security-sensitive changes.

## 1. Preconditions

- Run from repo root: `third_party/GoGRPCBridge`
- Ensure no stale fuzz/test processes are running.
- Ensure your working tree changes are intentional before starting.

Windows process check:

```powershell
Get-CimInstance Win32_Process -Filter "Name='go.exe'" |
  Where-Object { $_.CommandLine -match 'fuzz|go test ./...|direct-bridge-e2e' } |
  Select-Object ProcessId,CommandLine
```

## 2. List Available Fuzz Targets

```powershell
go test ./pkg/bridge -list=Fuzz
go test ./pkg/grpctunnel -list=Fuzz
```

Important:

- Always target exactly one fuzzer with an anchored regex (`^FuzzName$`).
- Always use `-run=^$` during fuzzing to skip normal tests in the same package.

## 3. Baseline Package Validation

```powershell
go test -mod=mod ./pkg/bridge ./pkg/grpctunnel -count=1
```

## 4. Deep Security Fuzz Matrix

Run each command independently and stop on first failure.

Security parser/config fuzzers (long):

```powershell
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzParseBridgeTargetURL$ -fuzztime=120s
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzGetHandlerConfigError$ -fuzztime=120s
go test -mod=mod ./pkg/grpctunnel -run=^$ -fuzz=^FuzzParseTunnelTargetURL$ -fuzztime=120s
go test -mod=mod ./pkg/grpctunnel -run=^$ -fuzz=^FuzzGetTunnelConfigError$ -fuzztime=120s
```

Protocol/message fuzzers (medium):

```powershell
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzWebSocketConnWrite$ -fuzztime=60s
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzWebSocketConnRead$ -fuzztime=60s
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzBinaryMessage$ -fuzztime=60s
go test -mod=mod ./pkg/bridge -run=^$ -fuzz=^FuzzMessageSizes$ -fuzztime=60s
```

## 5. Failure Handling

If any fuzz target fails:

1. Save the failing output and minimized crashing input from the fuzz output.
2. Reproduce with:

```powershell
go test -mod=mod <package> -run=^$ -fuzz=^<FuzzName>$ -fuzztime=1s
```

3. Implement the smallest root-cause fix.
4. Add/expand deterministic tests for the bug.
5. Re-run the failed fuzzer, then re-run the full matrix.

## 6. Post-Run Hygiene

Re-check for stuck processes:

```powershell
Get-CimInstance Win32_Process -Filter "Name='go.exe'" |
  Where-Object { $_.CommandLine -match 'fuzz|go test ./...|direct-bridge-e2e' } |
  Select-Object ProcessId,CommandLine
```

Re-run package verification:

```powershell
go test -mod=mod ./pkg/bridge ./pkg/grpctunnel -count=1
```

Review dependency drift introduced during fuzz/test runs:

```powershell
git diff -- go.mod go.sum
```

## 7. Recommended Cadence

- Every security-sensitive PR: quick fuzz (`go run ./tools/runner.go fuzz-quick`)
- Pre-release security sign-off: full deep matrix in this document
- Incident response hardening: full deep matrix plus longer soak (`300s+` per target)
