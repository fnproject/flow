# Fn Flow Service

[![CircleCI](https://circleci.com/gh/fnproject/flow.svg?style=svg)](https://circleci.com/gh/fnproject/flow)

![logo: you complete me!](logo.jpg) 

To find out how to use Fn Flow in Java read the [user guide](https://github.com/fnproject/fdk-java/blob/master/docs/FnFlowsUserGuide.md). 

The Flow Service is a service that implements long-running computations  based on fn invocations allowing reliable promise-like continuations of function code. 

Functions create *flows* (graphs of *completion stages*) dynamically using an API from within a function runtime - the Flow Service then invokes these stages, triggering any dependant stages as the computation progresses which in turn can append new stages to the graph.

Dependent stages may be independent function calls (i.e. to compose multiple functions together into a workflow) or can be closures of the original function - in the latter case the Flow Service invokes the function with a special input that allows a wrapper within the function to dispatch the closure correctly (vs calling the original main part of the function).

The Flow Service is language agnostic - to add support for your language check out the [API docs](docs/API.md). 

In languages such as Java where closures (labmdas) can be serialized this allows very simple fluent programs to be produced to control complex processes based on functions. 



## Running the Flow Service

Make sure the functions server is running 
```bash 
fn start                                                                                                                                                 master ✭ ◼
mount: permission denied (are you root?)
Could not mount /sys/kernel/security.
AppArmor detection and --privileged mode might break.
mount: permission denied (are you root?)
time="2017-09-16T22:04:49Z" level=info msg="datastore dialed" datastore=sqlite3 max_idle_connections=256
time="2017-09-16T22:04:49Z" level=info msg="no docker auths from config files found (this is fine)" error="open /root/.dockercfg: no such file or directory"
time="2017-09-16T22:04:49Z" level=info msg="available memory" ram=1590210560

      ______
     / ____/___
    / /_  / __ \
   / __/ / / / /
  /_/   /_/ /_/

time="2017-09-16T22:04:49Z" level=info msg="Serving Functions API on address `:8080`"
```

set $DOCKER_LOCALHOST to the loopback interface for your docker. 

On Mac: 
```bash
export DOCKER_LOCALHOST=docker.for.mac.localhost
```

Otherwise run

```bash
$ export DOCKER_LOCALHOST=$(docker inspect --type container -f '{{.NetworkSettings.Gateway}}' functions)
```

Then run the flow service  : 
```
docker run --rm  -d -p 8081:8081 \
           -e API_URL="http://$DOCKER_LOCALHOST:8080/r" \
           -e no_proxy=$DOCKER_LOCALHOST \
           --name flow-service \
           fnproject/flow:latest
```


Note if you have an HTTP proxy configured in docker you should add the docker loopback interface (and docker.for.mac.localhost) to your `no_proxy` settings.  

Configure via the environment 

| Env | Default | Usage |
| --- | --- | --- |
| API_URL | http://localhost:8080 | sets the FN API endpoint for outbound invocations | 
| DB_URL | sqlite3://./data/flow.db | DB url, also use "inmem:/" for in memory storage |
| LISTEN |  :8081 | listen host/port (overrides PORT)  |

# Get help

   * Come over and chat to us on the [fnproject Slack](https://join.slack.com/t/fnproject/shared_invite/enQtMjIwNzc5MTE4ODg3LTdlYjE2YzU1MjAxODNhNGUzOGNhMmU2OTNhZmEwOTcxZDQxNGJiZmFiMzNiMTk0NjU2NTIxZGEyNjI0YmY4NTA).
   * Raise an issue in [our github](https://github.com/fnproject/flow/).


## Contributing 

Please see [CONTRIBUTING.md](CONTRIBUTING.md).
