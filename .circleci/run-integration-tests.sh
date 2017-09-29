#!/bin/bash
set -ex

user="fnproject"
service="completer"
tag="latest"

make docker-build >/dev/null 2>&1

(
  cd ~
  git clone git@github.com:fnproject/fdk-java.git
  cd fdk-java

  export FDK_ARTIFACT_DIR=/tmp/artifacts/fdk
  export TEST_ARTIFACT_DIR=/tmp/artifacts/tests
  export STAGING_DIR=/tmp/staging-repository


  release_version=$(git tag --sort=-version:refname | head -1)
  echo "Updating tests to use $release_version"
  echo $release_version > release.version  # test-3 uses this :-(

  # (sed syntax for portability between MacOS and gnu)
  find . -name pom.xml |
     xargs -n 1 sed -i.bak -e "s|<fnproject\\.version>.*</fnproject\\.version>|<fnproject.version>${release_version}</fnproject.version>|"
  find . -name pom.xml.bak -delete

  docker pull fnproject/fn-java-fdk-build:latest      >/dev/null 2>&1
  docker pull fnproject/fn-java-fdk-build:jdk9-latest >/dev/null 2>&1
  docker pull fnproject/fn-java-fdk:latest            >/dev/null 2>&1
  docker pull fnproject/fn-java-fdk:jdk9-latest       >/dev/null 2>&1

  ./.circleci/install-fn.sh                           >/dev/null 2>&1
  sudo apt install -y maven                           >/dev/null 2>&1

  ./integration-tests/run-local.sh
)
