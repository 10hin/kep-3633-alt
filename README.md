# KEP-3633 alternative

## Overview

[KEP-3633][original-kep-latest] is great and now under developing, but I need it **NOW**.
Luckily, it's behavior can mimic with MutatingAdmissionWebhook.

[original-kep-latest]: https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/3633-matchlabelkeys-to-podaffinity

So, I implement this according to [KEP][original-kep-referencing].

[original-kep-referencing]: https://github.com/kubernetes/enhancements/tree/35befff0ad33187b2c93141d5fe1513a1b4a39a1/keps/sig-scheduling/3633-matchlabelkeys-to-podaffinity

## Roadmap and statuses

- [ ] Implement webhook
    - [ ] append `.spec.affinity.podAffinity.required...duling`
    - [ ] append `.spec.affinity.podAffinity.preferred...duling`
    - [ ] append `.spec.affinity.podAntiAffinity.required...duling`
    - [ ] append `.spec.affinity.podAntiAffinity.preferred...duling`
- [X] Build and publish container image
    - published to `ghcr.io/10hin/kep3633alt:latest`
- [ ] Write installation manifest
    - [ ] Kustomize manifest (depends on [cert-manager](https://cert-manager.io) to provision webhook certificates)
    - [ ] Helm chart without dependencies to cert-manager

## Install

> TODO

## Usage

> TODO
