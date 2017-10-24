FROM funcy/go
WORKDIR /fnproject
ADD flow-service-docker /fnproject/flow-service
CMD ["/fnproject/flow-service"]
