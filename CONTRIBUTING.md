## How to Contribute

The GitLab-Controller project is under [Apache 2.0 license](LICENSE). We accept contributions via
GitHub pull requests. This document outlines some of the conventions related to
development workflow, commit message formatting, contact points and other
resources to make it easier to get your contribution accepted.

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

Contributors sign-off that they adhere to these requirements by adding a
Signed-off-by line to commit messages. For example:

```
This is my commit message

Signed-off-by: Random J Developer <random@developer.example.org>
```

Git even has a -s command line option to append this automatically to your
commit message:

```
git commit -s -m 'This is my commit message'
```

If you have already made a commit and forgot to include the sign-off, you can amend your last commit
to add the sign-off with the following command, which can then be force pushed.

```
git commit --amend -s
```

We use a [DCO bot](https://github.com/apps/dco) to enforce the DCO on each pull
request and branch commits.

## Getting Started

- Fork the repository on GitHub
- Read the [install](INSTALL.md) for build and test instructions
- Play with the project, submit bugs, submit patches!

## Contribution Flow

This is a rough outline of what a contributor's workflow looks like:

- Create a branch from where you want to base your work (usually master).
- Make your changes and arrange them in readable commits.
- Make sure your commit messages are in the proper format (see below).
- Push your changes to the branch in your fork of the repository.
- Make sure all linters and tests pass, and add any new tests as appropriate.
- Submit a pull request to the original repository.

## Building

Details about building gitlab-controller can be found in [INSTALL.md](INSTALL.md).

## Coding Style and Linting

Crossplane projects are written in Go. Coding style is enforced by
[golangci-lint](https://github.com/golangci/golangci-lint). GitLab-Controller's linter
configuration is [documented here](.golangci.yml). Builds will fail locally and
in CI if linter warnings are introduced:

```
$ make build
==> Linting /home/illya/go/src/github.com/crossplaneio/gitlab-controller/cluster/charts/gitlab-controller
[INFO] Chart.yaml: icon is recommended

1 chart(s) linted, no failures
18:25:18 [ .. ] golangci-lint
18:25:24 [ OK ] golangci-lint
18:25:24 [ .. ] helm dep gitlab-controller 0.0.0-1.545b120.dirty
No requirements found in /home/illya/go/src/github.com/crossplaneio/gitlab-controller/cluster/charts/gitlab-controller/charts.
18:25:24 [ OK ] helm dep gitlab-controller 0.0.0-1.545b120.dirty
18:25:24 [ .. ] go build linux_amd64
18:25:24 [ OK ] go build linux_amd64
18:25:25 [ .. ] helm package gitlab-controller 0.0.0-1.545b120.dirty
Successfully packaged chart and saved it to: /home/illya/go/src/github.com/crossplaneio/gitlab-controller/_output/charts/gitlab-controller-0.0.0-1.545b120.dirty.tgz
18:25:25 [ OK ] helm package gitlab-controller 0.0.0-1.545b120.dirty
18:25:25 [ .. ] helm index
18:25:25 [ OK ] helm index
18:25:25 [ .. ] docker build build-e54cdf34/gitlab-controller-amd64
sha256:4ecc71ff3b8814a3c8597006bdb84098f3d543250a098ee148e23232851c67bf
18:25:27 [ OK ] docker build build-e54cdf34/gitlab-controller-amd64
```

Note that Jenkins builds will not output linter warnings in the build log.
Instead `upbound-bot` will leave comments on your pull request when a build
fails due to linter warnings. You can run `make lint` locally to help determine
whether you've fixed any linter warnings detected by Jenkins.

In some cases linter warnings will be false positives. `golangci-lint` supports
`//nolint[:lintername]` comment directives in order to quell them. Please
include an explanatory comment if you must add a `//nolint` comment. You may
also submit a PR against [`.golangci.yml`](.golangci.yml) if you feel
particular linter warning should be permanently disabled.

## Comments

Comments should be added to all new methods and structures as is appropriate for the coding
language. Additionally, if an existing method or structure is modified sufficiently, comments should
be created if they do not yet exist and updated if they do.

The goal of comments is to make the code more readable and grokkable by future developers. Once you
have made your code as understandable as possible, add comments to make sure future developers can
understand (A) what this piece of code's responsibility is within Crossplane's architecture and (B) why it
was written as it was.

The below blog entry explains more the why's and how's of this guideline.
https://blog.codinghorror.com/code-tells-you-how-comments-tell-you-why/

For Go, GitLab-Controller follows standard godoc guidelines.
A concise godoc guideline can be found here: https://blog.golang.org/godoc-documenting-go-code

## Commit Messages

We follow a rough convention for commit messages that is designed to answer two
questions: what changed and why. The subject line should feature the what and
the body of the commit should describe the why.

```
aws: add support for feature XYZ

this commit enables controllers and apis for SYZ.
```

The format can be described more formally as follows:

```
<subsystem>: <what changed>
<BLANK LINE>
<why this change was made>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the
second line is always blank, and other lines should be wrapped at 80 characters.
This allows the message to be easier to read on GitHub as well as in various
git tools.


## Local Build and Test

To learn more about the developer iteration workflow, including how to locally test new types/controllers, please refer to the [Local Build](cluster/local/README.md) instructions.
