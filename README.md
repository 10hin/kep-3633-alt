# KEP-3633 alternative

## Overview

[KEP-3633][original-kep-latest] is great and now under developing, but I need it **NOW**.
Luckily, it's behavior can mimic with MutatingAdmissionWebhook.

[original-kep-latest]: https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/3633-matchlabelkeys-to-podaffinity

So, I implement this according to [KEP][original-kep-referencing].

[original-kep-referencing]: https://github.com/kubernetes/enhancements/tree/35befff0ad33187b2c93141d5fe1513a1b4a39a1/keps/sig-scheduling/3633-matchlabelkeys-to-podaffinity

## Roadmap and statuses

- [X] Implement webhook
    - [X] append `.spec.affinity.podAffinity.required...duling`
    - [X] append `.spec.affinity.podAffinity.preferred...duling`
    - [X] append `.spec.affinity.podAntiAffinity.required...duling`
    - [X] append `.spec.affinity.podAntiAffinity.preferred...duling`
- [X] Build and publish container image
    - published to `ghcr.io/10hin/kep3633alt:latest`
- [ ] Write installation manifest
    - [ ] Kustomize manifest (depends on [cert-manager](https://cert-manager.io) to provision webhook certificates)
    - [X] Helm chart without dependencies to cert-manager

## Install

Use `helm` with adding our repository:

```shell
helm repo add kep3633alt https://10hin.github.io/kep-3633-alt
helm repo update
helm upgrade -i -n kube-system kep3633alt kep3633alt/kep3633alt
```

Or without it:

```shell
helm upgrade -i -n kube-system kep3633alt kep3633alt --repo https://10hin.github.io/kep-3633-alt
```

Chart source is [here](./deployments/helm/kep3633alt).

## Usage

After [installation](#install), deploy pods with pod affinity (or anti-affinity) configured not on `spec` but on `annotations` with JSON format.

If you have pod manifest (typically as pod template in deployment resource) using KEP3633 like following:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: nginx
    pod-template-hash: UNEXPECTABLEVALUE
spec:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: nginx
        topologyKey: topology.kubernetes.io/zone
        matchLabelKeys:
        - pod-template-hash
  # ...
```

above manifest may invalid for now. So, you can alternate it as follows:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kep-3633-alt.10h.in/podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution: |
      [
        {
          "labelSelector": {
            "matchLabels": {
              "app": "nginx"
            }
          },
          "topologyKey": "topology.kubernetes.io/zone",
          "matchLabelKeys": [
            "pod-template-hash"
          ]
        }
      ]
  name: nginx
  labels:
    app: nginx
    pod-template-hash: UNEXPECTABLEVALUE
spec:
  # ...
```

then it applied as follows:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kep-3633-alt.10h.in/podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution: |
      # reduced
  name: nginx
  labels:
    app: nginx
    pod-template-hash: UNEXPECTABLEVALUE
spec:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchLabels:
              app: nginx
              pod-template-hash: UNEXPECTABLEVALUE
          topologyKey: topology.kubernetes.io/zone
  # ...
```

## Usecases

see [KEP3633][kep-3633-userstory]

[kep-3633-userstory]: https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/3633-matchlabelkeys-to-podaffinity#user-stories-optional
