locals {
  main = "cmd/main.go"
  binary = replace(local.main, ".go", ".${data.revision.std_out}.bin")
}

task "main" {
  command = "echo 'I did it :) ${data.version.std_out}'"

  depends_on = [first]
}

task "first" {
  filename = local.binary
  command  = "go build -o ${local.binary} ${local.main}"
  sources  = [local.main]

  depends_on = [second]
}

task "second" {
  command = "echo 'hello ${path.module}, I am in ${data.branch.std_out}'"
}

data "revision" {
  command = "git rev-parse --short HEAD"
}

data "version" {
  command = "git describe --tags --abbrev=0"
}

data "branch" {
  command = "git rev-parse --abbrev-ref HEAD | tr A-Z/ a-z-"
}

// todo: enable for_each
/*
phony "data" {
  for_each = {
    version  = "git describe --tags --abbrev=0"
    revision = "git rev-parse --short HEAD"
    branch   = "git rev-parse --abbrev-ref HEAD | tr A-Z/ a-z-"
  }

  command = each.value
}
*/
