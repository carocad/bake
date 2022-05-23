locals {
  reports_dir = "cmd"
  main = split(" ", data.main.std_out)[0]
  binary = replace(local.main, ".go", ".bin")
  goSources = "**/**/*.go"
  vet_report = "${local.reports_dir}/vet.txt"
  test_report = "${local.reports_dir}/test.txt"
  version_filename = "internal/version.go"
}

task "main" {
  command = "echo 'I did it :)'"

  depends_on = [compile, test]
}

task "compile" {
  creates = local.binary
  command  = "go build -o ${local.binary} ${local.main}"
  sources  = [local.goSources]

  depends_on = [vet, version]
}

task "vet" {
  command = "go vet ./... | tee ${local.vet_report}"
  creates = local.vet_report
  sources  = [local.goSources]
}

task "test" {
  command = "go test ./... | tee ${local.test_report}"
  creates = local.test_report
  sources  = [local.goSources]
}

task version {
  creates = local.version_filename
  sources = [local.goSources]
  command = <<COMMAND
  cat << EOF > ${local.version_filename}
// automatically generated by bake
package internal

const (
  Version  = "${data.version.std_out}"
  Revision = "${data.revision.std_out}"
  Branch = "${data.branch.std_out}"
)
EOF
COMMAND
}
