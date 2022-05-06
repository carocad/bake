phony "main" {
  command = "where am i? in ${path.current} ?"

  depends_on = [first, phony.second]
}

target "first" {
  command = "how to say ${pid}"
}

phony "second" {
  command = "the cow says ${path.current}"
}
