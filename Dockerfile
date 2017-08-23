FROM funcy/go
WORKDIR /fnproject
ADD completer-docker /fnproject/completer
CMD ["/fnproject/completer"]
