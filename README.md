# Block Multipart

Temporary Traefik middleware: returns HTTP 403 whenever any Content-Type header
contains `multipart` (case insensitive), on every method. It never reads or closes
the request body, buffers uploads, or checks Content-Length. Other requests pass
to the next handler unchanged.

Register as `blockmultipart` in Traefik's static plugin configuration, pinning a
release and the SHA-256 of the archive served by the Traefik plugin registry.
Traefik expects the hexadecimal digest without a `sha256:` prefix.

For immediate deployment before the catalog's daily import, prod-infra mounts
the versioned sources in an immutable ConfigMap and registers `localPlugins`.
Terraform verifies each source file against its pinned SHA-256 before applying.

Registration alone does not block traffic. The application owns creating and
attaching a Middleware to selected customer ingresses:

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: blockmultipart
  namespace: customer-namespace
spec:
  plugin:
    blockmultipart: {}
```

Reference it as `customer-namespace-blockmultipart@kubernetescrd`. Remove the
ingress reference before removing the Middleware. Do not attach it to global
entrypoints: that would also disable uploads to application APIs.

This blocks legitimate uploads too. It is a temporary control, not a PHP patch
or a claim to mitigate every PHP vulnerability.

Run `go test ./...` to verify rejection, passthrough and zero body access.
