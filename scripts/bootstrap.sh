#!/usr/bin/env bash
set -euf -o pipefail

cd "$(dirname "$0")/../" # to root project dir

go test -tags integration_test -coverpkg ./... -c -o ./cmd/main.test ./cmd
./cmd/main.test -test.coverprofile cmd/main.test.txt $@