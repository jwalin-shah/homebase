# Legacy decision route

`POST /api/v1/decisions` is not mounted by the HomeBase server.

The old handler accepted caller-controlled decision fields, validated them
through the legacy graph path, signed them with a process-local key, and
returned success. That is not an authority boundary: a caller could obtain a
durable-looking decision without an authenticated captain or service-owned
contract.

The running-machine path therefore exposes only typed record ingress and the
authenticated transcript-promotion endpoint. A replacement decision endpoint
must be owner-authenticated, bind the decision to durable evidence and a
versioned contract, and have negative fixtures for forged authority before it
is mounted.
