# edgewize values

## In Host cluster

| Parameter                          | Description                                                                      | Default                          |
|------------------------------------|----------------------------------------------------------------------------------|----------------------------------|
| `global.imageRegistry`             | global image registry                                                            | `""`                             |
| `global.tag`                       | edgewize image tag                                                               | `v0.6.1`(current latest version) |
| `global.imagePullSecrets`          | image pull secret                                                                | ``                               |
| `edgewize.hostCluster.kubeconfig`  | Host cluster kubeconfig                                                          | `""`                             |
| `gateway.certificateAuthority.crt` | egdewize gateway CA cert, if not set, it will generate a self-signed certificate | `""`                             |
| `gateway.certificateAuthority.key` | egdewize gateway CA key, if not set, it will generate a self-signed certificate  | `""`                             |
| `gateway.advertiseAddress`         | egdewize gateway address                                                         | `""`                             |
| `gateway.dnsNames`                 | egdewize gateway dnsName                                                         | `""`                             |

> if gateway.certificateAuthority.crt and gateway.certificateAuthority.key are not set, it will regenerate a self-signed certificate when upgrade.
> 
> if gateway.advertiseAddress and gateway.dnsNames changed, it will regenerate when upgrade.

## In Edge cluster
