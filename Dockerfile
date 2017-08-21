FROM buildpack-deps:jessie-scm

WORKDIR /fnproject

ADD completer-docker /fnproject/completer

CMD ["/fnproject/completer"]
