module github.com/runners-io/tflint-ruleset-tofucompat

// RUN-2409 (2026-10-31): x/net v0.60.0, grpc v1.83.2 and x/text v0.43.0 fix the
// eight CALLED govulncheck findings this module carried (GO-2026-6617, 6612,
// 6611, 6603 in x/net; 6443, 6348, 6061 in grpc; 5970 in x/text). All three are
// over two weeks old (2026-10-08, 2026-08-25, 2026-10-09), so no exception to
// the two-week rule applies. x/net v0.62.0 (GO-2026-6860 and 6866) was four days
// old and those two are imported, not called, so it was deliberately not taken.
// 2026-11-07: v0.62.0 is taken (with x/sys v0.49.0, which it requires) because
// CVE-2026-78661 (HIGH, fixed in 0.62.0) turned the stack vulnerability scan red
// on every pull request through this go.mod; both engines already carry v0.62.0.

go 1.26.11

require (
	github.com/hashicorp/go-version v1.9.0
	github.com/hashicorp/hcl/v2 v2.24.0
	github.com/terraform-linters/tflint-plugin-sdk v0.25.0
	github.com/zclconf/go-cty v1.18.1
)

require (
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-plugin v1.8.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
