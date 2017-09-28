#!/bin/bash
set -ex

user="fnproject"
service="completer"
tag="latest"

make docker-build

(
  cd ~
  git clone git@github.com:fnproject/fdk-java.git
  cd fdk-java.git

  export FDK_ARTIFACT_DIR=/tmp/artifacts/fdk
  export TEST_ARTIFACT_DIR=/tmp/artifacts/tests
  export STAGING_DIR=/tmp/staging-repository

  ./.circleci/update-versions.sh

  docker pull fnproject/fn-java-fdk-build:jdk9-latest
  docker pull fnproject/fn-java-fdk:jdk9-latest

  ./.circleci/install-fn.sh

  ./integration-tests/run-local.sh
)
