phony "main" {
  command = "where am i? in ${path.current} ?"

  depends_on = [target.first]
}

target "first" {
  command = "how to say ${pid}"

  depends_on = [phony.main]
}
