# Setting up a small demo on k8s

Deploy the fnproject runner - either as a DaemonSet (with appropriate
message queues, etc) or as a standalone. The service should be available
with the name "fn-service.fn" on port 8080.

    kubectl create -f fn-service.yaml

Then deploy the StatefulSet for the Fn Flow completer. The kubectl
configuration here should work on k8s 1.7.

    kubectl create -f flow-namespace.yaml
    kubectl create -f flow-database.yaml
    kubectl create -f flow-service.yaml

It should be arranged that COMPLETER_API is configured on all applications
requiring access to the Flow completer. (In order for this to work, a
LoadBalancer or some other arrangement should front the Flow completer
with a name or IP address that is resolvable/reachable from the docker
containers that Fn launches.)
