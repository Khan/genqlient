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

The PR description template will remind you of these. Pull requests will be squash-merged, so subsequent commit messages may be brief (e.g. "review comments").

Large changes should typically be discussed on the issue tracker first, and should ideally be broken up into separate PRs, or failing that, several commits, for ease of reviewing. This is especially true of breaking changes; see the [versioning policy](versioning.md) for what we consider breaking.

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

If you're new to genqlient, start out by reading the source of `generate.Generate`, whose comments describe most of the high-level operation of genqlient.  In general, the code is documented inline, often with an introductory comment at the top of the file.  See the [design note](design.md) for documentation of major design decisions, which is a good way to get a sense of why genqlient is structured the way it is.

## Making a release

See the [versioning strategy](versioning.md) for when to make a release. To make a release:

- Scan PRs since the last release to check we didn't miss anything in the changelog.
- Check if there are any regressions or major problems with new features we want to fix before cutting the release.
- Decide the new version number. We do a minor version bump for anything with breaking changes or significant new features, otherwise it can be a patch version bump.
- Add a new section to the changelog for the release (see comments in the changelog for instructions), and add a brief summary of the release at the top.
- Make a PR with the above. (Example: [#208](https://github.com/Khan/genqlient/pull/208).)
- After it merges, tag it as the new release, e.g. `git checkout main && git pull && git tag v0.X.Y && git push origin v0.X.Y`.
- Then, create a release in GitHub, either [on the web](https://github.com/Khan/genqlient/releases/new) or with `export VERSION=v0.6.0; gh release create $VERSION --latest --verify-tag --generate-notes --title $VERSION`. (TODO(benkraft): Figure out how to pull in the changelog we've already written instead!)
