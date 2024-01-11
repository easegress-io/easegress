# Contributing <!-- omit from toc -->

Welcome to the Easegress contributing guide. Are you looking for help or a way to create bug tickets or discuss new potential features? Perhaps you want to fix a typo in the documentation or are you interested in getting involved in the project? You can find answers to these questions and how to contribute to Easegress in this document.

- [Getting help](#getting-help)
- [Feature requests](#feature-requests)
- [Reporting general issues](#reporting-general-issues)
- [Contributing](#contributing)
- [Pull request guide](#pull-request-guide)

## Getting help

If you have not already, please check the [README.md](./README.md#getting-started) and the Easegress [documentation](./doc/README.md#easegress-documentation). If you don’t find an answer to your problem, you can ask your question at [Slack](./README.md#community) or in other community channels.

## Feature requests

Do you want to suggest an idea for the project? Use this *Feature request* [issue template](https://github.com/megaease/easegress/issues/new?template=feature_request.md) to describe your idea.

## Reporting general issues

There are many scenarios when you could open an issue, including:
- Feature request
- Feature proposal (new filter, new object)
- Feature design
- Bug found
- Performance issues
- Help wanted
- Documentation out of date
- Test improvement
- Any questions on project

Please describe clearly and explicitly your issue. Try to add as many details as you can. You can follow the instructions in the issue ticket to ensure that the issue contains enough background: https://github.com/megaease/easegress/issues/new/choose

## Contributing

All contributions to Easegress are welcome! It does not necessary need to be coding; you can also contribute without coding by

- Reporting a bug
- Helping other members of the community at Slack channel
- Fixing a typo in the code
- Fixing a typo in the documentation
- Providing your feedback on the proposed features and designs
- Reviewing Pull Requests

Contributing code, like bug fixes or new features are equally encouraged! Easegress accepts proposals for new Filters and new Objects or any other useful code changes. Here’s incomplete list of possible code contributions:

- New Filter
- New Object
- New feature (other than filter or object)
- Fixing a bug in the code
- Code refactoring
- Performance improvement
- New unit test or improvement of existing test


If you’re unsure, whether your code contribution will be beneficial, open a ticket or ask in Slack. You can check the [Developer Guide](./docs/06.Development-for-Easegress/6.1.Developer-Guide.md) to understand the high level architecture and Filter and Object extensions of Easegress.

For any code or documentation change, please read the following Pull request guide.

For coding standards, refer to the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

## Pull request guide

When contributing to Easegress, it’s good idea to follow these steps:

1. If there is no issue yet, create an issue of the fix or improvement
2. Fork the repository and clone your fork:
   1. `git clone https://github.com/<yourusername>/easegress.git`
3. Track the upstream remote
   1. `git remote add upstream https://github.com/megaease/easegress.git`
4. Create your branch, for example
   1. `git checkout -b fix/<micro-title>-<issue-number>`
5. Do your changes and commit them
   1. `git commit -am '<descriptive-message>'`
6. Rebase latest changes from upstream remote.
   1. `git checkout main`
   2. `git pull --rebase origin main`
   3. `git pull --rebase upstream main`
   4. `git checkout <your-branch>`
   5. `git rebase main`
7. Push your changes to your branch.
   1. `git push -f origin <your-branch>`
8. You can now open PR. Add a description for your PR.
   
For code changes, remember to add unit tests. Before opening PR, you can use Makefile entries (`make fmt`, `make test`, `make integration_test`) to run unit tests and formatting. CI runs Revive code analysis that you can install and execute locally following [these instructions](https://github.com/mgechev/revive#installation).
