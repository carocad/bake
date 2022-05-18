data "version" {
  command = "echo '${path.current}' && git describe --tags --abbrev=0 || true"
}

data "revision" {
  command = "git rev-parse --short HEAD"
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

