# Versioning policy

This document describes how we manage genqlient versions. See all published versions on [pkg.go.dev](https://pkg.go.dev/github.com/Khan/genqlient?tab=versions) or [GitHub](https://github.com/Khan/genqlient/releases).

## When do we make a release?

In general, we do not cut a release for every bugfix; instead we try to cut a release after major changes have had some time to bake, and in some cases after large codebases using genqlient have tried them. This ensures releases are somewhat more likely to work.

If that stability is desirable to you, use tagged releases of genqlient only (e.g. `go get github.com/Khan/genqlient@latest`), and be aware that new features may take somewhat longer to make it to a release. (Feel free to make an issue to request a release if it's been a while.)

If you always want the latest and greatest changes quickly, Go has excellent support for installing packages at any commit. We do have extensive tests and try to keep the main branch safe for production use, but we're never perfect, so take care appropriate to your use case. You can install the main branch with `go get github.com/Khan/genqlient@main`, or replace `main` with any commit SHA. Please report any bugs you see so they can be fixed before the next release!

For the details of actually making a release, see the [contributor docs](CONTRIBUTING.md#making-a-release).

## What is a breaking change?

We consider the following changes to be breaking:
- breaking changes to the runtime `graphql` package (obviously)
- changes which cause genqlient to, given the same valid query, make a breaking change to the API or behavior of the generating code (i.e. it should be safe to re-run a newer version of genqlient on existing queries)
- changes to the `graphql` runtime package which require corresponding changes to the code-generator (i.e. there's no obligation to run genqlient every time you upgrade)

We don't consider the following changes to be breaking:
- syntactic changes to the generated output for existing queries; if you check that your generated code is up to date in CI you should expect to need to update it when you update genqlient
- changes, including breaking API changes, to any double-underscore-prefixed names in the generated code (i.e. don't refer to these in your code); the same applies to any names from the `graphql` runtime package documented as "intended for the use of genqlient's generated code only"
- changes to the code-generator which require corresponding changes to the `graphql` runtime package (these are safe because your runtime should be the same or newer).
- dropping support for Go versions which are no longer supported by the Go project (all but the [two newest](https://go.dev/doc/devel/release))

Additionally, your version of the `graphql` runtime package must be the same or newer (and the same major version) as your version of the code-generator. (It's recommended to use the same version of both, but it's not required to regenerate all your queries after upgrading.)

Note that while genqlient is on version 0.x we may make breaking changes at any time, although we still aim to do so only in minor version bumps (0.6.0, not 0.5.1), and we aim to minimize breaking changes, especially to core functionality.
