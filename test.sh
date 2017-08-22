#!/bin/bash
set -ex

if [[ -z "$TEST_RESULTS" ]]; then
  TEST_RESULTS=/tmp/test-results
fi

mkdir -p $TEST_RESULTS
go get -u -v github.com/jstemmer/go-junit-report
make test | tee ${TEST_RESULTS}/go-test.out
$GOPATH/bin/go-junit-report <${TEST_RESULTS}/go-test.out > ${TEST_RESULTS}/go-test-report.xml
