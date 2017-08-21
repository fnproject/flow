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
if [[ ! -z `docker ps | grep "local-functions"` ]]; then
    echo "Functions server is already up, tearing it down and starting again."
    docker stop local-functions
    docker rm local-functions
fi
docker run -d --name local-functions -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock funcy/functions:latest
# Give it time to start up
sleep 3
# Get its IP
FUNCTIONS_SERVER_IP=`docker inspect --type container -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' local-functions`

# Start or replace completer server
IMAGE_FULL=$1
if [[ -z "$IMAGE_FULL" ]]; then
    echo "error: No completer docker image provided as an argument."
    exit 1
fi
echo "Using completer image $IMAGE_FULL"
if [[ ! -z `docker ps | grep "local-completer"` ]]; then
    echo "Completer server is already up, tearing it down and starting again."
    docker stop local-completer
    docker rm local-completer
fi
docker run -d --name local-completer -p 8081:8081 --env API_URL=http://${FUNCTIONS_SERVER_IP}:8080/r $IMAGE_FULL
# Give it time to start up
sleep 3
# Get its IP
COMPLETER_SERVER_IP=`docker inspect --type container -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' local-completer`

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
