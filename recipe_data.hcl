
data "git" {
  for_each = {
    tag       = "git describe --tags --abbrev=0"
    revision  = "git rev-parse --short HEAD"
    branch    = "git rev-parse --abbrev-ref HEAD | tr A-Z/ a-z-"
  }
  
  command = each.value
}

data "main" {
  command = "find . -name main.go"
}

