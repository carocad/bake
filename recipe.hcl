phony "main" {
  command = "echo 'I did it :)'"

  depends_on = [first]
}

target "first" {
  command = "ls -l ."

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
