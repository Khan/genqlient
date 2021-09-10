# Contributing to genqlient

genqlient is not yet accepting contributions.  The library is still in a very rough state, so while we intend to accept contributions in the future, we're not ready for them just yet.  Please contact us (via an issue or email) if you are interested in helping out, but please be aware the answer may be that we aren't ready for your help yet, even if such help will be greatly useful once we are!  We'll be ready for the rest of the world soon.

Khan Academy is a non-profit organization with a mission to provide a free, world-class education to anyone, anywhere. If you're looking for other ways to help us, You can help us in that mission by [donating](https://khanacademy.org/donate) or looking at [career opportunities](https://khanacademy.org/careers).

## Tests

To run tests and lint, `make check`.  (GitHub Actions also runs them.)

Notes for contributors:
- Most of the tests are snapshot-based; see `generate/generate_test.go`.  All new code-generation logic should be snapshot-tested.  Some code additionally has standalone unit tests, when convenient.
- Integration tests run against a gqlgen server in `internal/integration/integration_test.go`, and should cover everything that snapshot tests can't, including the GraphQL client code and JSON marshaling.
- If `GITHUB_TOKEN` is available in the environment, it also checks that the example returns the expected output when run against the real API.  This is configured automatically in GitHub Actions, but you can also use a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) with no scopes.  There's no need for this to cover anything in particular; it's just to make sure the example in fact works.

## Design

See [DESIGN.md](DESIGN.md) for documentation of major design decisions in this library.
