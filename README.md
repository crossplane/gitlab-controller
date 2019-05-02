# GitLab Controller

[![Build Status](https://jenkinsci.upbound.io/buildStatus/icon?job=gitlab-controller/build/master)](https://jenkinsci.upbound.io/blue/organizations/jenkins/gitlab-controller%2Fbuild/activity)
[![GitHub release](https://img.shields.io/github/release/crossplaneio/gitlab-controller/all.svg?style=flat-square)](https://github.com/crossplaneio/gitlab-controller/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/gitlab-controller/gitlab-controller.svg)](https://img.shields.io/docker/pulls/gitlab-controller/gitlab-controller.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/crossplaneio/gitlab-controller)](https://goreportcard.com/report/github.com/crossplaneio/gitlab-controller)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplaneio%2Fcrossplane.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplaneio%2Fcrossplane?ref=badge_shield)
[![Slack](https://slack.crossplane.io/badge.svg)](https://slack.crossplane.io)
[![Twitter Follow](https://img.shields.io/twitter/follow/crossplane_io.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=crossplane_io&user_id=788180534543339520)

## Overview
GitLab-Controller is Crossplane native application which enables provisioning production-grade GitLab services across 
multiple supported cloud providers. GitLab-Controller leverages Crossplane core constructs such as 
CloudProvider(s), ResourceClass(es), and ResourceClaim(s) to satisfy GitLab Services dependencies on public cloud
managed services. GitLab-Controller utilizes Crossplane Workloads to provision GitLab services and all its dependencies 
on target Kubernetes clusters managed and provisioned by the Crossplane.   

## Architecture and Vision

The design draft of the Crossplane GitLab-Controller 
[initial design](https://docs.google.com/document/d/1_pD0w5rmkx6Rch5IRYhuVIYuSbFCJGlRGNiUxpTfrZ0/edit?usp=sharing). 

## Getting Started and Documentation

TBD: For getting started guides, installation, deployment, and administration, see our 
[Documentation](https://gitlab-controller.io/docs/latest).

## Contributing

Crossplane GitLab-Controller is a community-driven project, and we welcome contributions. 
See [Contributing](CONTRIBUTING.md) to get started.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please open an 
[issue](https://github.com/crossplaneio/gitlab-controller/issues).

## Contact

Please use the following to reach members of the community:

- Slack: Join our [slack channel](https://slack.crossplane.io)
- Forums: [crossplane-dev](https://groups.google.com/forum/#!forum/crossplane-dev)
- Twitter: [@crossplane_io](https://twitter.com/crossplane_io)
- Email: [info@crossplane.io](mailto:info@crossplane.io)

## Community Meeting

A regular Crossplane community meeting takes place every other Tuesday.
For up-to-date meeting information and details see 
[Crossplane Community Meeting](https://github.com/crossplaneio/crossplane#community-meeting)

## Project Status

The project is an early preview. We realize that it's going to take a village to arrive at the vision of a multicloud 
control plane, and we wanted to open this up early to get your help and feedback. Please see the [Roadmap](ROADMAP.md) 
for details on what we are planning for future releases.

### Official Releases

Official releases of GitLab-Controller can be found on the 
[releases page](https://github.com/crossplaneio/gitlab-controller/releases).
Please note that it is **strongly recommended** that you use 
[official releases](https://github.com/crossplaneio/gitlab-controller/releases) of 
GitLab-Controller, as unreleased versions from the master branch are subject to changes and incompatibilities that will
 not be supported in the official releases. Builds from the master branch can have functionality changed and even 
 removed at any time without compatibility support and prior notice.

## Licensing

Gitlab-Controller is under the Apache 2.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fcrossplaneio%2Fgitlab-controller.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fcrossplaneio%2Fgitlab-conroller?ref=badge_large)
