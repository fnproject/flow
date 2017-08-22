FROM funcy/dind

WORKDIR /fnproject

ADD completer-docker /fnproject/completer

CMD ["/fnproject/completer"]
