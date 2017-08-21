FROM buildpack-deps:jessie-scm

WORKDIR /fnproject

ADD completer /fnproject/completer

CMD ["/fnproject/completer"]
