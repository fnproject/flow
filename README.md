# Completer

![logo: you complete me!](logo.jpg) 

The completer is a service that implements long-running computations based on fn invocations allowing reliable promise-like continuations of function code. 

Functions create `threads` (graphs of completion stages) dynamically using an API from within a function runtime - the completer then invokes these stages, triggering any depenant stages as the computation progresses which in turn can append new stages to the graph.

Dependent stages may be independent function calls (i.e. to compse multiple functions together into a workflow) or can be closures of the original function - in the latter case the completer invokes the function with a special input that allows a wrapper within the function to dispatch the closure correctly (vs calling the original main part of the function). 

The completer is language agnostic - to add support for your language check out the [API docs](docs/API.md). 

In languages such as Java where closures (labmdas) can be serialized this allows very simple fluent programs to be produced to control complex processes based on functions. 



## Running the completer 
*TBD* 


## Contributing 

*TBD* 

## License 
*TBD* 



