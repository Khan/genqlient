# Contributing to genqlient

genqlient welcomes contributions from the community!  Our [help wanted](https://github.com/Khan/genqlient/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22) label tags issues where community help is especially welcome, and [good first issue](https://github.com/Khan/genqlient/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) lists issues that are likely most approachable for new contributors.

While we're on the subject, Khan Academy is a non-profit organization with a mission to provide a free, world-class education to anyone, anywhere. If you're looking for other ways to help us, you can help us in that mission by [donating](https://khanacademy.org/donate) or looking at [career opportunities](https://khanacademy.org/careers).

## Formalities

In order to contribute to genqlient, you must first sign the [Khan CLA](https://www.khanacademy.org/r/cla). genqlient is released under the [MIT License](../LICENSE). While contributing, you must abide by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Security issues

Security issues should be reported to the [Khan Academy HackerOne program](https://hackerone.com/khanacademy).

## Pull requests

Pull requests should have:

- a clear PR title and description, describing the changes
- a link to the issue fixed, if applicable
- test coverage, as appropriate (see [below](#tests))
- documentation, for new features
- changelog entries

Pull requests will be squash-merged, so subsequent commit messages may be brief (e.g. "review comments").

Large changes should typically be discussed on the issue tracker first, and should ideally be broken up into separate PRs, or failing that, several commits, for ease of reviewing.

## Style

Go style should generally follow the conventions of [Effective Go](https://golang.org/doc/effective_go), and should have no lint errors (`make lint` to check). TODOs follow the [Khan Academy style](https://github.com/Khan/style-guides#todosmessage), i.e. `// TODO(yourusername): Simplify once we drop support for Go 1.23.` In general, try to make your code match the style of the surrounding code.

## Tests

To run tests and lint, `make check`.  (GitHub Actions also runs them.)

Notes for contributors:
- Most of the tests are snapshot-based; see `generate/generate_test.go`.  All new code-generation logic should be snapshot-tested.  Some code additionally has standalone unit tests, when convenient.
- Integration tests run against a gqlgen server in `internal/integration/integration_test.go`, and should cover everything that snapshot tests can't, including the GraphQL client code and JSON marshaling.
- If `GITHUB_TOKEN` is available in the environment, it also checks that the example returns the expected output when run against the real API.  This is configured automatically in GitHub Actions, but you can also use a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) with no scopes.  There's no need for this to cover anything in particular; it's just to make sure the example in fact works.
- Tests should use `testify/assert` and `testify/require` where convenient (when making many simple assertions).

If you update any code-generation logic or templates, even if no new tests are needed you'll likely need to run `UPDATE_SNAPSHOTS=1 go test ./...` to update the [cupaloy](https://github.com/bradleyjkemp/cupaloy) snapshots and the genqlient-generated files used in integration tests and documentation.

## Finding your way around

If you're new to genqlient, start out by reading the source of `generate.Generate`, whose comments describe most of the high-level operation of genqlient.  In general, the code is documented inline, often with an introductory comment at the top of the file.  See [DESIGN.md](DESIGN.md) for documentation of major design decisions, which is a good way to get a sense of why genqlient is structured the way it is.
