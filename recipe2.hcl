data "version" {
  command = "echo '${path.current}' && git describe --tags --abbrev=0 || true"
}
