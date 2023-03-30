#!/bin/bash

set -e

GV="$1"

rm -rf ./pkg/client
./hack/generate_group.sh "client,lister,informer" github.com/edgewize-io/edgewize/pkg/client github.com/edgewize-io/edgewize/pkg/apis "${GV}" --output-base=./  -h "$PWD/hack/boilerplate.go.txt"
mv github.com/edgewize-io/edgewize/pkg/client ./pkg/
rm -rf ./github.com
