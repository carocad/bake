phony "main" {
  command = "where am i? in ${path.current} ?"

  depends_on = [first]
}

target "first" {
  command = "how to say ${pid}"

  depends_on = [phony.second]
}

phony "second" {
  command = "the cow says ${path.current}"
}
