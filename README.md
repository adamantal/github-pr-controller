# GitHub PR Controller

This Kubernetes controller creates custom resources based on GitHub repositories and pull requests.
The created resources can be  further used by downstream controllers or applications to make complex tasks that Github Workflows are not capable of doing.

## TODOs

- admission webhook to forbid changes of repository and pull requests fields
- set up github workflows
- implement repository sync
- implement pullrequest sync
- create deployable helm chart
