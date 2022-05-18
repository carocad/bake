locals {
  main = "cmd/main.go"
  binary = replace(local.main, ".go", ".bin")
}

task "main" {
  command = "echo 'I did it :) ${data.version.std_out}'"

  depends_on = [compile]
}

task "compile" {
  creates = local.binary
  command  = "go build -o ${local.binary} ${local.main}"
  sources  = [local.main]

  depends_on = [vet, test]
}

task "vet" {
  command = "go vet ./..."
}

task "test" {
  command = "go test ./..."
}
