phony "main" {
  command = "where am i? in ${path.current} ?"

  depends_on = [first]
}

target "first" {
  command = "how to say ${second.command}"

  depends_on = [phony.second]
}

phony "second" {
  command = "the cow says ${path.current}"
}
