phony "main" {
  command = "echo 'I did it :)'"

  depends_on = [first]
}

target "first" {
  filename = "cmd/main.bin"
  command = "go build -o cmd/main.bin cmd/main.go"

  depends_on = [phony.second]
}

phony "second" {
  command = "pwd"
}


/*
target "first" {
  command = "how to say ${second.command}"

  depends_on = [phony.second]
}

phony "second" {
  command = "the cow says ${path.current}"
}
*/
