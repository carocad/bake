locals {
  reports_dir = "cmd"
  main = split(" ", data.main.std_out)[0]
  binary = replace(local.main, ".go", ".bin")
  goSources = "**/**/*.go"
  vet_report = "${local.reports_dir}/vet.txt"
  test_report = "${local.reports_dir}/test.txt"
}

task "main" {
  command = "echo 'I did it :) ${data.version.std_out}, ${data.revision.std_out}, ${data.branch.std_out}'"

  depends_on = [compile, test]
}

task "compile" {
  creates = local.binary
  command  = "go build -o ${local.binary} ${local.main}"
  sources  = [local.goSources]

  depends_on = [vet]
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
