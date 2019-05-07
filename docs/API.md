# Fn Flow API

This document defines how to interact with flows via the flow service, and how the flow service invokes flow stages via fn.

There are two API call contracts: 

The [Client API](#completer-client-api) between a client function and the flow service: Functions make calls to the flow service to create flows and append completion stages , the flow service stores these and invokes the stages when they are triggered.

The [Invoke API](#completer-invoke-api) between the flow service and a the fn service: The flow service invokes back into fn via its public API to trigger stages of the computation. The function code interprets incoming requests and dispatches code to the appropriate implementations before returning a result back to the flow service.

 

## Key Concepts: 

A *flow* is a single graph of computation associated with a single function, created from an invocation of that function. Flows are identified by a `FlowID`.

A *completion stage* is a single node in the flow graph that is created by user code either from the original function invocation or from a subsequent *continuation invocation* of that graph. Completion stages are identified by a `StageID` which is unique within the respective graph.

Completion stages consist of the following : 
  *  A *stage type* that describes what the stage should do, how arguments should be passed into the stage and how results of the stage should be handled see [Stage Types](#stage-types) for a list of supported stage types
  *  A *closure* that describes what code to run within the original function container, when the stage is activated -  Not all stage types require a closure (e.g. delay). The closure is interpreted by the calling function and may be of any type - in Java this may for instance be a serialized lambda expression and its captured arguments.
  *  Zero or more *stage dependencies* that trigger the stage, the stage type determines under which circumstances the dependencies cause the stage to trigger.
  * A *stage result* : this corresponds to the (successful or failed) value associated with stage once it has completed - this value is used to trigger downstream stages.  
  

## Fn Flow Application Lifecycle

The following sections define the request/response protocol for the lifetime of a Fn Flow application.
### Runtime Creates a Flow (Function->Flow Service)

The function creates a new flow by POST an empty request to the `/v1/flows` endpoint with a function ID  of the current function.
 
The function ID is the qualified path of the function in Fn, containing the app name and route. 


```
POST /v1/flows HTTP/1.1
Content-type: application/json 

{
   "function_id" : "myapp/myroute",
}

```

The flow service returns with an empty response containing the new flow ID in the FnProject-FlowID header:

```
HTTP/1.1 200 OK 
Content-type: application/json 

{"graph_id":"1212b145-5695-4b57-97b8-54ffeda83210"}
``` 

### Runtime creates a stage in the graph
Stages can be added to a graph at any time and are executed as soon as their dependencies are satisfied.

#### Storing Blob data  

The flow services does not directly handle content from functions (with the exception of HTTP headers, see below)

Data must be persisted by function invocations before being passed to flow services:

```
POST /blobs/flow-abcd-12344 HTTP/1.1
Content-type: application/java-serialized-object

...serialized lambda...
```

Which returns a blob description: 

```
HTTP/1.1 200 OK 
Content-type: application/json 

{ 
   "blob_id" : "adsadas",
   "length": 21321, 
   "content_type": "application/java-serialized-object"
}
```

Once a blob is stored you can pass it by reference into a stage as either a value or a closure. 


#### Runtime creates a stage with a closure  

For example, the runtime POSTs a *closure*  to one of the stage operations (see API below): 

```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stage HTTP/1.1
Content-type: application/json 
    
{

    "operation": "supply",
    "closure": { 
         "blob_id": "my_blob_id",
         "length": 100, 
         "content_type": "application/java-serialized-object"
    },
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}

```

(`code_location` is optional and is used for information purposes) 

The flow service returns a new `stage_id"  in the body: 
 
```
HTTP/1.1 200 OK
Content-type: application/json 

{"graph_id":"b4a726bd-b043-424a-b419-ed1cfb548f4d","stage_id":"1"}
```

#### Runtime creates a stage with dependencies 
Some stages take other stages as dependencies, and will execute when some or all of these dependencies succeed or fail


e.g. to create a `thenApply` stage that executes a closure after a preceding stage is complete: 

```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stage HTTP/1.1
Content-type: application/json 
    
{

    "operation": "thenApply",
    "closure": { 
         "blob_id": "my_blob_id",
         "length": 100, 
         "content_type": "application/java-serialized-object"
    },
    "deps" : ["1"]
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}

```

or an `thenCombine` stage that blocks until two stages are complete and passes both results to a closure 
```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stage HTTP/1.1
Content-type: application/json 
    
{
    "operation": "thenCombine",
    "closure": { 
         "blob_id": "my_blob_id",
         "length": 100, 
         "content_type": "application/java-serialized-object"
    },
    "deps" : ["1","2","3"]
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}
```

or an `allOf` stage that blocks until all other stages are complete but takes no closure: 
```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stage HTTP/1.1
Content-type: application/json 
    
{

    "operation": "allOf",
    "deps" : ["1","2","3"]
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}

```
 

### Runtime requests a function invocation via the flow service

`invoke` creates a stage that immediately executes a call to another function in Fn and contains the target `function_id`, the  function's HTTP headers, method and body. The flow service will then use this datum to create and send a request to fn upon successfully triggering this stage.

```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/invoke
Content-type: application/json 

{
    "function_id" :"otherapp/fn",
    "arg": {
          "body" : {
            "blob_id": "my_blob_id",
            "length": 100,
            "content_type": "application/java-serialized-object"
           },
           "method": "post",
           "headers": [ { "key":"accept","value":"*/*"}]
    },
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}
```

Again the flow service returns a new stage ID in the body: 


The `body` field is optional (in which case no HTTP body will be passed to the target function): 



```
POST /v1 
```
HTTP/1.1 200 OK
Content-type: application/json 

{"graph_id":"b4a726bd-b043-424a-b419-ed1cfb548f4d","stage_id":"1"}
```/flows/1212b145-5695-4b57-97b8-54ffeda83210/invoke
Content-type: application/json 

{
    "function_id" :"otherapp/fn",
    "arg": {       
           "method": "get",
    },
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}
```

#### Runtime creates a standalone completion stages 

Most stages are designed to be chained together but you can also create and complete stages directly: 


create an empty stage: 

```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stage
Content-type: application/json 

{
   "operation": "externalCompletion" 
}
```
 
```
HTTP/1.1 200 OK
Content-type: application/json 

{"graph_id":"b4a726bd-b043-424a-b419-ed1cfb548f4d","stage_id":"3"}
```

You can then complete this stage with an existing datum and a status (indicating whether the stage should be treated as successful or not ) : 




```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stages/3/complete
Content-type: application/json 

{
    "value" : {
            "successful": true, 
            "datum": {
                 "empty": {}
               }
       }
}
```



### Runtime creates a completed future on the flow service 

Clients can create stages that are already completed using the `value` operation 


```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/value

Content-Type: application/json


{
    "value" : {
         "successful": true, 
         "datum": {
              "blob": { 
                          "blob_id": "my_blob_id",
                          "length": 100, 
                          "content_type": "application/java-serialized-object"
                 }
            }
    },
    "code_location" : "com.myfn.MyClass#invokeFunction:123"
}

```

The flow service returns a new stage response: 

```
HTTP/1.1 200 OK
Content-type: application/json 

{"graph_id":"b4a726bd-b043-424a-b419-ed1cfb548f4d","stage_id":"1"}
```




#### Runtime waits for a stage to complete 

Generally runtimes should not block on graph events as it will consume resources in the client side unnecessarily, however this is possible using the `await` endpoint : 

```
GET /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/stages/1/await?timeout_ms=1000

```

If the stage completes within the timeout or is already completed then it will return a result: 

```
HTTP/1.1 200 OK 
Content-type: application/json 


{
    "flow_id" : "1212b145-5695-4b57-97b8-54ffeda83210",
    "stage_id" : "1",
    "result" : {
         "successful": true, 
         "datum": {
              "blob": { 
                          "blob_id": "my_blob_id",
                          "length": 100, 
                          "content_type": "application/java-serialized-object"
                 }
            }
    }
}
```

if the stage does not complete within the timeout the service replies with a 408 

```
HTTP/1.1 408 timeout 
Content-type: application/json 


{
   "error": "Deadline Exceeded", 
   "code:" 4
}
```


### Runtime signals that initial flow is committed 
  
A graph is completed  (and can no longer be modified) once all stages in the graph that can be executed are completed (note that  some stages may not be run). 

The flow service observes the state of the graph to determine when pending work is complete,  to detect this condition, however as the graph is is created by a process that is outside of the flow service's control (e.g. a function not run by the flow service itself) that process must  indicate to the flow service that it has finished modifying the graph by calling the `commit` API call on a graph. 

e.g.: 
```
POST /v1/flows/1212b145-5695-4b57-97b8-54ffeda83210/commit HTTP/1.1
```


### Flow service Invokes a Continuation

The flow service invokes stages using the Fn API using a special JSON message. 

FDKs implementing flow should detect incoming flow invocations (using the `Fnproject-FlowID` header ) and handle them as flow stages rather than Fn invocations 

For example:

```
POST /invoke/fnid  HTTP/1.1
Content-Type: application/json 

FnProject-FlowID: 767b1b6d-bf7e-4739-b720-783518198176

{
  "flow_id" : "767b1b6d-bf7e-4739-b720-783518198176", 
  "stage_id" : "2", 
  "closure" : {
    "blob_id" : "07284d92-b38e-41c9-8a61-994a0783994e",
    "length" : 1201, 
    "content_type" : "application/java-serialized-object"
  },
  "args" : [
    {
            "successful": true, 
            "datum": {
                 "empty": {}
               }
       },
       {
            "successful": true, 
            "datum": {
                 "blob": {
                    "blob_id" : "bf1ec054-ed15-4802-9f9f-5f1c73a21eb3",
    "length" : 1201, 
    "content_type" : "application/java-serialized-object"
                 }
               }
       }
    ]
}
```

#### Runtime response to stage invocation  of the result

The FDK function should reply to an invocation request with an invocation response : 

```
{
 "result" :  {
                        "successful": true, 
                        "datum": {
                             "blob": {
                                "blob_id" : "bf1ec054-ed15-4802-9f9f-5f1c73a21eb3",
                "length" : 1201, 
                "content_type" : "application/java-serialized-object"
                             }
                           }
                   }
}

```





##  Data handling  

Flow operations use a consistent  set of data types to describe information being passed through the graph. 


###  Blobs

A blob is a wrapper for some data stored externally to flow, it is used to describe closures and stage arguments. 

```json 
{ 
         "blob_id": "my_blob_id",
         "length": 100, 
         "content_type": "application/java-serialized-object"
}
``` 

### Datum types 
All data that is passed between stages in flow are expressed as _datum_ types that wrap the underlying raw type of a given data type: 

#### Blob Datums:

A blob datum wraps a blob
```json
{
 "blob" : { 
                   "blob_id": "my_blob_id",
                   "length": 100, 
                   "content_type": "application/java-serialized-object"
          }
}
```

#### Empty Datum: 
An empty datum represents a null or empty value : 
```json
{
  "empty" : {}
}
```

#### HTTP Request Datum 
 
```json
  { "http_req" : {
          "body" : {
            "blob_id": "my_blob_id",
            "length": 100,
            "content_type": "application/java-serialized-object"
           },
           "method": "post",
           "headers": [ { "key":"accept","value":"*/*"}]
    }
   }
```
#### HTTP Response Datum

 
```json
{ 
    "http_resp" : {
          "body" : {
            "blob_id": "my_blob_id",
            "length": 100,
            "content_type": "application/java-serialized-object"
           },
           "status_code": "200",
           "headers": [ { "key":"accept","value":"*/*"}]
    }
}
```


#### StageRef Datum : 
StageRefs correspond to a pointer to another, existing stage in the current graph. These are used in operations like `thenCompose` to link sub-graphs together. 

```
{
    "stage_ref" : {
         "stage_id" : "0" 
    }

```


#### Error Datum: 

Completion stages can also fail due to errors thrown outside of the user's
code. For example, the flow service may time out while waiting for a response to
a continuation request. In such cases, the completion stage will fail, but
there will be no exception or stacktrace associated with the failure.

In the case of such a failure the flow-service will generate an _error datum_ that represents teh type of the message. 

```json
{
 "error" :{
     "type": "stage-timeout", 
     "message" : "Stage invocation timed out"
 }
}
```

| Error Type | Meaning |
| ---         | ----   |
| stage_timeout | a completion stage function timed out - the stage may or may not have completed normally'|
| stage_invoke_failed | a completion stage invocation failed  within Fn  - the stage may or may not have been invoked  and that invocation may or may not have completed |
| function_timeout | A function call timed out | 
| function_invoke_failed | A function call failed within Fn platform  - the function may or may not have been invoked  and that invocation may or may not have completed | 
| stage_lost | A stage failed after an internal error in the flow service the stage may or may not have been invoked  and that invocation may or may not have completed| 
| invalid_stage_response | A stage generated an invalid response or an invalid datum type  (e.g. thenCompose returning a blob datum) |


Recipients must accept unknown values for this field.

#### Status Datum

The state datum is a special datum that is only used in termination hooks to denote how the graph was terminated :

```json
{
	"status_datum" : {
		 "type" :"succeeded"
	}
}

```

Valid types are:
*   succeeded
*   failed
*   cancelled
*   killed


### Flow Client API
See [swagger](../model/model.swagger.json)



# <a name="stage_types">Completion Stage Types</a>




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
|thenRun|when any of the parent stages completes|invoke closure|complete with error|result of closure invocation or error|
|runAsync|immediately|invoke closure|complete with error|result of closure invocation or error|
|whenComplete|when parent stage completes|invoke closure with parent result/error tuple|invoke closure with parent result/error tuple|result of parent stage or error|
|thenAcceptBoth|when all of the parent stages completes|invoke closure with both parents' results|complete with error|result of closure invocation or error|
|thenCombine|when all of the parent stages completes|invoke closure with both parents' results|complete with error|result of closure invocation or error|
|allOf|when all of the parent stages completes|complete with null/void|complete with error|Completes on trigger - see execution strategies|
|anyOf|when any of the parent stages completes|complete with parent result|complete with error|Completes on trigger - see execution strategies|
|value|immediately|complete with literal value|complete with error|Completes on trigger - see execution strategies|
|delay|externally by timer|complete with null/void once timer elapses|complete with error|Completes on trigger - see execution strategies|

