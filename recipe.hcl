locals {
  reports_dir = "cmd"
  main = split(" ", data.main.std_out)[0]
  go_sources = "**/*.go"
  vet_report = "${local.reports_dir}/vet.txt"
  test_report = "${local.reports_dir}/test.txt"
  version_filename = "internal/info/version.go"
  go_archs = [
    "amd64",
    "arm",
    "arm64"
  ]
  binaries = {
    for arch in local.go_archs:
      arch => replace(local.main, ".go", ".${arch}.bin")
  }
}

task "main" {
  description = "the default task to run"

  // WARNING: never run 'main' from cmd/main.test as that would create
  // an infinite loop
  depends_on = [vet, compile, unit_test]
}

task "compile" {
  for_each = local.binaries

  creates = each.value
  command  = "GOARCH=${each.key} go build -o ${each.value} ${local.main}"
  sources  = [local.go_sources]

  depends_on = [libraries]
}

task "vet" {
  command = "go vet -v ./... | tee ${local.vet_report}"
  creates = local.vet_report
  sources  = [local.go_sources]
}

task "unit_test" {
  command = "go test -timeout 10s ./... | tee ${local.test_report}"
  creates = local.test_report
  sources  = [local.go_sources]

  depends_on = [compile]
}

task "version" {
  description = "generate a version file from git information"
  creates = local.version_filename
  command = <<COMMAND
  cat << EOF > ${local.version_filename}
// automatically generated by bake
package info

import "time"

const (
  Version  = "${data.git["tag"].std_out}"
  Revision = "${data.git["revision"].std_out}"
  Branch = "${data.git["branch"].std_out}"
)

var CompiledAt = time.Now()
EOF
COMMAND
}

task "libraries" {
  creates = "go.sum"
  sources = ["go.mod"]
  command = <<COMMAND
  go mod download
  go mod tidy
  COMMAND
}
