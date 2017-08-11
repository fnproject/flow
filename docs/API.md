# Threads API - DRAFT 

This document defines how to interact with threads via the completer, and how the completer invokes threads via fn.

There are two API call contracts: 

The [Client API](#Completer-Client-API) between a client function and the completer: Functions make calls to the completer to create threads and append completion stages , the completer stores these and invokes the stages when they are triggered.   

The [Invoke API](#Completer-Invoke-API) between the completer and a the fn service: The completer invokes back into fn via it's public API to trigger stages of the computation. The function code inteprets incoming requests and dispatches code to the appropriate implementations before returning a result back to the completer. 

 

## Key Concepts: 

A *thread* is a single graph of computation associated with a single function, created from an invocation of that function. Threads are identified by a `ThreadID`. 

A *completion stage* is a single node in the thread graph that is created by user code either from the original function invocation or from a subsequent *continuation invocation* of that graph. Completion stages are identified by a `StageID` which is unique within the respective graph.  

Completion stages consist of the following : 
  *  A *stage type* that describes what the stage should do, how arguments should be passed into the stage and how results of the stage should be handled see [Stage Types](#stage-types) for a list of supported stage types
  *  An *closure* that describes what code to run within the original function container, when the stage is activated -  Not all stage types require a closure (e.g. delay). The closure is interpreted by the calling function and may be of any type - in java this may for instance be a serialized lambda expression and its captured arguments. 
  *  Zero or more *stage dependencies* that trigger the stage, the stage type determines under which circumstances the dependencies cause the stage to trigger.
  * A *stage value* : this corresponds to the (successful or failed) value associated with stage once it has completed - this value is used to trigger downstream stages.  
  



## Datum Types

The protocol assigns semantic meaning to byte streams which must fall into one of the following types: 

```
FnProject-DatumType: blob
FnProject-DatumType: empty
FnProject-DatumType: error
FnProject-DatumType: stageref
FnProject-DatumType: httpreq
FnProject-DatumType: httpresp
```

When using _blob_ types, their media type must be defined by including the `Content-Type` header. For example, to transmit a serialized Java value the following headers must be included in the request/response:

```
Content-Type: application/java-serialized-object
FnProject-DatumType: blob
```
The _error_ type is always of type _text/plain_ and uses the body for its error message.

The _empty_ datum type is used to signify a null/void type and has no body. 

The _stageref_ type is used to return the stage ID of a composed stage reference (see below) and also has no body.

Finally, the _httpreq_  and _httpresp__ types encapsulate an HTTP request or response (such as a function call or response). They include the body of the HTTP message and  any headers present in the original response by prefixing them with `FnProject-Header-`. HTTP requests must specify an HTTP Method  for the requested call via `FnProject-Method` Additionally, HTTP responses must include the header `FnProject-ResultCode` with the HTTP status code. The `Content-type` header is preserved as per a `blob` datum. 

## Cloud Threads Application Lifecycle

The following sections define the request/response protocol for the lifetime of a Cloud Threads application.

### Runtime Creates a Cloud Thread (Function->Completer)

The function creates a new thread by POST am empty request to the `/graph` endpoint with a function ID  of the current function.  


```
POST /graph?functionId=${fn_id} HTTP/1.1
Content-length: 0


```

The completer returns with an empty response containing the new thread ID in the FnProject-ThreadID header: 

```
HTTP/1.1 200 OK 
FnProject-ThreadID: thread-abcd-12344
``` 

### Runtime creates a stage in the graph

HTTP POST requests to the Completer REST API should include a `Content-Type` header to designate the media type of the body. In the case of a Java runtime, requests that POST a Java lambda expression or a Java serialized instance should set this header to `application/java-serialized-object`.

For example, the runtime POSTs a *closure*  to one of the stage operations (see API below): 

```
POST /graph/thread-abcd-12344/supply HTTP/1.1
FnProject-DatumType: blob
Content-type: application/java-serialized-object
Content-length: 100 

...serialized lambda...
```

The completer returns a new `StageID` in the `FnProject-StageID` header. 
```
HTTP/1.1 200 OK
FnProject-StageID: 1
```

### Runtime requests a function invocation via the completer 

Invoke Function stages take an *httpreq* datum which encapsulates the request's route (path in functions), as well as the HTTP headers, method and body. The completer will then use this datum to create and send a request to fn upon successfully triggering this stage.

```
POST /graph/thread-abcd-12344/invokeFunction?functionId=/fnapp/somefunction/path
FnProject-DatumType: httpreq
FnProject-Method: POST
FnProject-Header-CUSTOM-HEADER: user-12334
Content-Type: application/json

{
   "transaction-id" : "12345678"
}
```

Again the completer returns a new `StageID` in the `FnProject-StageID` header. 
```
HTTP/1.1 200 OK
FnProject-StageID: 3
```

### Completer Invokes a Continuation

A continuation request inside a function must include a serialized closure along with one or more arguments. Some of these arguments may be empty/null. HTTP multipart is used to frame the different elements of the request.

For example:

```
POST /r/app/path HTTP/1.1
Content-Type: multipart/form-data; boundary="01ead4a5-7a67-4703-ad02-589886e00923"
FnProject-ThreadID: thread-abcd-12344
FnProject-StageID: 2
Content-Length: 707419

--01ead4a5-7a67-4703-ad02-589886e00923
Content-Type: application/java-serialized-object
Content-Disposition: form-data; name=closure
FnProject-DatumType: blob

...serialized closure...
--01ead4a5-7a67-4703-ad02-589886e00923
Content-Type: application/java-serialized-object
Content-Disposition: form-data; name=arg_0
FnProject-DatumType: blob

...serialized arg 0...
--01ead4a5-7a67-4703-ad02-589886e00923
Content-Disposition: form-data; name=arg_1
FnProject-DatumType: empty

--01ead4a5-7a67-4703-ad02-589886e00923
```

#### Successful Result

If the execution of the closure succeeds, the runtime must include the header `FnProject-ResultStatus: success`.

```
Content-Type: application/java-serialized-object
FnProject-DatumType: blob
FnProject-ResultStatus: success

...serialized result...
```

##### Empty/Void Result
Closures may succeed and return an empty (void) response (e.g. the result of _thenAccept_), in which case the empty datum type should be used and the body should be empty:

```
FnProject-DatumType: empty
FnProject-ResultStatus: success
```

##### Completion Stage ID Result
Successful execution of the continuation associated with a _thenCompose_ stage returns a datum of type `stageref` and must include the header `FnProject-StageID` containing the inner completion stage's ID. The body of the response should be empty.
For example:
```
FnProject-DatumType: stageref
FnProject-ResultStatus: success
FnProject-StageID: 2
```


#### Failed Result

Similary, if executing the closure fails inside the runtime, it responds by including the header `FnProject-ResultStatus: failure`. For a Java runtime, the body will contain the serialized bytes of the Java exception thrown by the user code inside the closure:

```
Content-Type: application/java-serialized-object
FnProject-DatumType: blob
FnProject-ResultStatus: failure

...serialized exception...
```

#### Platform Error

Completion stages can also fail due to errors thrown outside of the user's code. For example, the completer may time out while waiting for a response to a continuation request. 
In such cases, the completion stage will fail, but there will be no exception or stacktrace associated with the failure.
Retrieving the value of a failed stage due to a platform error will return the following headers, and will include a message describing the error in the body of the response.

```
Content-Type: text/plain
FnProject-DatumType: error
FnProject-ErrorType: stage-timeout

The continuation request timed out.
```

In the Java runtime, a platform error will be internally converted to a `CloudCompletionStageTimeoutException` and contain the original error message string.

`FnProject-ErrorType` indicates the type of the error, currently supported values are: 

| Error Type | Meaning |
| ---         | ----   |
| stage-timeout | a completion stage function timed out - the stage may or may not have completed normally'|
| stage-invoke-failed | a completion stage invocation failed - the stage may or may not have been invoked or and that invocation may or may not have completed |
| function-timeout | A function call timed out | 


Recipients should accept unknown values for this header.

### Completer Invokes a Function

Completion stages that invoke an external function via a call to _invokeFunction_ may also complete successfully or with an error. They may also fail to complete due to a platform error.

#### Successful Response

When a function call completes successfully, the completer will persist the HTTP status code and headers along with the body of the response. This HTTP response data will be included in the appropriate argument part of a multipart continuation request for any dependent completion stages, as well as when retrieving the value of this stage. For example:

```
FnProject-DatumType: httpresp
FnProject-ResultStatus: success
FnProject-ResultCode: 200
FnProject-Header-CUSTOM-HEADER: customValue
Content-Type: application/json

{
   "message" : "Successfully created user"
}
```

In the Java runtime, this stage's value will be transparently coerced to the `FnResult` type. FnResult is a wrapped HTTP response including:
* Status Code
* Headers (without the `FnProject-Header-` prefix)
* Body

Runtimes may also optionally provide coercions of function results to the appropriate runtime language types. In the case of Java, the standard Java functions coercions apply and can be leveraged to coerce a stage result to a specific Java type.

#### Failed Response

As with successful invocations, the completer will store body, status and headers for function invocations where the function indicates that it has terminated unsuccessfully. In this case the stage's status will be set to _failure_ and the body will echo the output from the function:

```
FnProject-DatumType: httpresp
FnProject-ResultStatus: failure
FnProject-ResultCode: 500
FnProject-Header-CUSTOM-HEADER: customValue
Content-Type: application/json

{
   "reason" : "INVALID_USER_AUTHENTICATION",
   "message" : "Failed to authenticate principal, password was invalid"
}
```

In the Java runtime, the stage's value will be coerced to a `FunctionInvocationException`, which like FnResult wraps the body, headers and status code of the original response.


#### Platform Error

Errors occurring outside the function will cause the stage to fail and contain an error message describing the failure. These failures are handled identically to platform errors when invoking continuations.

### External Completion

External completion stages are created by a call to `externalCompletion` and can be completed successfully or exceptionally via a POST request to the appropriate URI returned by the _completionUrl()_ or _failUrl()_ methods of `ExternalCloudFuture`.

#### Successful Value

When a POST request is made to the _completionUrl_ of a stage, the HTTP status and headers will be persisted alongside the body of the request. For example, completing the stage via the request
 
```
Content-Type: application/json
CUSTOM-HEADER: user-12334

{
   "result" : "12345678"
}
```

will result in the following being transmitted to the runtime:

```
FnProject-DatumType: httpreq
FnProject-Method: POST
FnProject-ResultStatus: success
FnProject-Header-CUSTOM-HEADER: user-12334
Content-Type: application/json

{
   "result" : "12345678"
}
```

In the Java runtime, this stage's value will be transparently coerced to the `ExternalResult` type, which wraps the body and headers of the original request.

#### Failed Value

POSTing to the _failUrl_ will also result in the completer saving HTTP status, headers and body of the original request. For example, the following POST request

```
Content-Type: application/json
CUSTOM-HEADER: user-12334

{
   "reason" : "INVALID_USER_AUTHENTICATION",
   "message" : "Failed to authenticate principal, password was invalid"
}
```

will result in the following being transmitted to the runtime:

```
FnProject-DatumType: http
FnProject-Method: POST
FnProject-ResultStatus: failure
FnProject-Header-CUSTOM-HEADER: user-12334
Content-Type: application/json

{
   "reason" : "INVALID_USER_AUTHENTICATION",
   "message" : "Failed to authenticate principal, password was invalid"
}
```

In the Java runtime, this stage's value will be transparently coerced to the `FunctionInvocationException` type, which wraps the body and headers of the original request.




### Completer Client API
We'll swagger this up at some point 


| Route														| HTTP Method  | Description |
| ---      													| ---         	 | ---       |
| /graph?functionId=${fn_id} 								| POST 			| Creates a new execution graph cloud for the given cloud thread function. |
| /graph/${graph_id}										| GET  			| Returns a JSON representation of the  thread completion graph. |
| /graph/${graph_id}/commit								    | POST 			| Signals that that  thread entrypoint function has finished executing and the graph is now committed. |
| /graph/${graph_id}/supply								    | POST 			| Adds a root stage to this thread's graph that asynchronously executes a Supplier closure. Analogous to CompletableFuture's `supplyAsync`. http datum |
| /graph/${graph_id}/invokeFunction?functionId=/app/somefn/path| POST 			| Adds a root stage to this thread's graph that asynchronously invokes an external FaaS (fn) function.  The content of the body should correspond to an httpreq datum. When the stage is completed it will yield a httpresp datum |
| /graph/${graph_id}/completedValue							| POST 			 | Adds a root stage to this thread's graph that is completed with the value provided in the HTTP request body. Analogous to CompletableFuture's `completedFuture`. |
| /graph/${graph_id}/delay?delayMs=uint									| POST 		| Adds a root stage to this thread's graph that completes asynchronously with an empty value after the specified delay. |
| /graph/${graph_id}/allOf?cids=c1,c2,c3									| POST 		 | Adds a stage to this thread's graph that is completed with an empty value when all of the referenced stages complete successfully (or at least one completes exceptionally). Analogous to CompletableFuture's `allOf`. |
| /graph/${graph_id}/anyOfcids=c1,c2,c3									| POST 	| Adds a stage to this thread's graph that is completed when at least one of the referenced stages completes successfully (or at least one completes exceptionally). This stage's completion value will be equal to that of the first completed referenced stage. Analogous to CompletableFuture's `anyOf`. |
| /graph/${graph_id}/externalCompletion						| POST 			| Adds a root stage to this thread's graph that can be completed or failed externally via an HTTP post to `/graph/${graph_id}/stage/${stage_id}/complete` or `/graph/${graph_id}/stage/${stage_id}/fail`. Analogous to creating an empty CompletableFuture. |
| /graph/${graph_id}/stage/${stage_id}						| GET 			| | datum | Blocks waiting for the given stage to complete, returning the associated value or error if executed exceptionally. |
| /graph/${graph_id}/stage/${stage_id}/complete				| POST 			| Completes an `externalCompletion` stage successfully with the value provided in the HTTP request body. Analogous to CompletableFuture's `complete`.|
| /graph/${graph_id}/stage/${stage_id}/fail					| POST 			| Completes an `externalCompletion` stage exceptionally with the error provided in the HTTP request body. Analogous to CompletableFuture's `completeExceptionally`.|
| /graph/${graph_id}/stage/${stage_id}/acceptEither?other=${other_stage}			| POST 	 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#acceptEither-java.util.concurrent.CompletionStage-java.util.function.Consumer-). |
| /graph/${graph_id}/stage/${stage_id}/applyToEither?other=${other_stage}		| POST  | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#applyToEither-java.util.concurrent.CompletionStage-java.util.function.Function-). |
| /graph/${graph_id}/stage/${stage_id}/thenAcceptBoth?other=${other_stage}		| POST 		 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenAcceptBoth-java.util.concurrent.CompletionStage-java.util.function.BiConsumer-). |
| /graph/${graph_id}/stage/${stage_id}/thenApply			| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenApply-java.util.function.Function-). |
| /graph/${graph_id}/stage/${stage_id}/thenRun				| POST 			| Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenRun-java.lang.Runnable-). |
| /graph/${graph_id}/stage/${stage_id}/thenAccept			| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenAccept-java.util.function.Consumer-). |
| /graph/${graph_id}/stage/${stage_id}/thenCompose			| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenCompose-java.util.function.Function-). |
| /graph/${graph_id}/stage/${stage_id}/thenCombine?other=${other_stage}			| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#thenCombine-java.util.concurrent.CompletionStage-java.util.function.BiFunction-). |
| /graph/${graph_id}/stage/${stage_id}/whenComplete			| POST 			| Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#whenComplete-java.util.function.BiConsumer-). |
| /graph/${graph_id}/stage/${stage_id}/handle				| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#handle-java.util.function.BiFunction-). |
| /graph/${graph_id}/stage/${stage_id}/exceptionally		| POST 			 | Analogous to the [CompletionStage operation of the same name](https://docs.oracle.com/javase/8/docs/api/java/util/concurrent/CompletionStage.html#exceptionally-java.util.function.Function-). |

Note that all operations that add a stage execute any associated closures asynchronously. The completion ID of the associated stage is returned in the `FnProject-CompletionID` header of the HTTP response. The caller can then block waiting for the stage value by making an HTTP GET request to `/graph/${graph_id}/stage/${stage_id}`.

Data is exchanged between the client and the completer and the completer and the function using HTTP multipart messages 
 
### Completer Invoke API

The following sections describe the completer contract for outgoing requests into fn's public API.

#### Closure Invocation

The completer makes a closure invocation request into fn when triggering a stage of computation. The request consists of a closure optionally followed by a variable number of arguments. In order to frame the different elements of the request, HTTP multipart should be used. The closure and arguments constitute the individual parts and must be named _closure_ and _arg_X_ where X is the index of the argument starting at 0. Note also that the arguments should appear in increasing order in the request, with the lowest index appearing immediately after the closure part. 

Example request:

```
POST /r/app/path HTTP/1.1
Content-Type: multipart/form-data; boundary="01ead4a5-7a67-4703-ad02-589886e00923"
FnProject-ThreadID: thread-abcd-12344
FnProject-StageID: 2
Content-Length: 707419

--01ead4a5-7a67-4703-ad02-589886e00923
Content-Type: application/java-serialized-object
Content-Disposition: form-data; name=closure
FnProject-DatumType: blob

...serialized closure...
--01ead4a5-7a67-4703-ad02-589886e00923
Content-Type: application/java-serialized-object
Content-Disposition: form-data; name=arg_0
FnProject-DatumType: blob

...serialized arg 0...
--01ead4a5-7a67-4703-ad02-589886e00923
Content-Disposition: form-data; name=arg_1
FnProject-DatumType: empty

--01ead4a5-7a67-4703-ad02-589886e00923
```

Example response: 

```
Content-Type: application/java-serialized-object
FnProject-DatumType: blob
FnProject-ResultStatus: success

...serialized result...
```

#### Function Invocation

The completer makes a function request into fn when triggering execution of a stage associated with a function (i.e. a stage created via _invokeFunction_). The outgoing request will incorporate the HTTP method, headers, and body supplied by the application when constructing the stage.

Example request:

```
POST /r/app/path HTTP/1.1
Content-Type: application/octet-stream
FnProject-ThreadID: thread-abcd-12344
FnProject-StageID: 2
Content-Length: 707419
Custom-Header: SomeValue

...request body from stage...
```

Example response:

```
HTTP/1.1 200 OK 
Content-Type: application/json
Content-Length: 10419
Custom-Header: SomeOtherValue

...response body from function...
```


### <a name="stage_types">Completion Stage Types</a>




|Stage Type|Trigger Conditions|Successful Execution Strategy|Failed Execution Strategy|Completion Strategy|
| ---      | ---              | ---                         | ---                     | ---                          |
|acceptEither|when any of the parent stages completes|invoke closure with first parent result|complete with error|result of closure invocation or error|
|supply|immediately|invoke closure|complete with error|result of closure invocation or error|
|thenAccept|when parent stage completes|invoke closure with parent result|complete with error|result of closure invocation or error|
|applyToEither|when any of the parent stages completes|invoke closure with parent result|complete with error|result of closure invocation or error|
|thenApply|when parent stage completes|invoke closure with parent result|complete with error|result of closure invocation or error|
|exceptionally|when parent stage completes|complete with parent result|invoke closure with parent error|result of closure invocation or error|
|thenCompose|when parent stage completes|invoke closure with parent result/error tuple|complete with error|result of completion stage returned in closure or error|
|handle|when parent stage completes|invoke closure with parent result/error tuple|invoke closure with parent result/error tuple|result of closure invocation or error|
|runAfterBoth|when all of the parent stages completes|invoke closure|complete with error|result of closure invocation or error|
|runAfterEither|when any of the parent stages completes|invoke closure|complete with error|result of closure invocation or error|
|thenRun|when any of the parent stages completes|invoke closure|complete with error|result of closure invocation or error|
|runAsync|immediately|invoke closure|complete with error|result of closure invocation or error|
|whenComplete|when parent stage completes|invoke closure with parent result/error tuple|invoke closure with parent result/error tuple|result of parent stage or error|
|thenAcceptBoth|when all of the parent stages completes|invoke closure with both parents' results|complete with error|result of closure invocation or error|
|thenCombine|when all of the parent stages completes|invoke closure with both parents' results|complete with error|result of closure invocation or error|
|allOf|when all of the parent stages completes|complete with null/void|complete with error|Completes on trigger - see execution strategies|
|anyOf|when any of the parent stages completes|complete with parent result|complete with error|Completes on trigger - see execution strategies|
|value|immediately|complete with literal value|complete with error|Completes on trigger - see execution strategies|
|externally completable|externally via http callback|complete with external value provided in http request|complete with error|Completes on trigger - see execution strategies|
|delay|externally by timer|complete with null/void once timer elapses|complete with error|Completes on trigger - see execution strategies|

