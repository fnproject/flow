#!/bin/bash
set -e
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Checks
if docker ps >/dev/null ; then
    echo "Docker is present."
else
    echo "error: docker is not available. Please install docker 17.05 or later."
    exit 1
fi
if fn --help >/dev/null ; then
    echo "Fn is present."
else
    echo "error: fn is not available. Please install the fn tool."
    exit 1
fi

# Start or replace functions server
if [[ ! -z `docker ps | grep "functions"` ]]; then
    echo "Functions server is already up, tearing it down and starting again."
    docker stop functions
    docker rm functions
fi
docker run -d --name functions -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions:latest
# Give it time to start up
sleep 3
# Get its IP
FUNCTIONS_SERVER_IP=`docker inspect --type container -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' functions`

# Start or replace completer server
if [[ -z $IMAGE_TAG ]]; then
    IMAGE_TAG=registry.oracledx.com/skeppare/fnproject-completer:latest
fi
echo "Using completer image $IMAGE_TAG"
if [[ ! -z `docker ps | grep "completer"` ]]; then
    echo "Completer server is already up, tearing it down and starting again."
    docker stop completer
    docker rm completer
fi
docker run -d --name completer -p 8081:8081 --env API_URL=http://${FUNCTIONS_SERVER_IP}:8080/r $IMAGE_TAG
# Give it time to start up
sleep 3
# Get its IP
COMPLETER_SERVER_IP=`docker inspect --type container -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' completer`

# Create app and routes
if [[ -z $API_URL || "http://localhost:8080" == $API_URL ]]; then
    if [[ `fn apps list` == *"myapp"* ]]; then
        echo "App myapp is already there."
    else
        fn apps create myapp
        fn apps config set myapp COMPLETER_BASE_URL http://${COMPLETER_SERVER_IP}:8081
    fi
else
    echo "error: if you want to use the local environment, set API_URL to http://localhost:8080, not $API_URL."
    exit 1
fi
