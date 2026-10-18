# Tunnels

`synacklab tunnel` gives you a local port that reaches a service your laptop
can't reach directly. The usual case is a database whose firewall or security
group only accepts traffic from the subnet your Kubernetes nodes run in.

```
laptop                         Kubernetes API             node in "app" subnet
psql → 127.0.0.1:5432  ──────  pods/portforward  ──────  socat pod ──→ db-host:5432
```

The command starts a short-lived socat pod in the cluster, forwards a local
port to it through the Kubernetes API, and deletes the pod when you press
Ctrl-C. It needs only a working kube context, so it works the same on any
cloud or on-prem cluster. Nothing else is installed on your machine.

## Using it

```bash
synacklab tunnel db                      # pick from configured databases
synacklab tunnel db prod-orders          # a configured database by name
synacklab tunnel db --engine postgres --host orders.internal            # ad hoc
synacklab tunnel db billing --local-port 13306 --context prod -n tunnels
```

```
✓ Tunnel pod synacklab-tunnel-jdoe-x7k2p ready (namespace synacklab-tunnels)
→ postgres orders.internal:5432 is available at 127.0.0.1:5432
  psql -h 127.0.0.1 -p 5432 -U <user> <database>
  Press Ctrl-C to close the tunnel
```

- The local port defaults to the database port (postgres 5432, mysql 3306).
  If it's taken, pass `--local-port`.
- The tunnel listens on `127.0.0.1` only.
- Database credentials and TLS stay between your client and the database:
  socat only relays bytes.
- Errors from the pod (for example `Connection refused` or `Operation timed
  out` when the database is unreachable) are printed to your terminal.
- If the connection to the API server drops, the tunnel reconnects as long as
  the pod is still running. If the pod stops or is deleted, the tunnel exits
  within about 5 seconds.

Configure databases and pod settings under `tunnel` in
`~/.synacklab/config.yaml` (see [Configuration Reference](config-reference.md#tunnel-configuration)):

```yaml
tunnel:
  node_selector: {subnet: app}
  databases:
    - {name: prod-orders, engine: postgres, host: orders.internal}
    - {name: billing,     engine: mysql,    host: 10.20.3.15, port: 3307}
```

### From a web page: `tunnel serve`

```bash
synacklab tunnel serve                   # http://127.0.0.1:4748
synacklab tunnel serve --context prod -n tunnels --port 4800
```

The page offers the same choices as `tunnel db`: a configured database or an
ad hoc host, engine and port, the local port, kube context and namespace.
It applies the same validation, shows the tunnel's status, how to connect and
the pod's output, and closes the tunnel (deleting the pod) with **Close
tunnel**. One tunnel runs at a time.

- The tunnel keeps running if you close the browser tab; reopen the page to
  see it, or stop `serve` with Ctrl-C, which deletes the pod before exiting.
- The page is local-only: `--bind` accepts only loopback addresses, and the
  server rejects requests addressed to another host name or sent from another
  website.
- If `tunnel.databases` is invalid, the page shows the error and still opens
  ad hoc hosts.

## The tunnel pod

Each tunnel is one pod, deleted on exit:

- runs `alpine/socat` pinned by digest (override with `tunnel.image`), as UID
  65534, read-only root filesystem, all capabilities dropped, `RuntimeDefault`
  seccomp, no service account token, 10m/16Mi requested, 200m/64Mi limit
- labelled `app.kubernetes.io/managed-by=synacklab`, `synacklab.io/user=<you>`,
  `synacklab.io/target=<engine>`
- killed by Kubernetes after `tunnel.max_duration` (default 8h) through
  `activeDeadlineSeconds`, even if the CLI crashed or your laptop slept

List or clean up tunnel pods with kubectl:

```bash
kubectl -n synacklab-tunnels get pods -l app.kubernetes.io/managed-by=synacklab -L synacklab.io/user
kubectl -n synacklab-tunnels delete pods -l app.kubernetes.io/managed-by=synacklab,synacklab.io/user=jdoe
```

## Cluster setup (for cluster admins)

### Namespace and RBAC

Engineers need to create, watch, read logs of, port-forward to, and delete
pods in one namespace, and nothing else:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: synacklab-tunnels
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: synacklab-tunnel
  namespace: synacklab-tunnels
rules:
  - apiGroups: [""]
    resources: [pods]
    verbs: [create, get, list, watch, delete]
  - apiGroups: [""]
    resources: [pods/log]
    verbs: [get]
  - apiGroups: [""]
    resources: [pods/portforward]
    verbs: [create]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: synacklab-tunnel
  namespace: synacklab-tunnels
subjects:
  - kind: Group
    name: engineers            # your IdP / EKS access-entry group
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: synacklab-tunnel
  apiGroup: rbac.authorization.k8s.io
```

If a user lacks these permissions, `synacklab tunnel db` reports the denied
request and points here.

`pods/portforward: create` is enough: synacklab first tries port-forward over
WebSocket, which RBAC checks as `get`, and falls back to SPDY, which it checks
as `create`. Adding `get` lets the WebSocket path succeed directly.

### Networking: make the pod's traffic acceptable to the database

The database sees connections coming from the **tunnel pod's IP**. That IP
must be one the database's firewall or security group accepts:

- **Pick the nodes.** Set `tunnel.node_selector` (per user, or distribute a
  config) to a label of the node pool in the allowed subnet, e.g.
  `{subnet: app}` or `{eks.amazonaws.com/nodegroup: app}`. If no node matches,
  the tunnel fails fast with `Unschedulable`.
- **Pod networking matters.** With the AWS VPC CNI (default on EKS) pods get
  IPs from their node's subnet, so the node selector is enough. With overlay
  CNIs (Calico VXLAN, Flannel, Cilium in tunnel mode) traffic leaving the
  cluster is usually SNATed to the node IP, which also works. With EKS
  custom networking or a separate pod subnet, allow that subnet instead.
- **Security groups for pods (EKS).** Alternatively attach a security group to
  tunnel pods with a `SecurityGroupPolicy` selecting
  `app.kubernetes.io/managed-by: synacklab`, and allow that group on the
  database.

A blocked connection shows up as `Operation timed out` in the tunnel's output
after about 10 seconds.

### Optional: limit where tunnel pods can connect

A NetworkPolicy restricts tunnel pods to your database ranges (requires a CNI
that enforces NetworkPolicy):

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: synacklab-tunnel-egress
  namespace: synacklab-tunnels
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/managed-by: synacklab
  policyTypes: [Ingress, Egress]
  ingress: []                    # port-forward goes through the kubelet, not pod ingress
  egress:
    - to:
        - ipBlock: {cidr: 10.20.0.0/16}   # database subnet(s)
      ports:
        - {protocol: TCP, port: 5432}
        - {protocol: TCP, port: 3306}
    - to:                                  # DNS, for database hostnames
        - namespaceSelector: {}
          podSelector:
            matchLabels: {k8s-app: kube-dns}
      ports:
        - {protocol: UDP, port: 53}
        - {protocol: TCP, port: 53}
```

### Private registries

If nodes can't pull from Docker Hub, mirror the image and set `tunnel.image`
to the mirrored reference (keep it pinned by digest). Any image whose
entrypoint is `socat` and that runs as a non-root user works.
