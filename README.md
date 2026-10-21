# Block Multipart

Temporary Traefik middleware: returns HTTP 403 whenever any Content-Type header
contains `multipart` (case insensitive), on every method, except allowed client IPs.
It never reads or closes
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
    blockmultipart:
      allowedIPs:
        - 169.1.17.232
        - 100.125.0.0/16
```

Reference it as `customer-namespace-blockmultipart@kubernetescrd`. Remove the
ingress reference before removing the Middleware. Do not attach it to global
entrypoints: that would also disable uploads to application APIs.

`allowedIPs` accepts IPv4/IPv6 addresses and CIDR ranges. The default empty list
grants no exceptions. Malformed entries reject the Middleware configuration.
Only multipart requests use the allowlist; other traffic always passes through.

By default the client is the connection's `RemoteAddr`. For Cloudflare-proxied
traffic, configure `trustedProxies` with Cloudflare's published IPv4/IPv6 ranges
(the same ranges as Traefik's entrypoint `trustedIPs`). Only a trusted immediate
peer permits `X-Forwarded-For` processing, scanning right to left and stopping at
the first untrusted address. Missing/malformed forwarding data grants no exception.
Headers from an untrusted peer, including `CF-Connecting-IP`, cannot grant access.
Do not trust LAN/node subnets just to recover a client address lost to source NAT.

Traefik must have `proxyProtocol.insecure: false` and trust PROXY protocol only
from Cloudflare; otherwise a caller can forge `RemoteAddr` before this plugin runs.
Keep `forwardedHeaders.insecure: false` with the same Cloudflare-only trust list.
This requirement applies to every entrypoint on which this Middleware is used.

This blocks legitimate uploads too. It is a temporary control, not a PHP patch
or a claim to mitigate every PHP vulnerability.

Run `go test ./...` to verify rejection, passthrough and zero body access.
