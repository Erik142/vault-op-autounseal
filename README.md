# Vault 1Password Autounseal Controller
## Introduction

This project intends to simplify deployments of Hashicorp Vault inside baremetal Kubernetes clusters. Hashicorp Vault is an industry standard tool used to store secrets. Running Hashicorp Vault in a cloud environment usually comes with an autoseal functionality which is not available in baremetal clusters. In baremetal clusters, Vault has to be unsealed manually after each and every restart of the Vault Pods. This project intends to change that, by instead utilizing 1Password as a Vault Key backend.

Vault 1Password Autounseal Controller continuously monitors the state of the Vault cluster, and will automatically initialize the Vault cluster (if applicable) and unseal any sealed Vault Pods. This is achieved by updating a 1Password secret with the Vault keys after initializing the Vault, combined with injecting 1Password secrets into the Kubernetes cluster by using [1Password Connect Kubernetes Operator](https://github.com/1Password/onepassword-operator) in order to unseal the Vault.

https://github.com/Erik142/vault-op-autounseal/assets/4168364/890673b0-17ae-4ce4-a2b2-12e9d87639e7

## Installation
### Pre-requisites

- [1Password Connect Kubernetes Operator](https://github.com/1Password/onepassword-operator)
- [Hashicorp Vault Helm Chart](https://github.com/hashicorp/vault-helm)

### Installation using Deployment

Vault 1Password Autounseal Controller can be installed using the [manifest file](examples/deployment.yaml). The manifest file will install the necessary RBAC items as well as the Deployment itself. However, in order to install Vault 1Password Autounseal Controller, the Deployment should be customized using the following environment variables:
<br/>
<br/>

| Environment variable | Default value | Description |
| -------------------- | ------------- | ----------- |
| ONEPASSWORD_ITEM_NAME | vault | The name of the OnePasswordItem object which injects the 1Password secret |
| ONEPASSWORD_ITEM_NAMESPACE | vault | The namespace of the OnePasswordItem object which injects the 1Password secret |
| ONEPASSWORD_TOKEN | "" | The 1Password Connect Token |
| ONEPASSWORD_HOSTNAME | op-connect.svc.cluster.local | The hostname of the 1Password Connect server |
| VAULT_STATEFULSET_NAMESPACE | vault | The namespace of the Vault server StatefulSet |
<br/>
<br/>

Download the manifest file, customize the environment variables, and apply it to the Kubernetes cluster:

```sh
kubectl apply -f ./deployment.yaml
```
