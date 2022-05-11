locals {
  hello = "main"
}

phony "main" {
  command = "echo 'I did it :)'"

  depends_on = [first]
}

target "first" {
  filename = "cmd/main.bin"
  command  = "go build -o cmd/${local.hello}.${phony.data.std_out}.bin cmd/main.go"
  sources  = ["cmd/main.go"]

  depends_on = [phony.second]
}

phony "second" {
  // command = "echo 'hello ${phony.third.command}'"
  command = "echo 'hello world'"
}

phony "data" {
  command = "git rev-parse --short HEAD"
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
