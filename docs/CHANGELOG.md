# Changelog

<!--

When releasing a new version:
- change "next" to the new version, and delete any empty sections
- copy the below template to be the new "next"

## template

### Breaking changes:

### New features:

### Bug fixes:

-->

## next

<!-- Add new changes in this section! -->

### Breaking changes:

### New features:

### Bug fixes:

## v0.8.1

This release fixes a bug introduced in v0.8.0 breaking path resolution on Windows, along with some other small features and bugs.

### New features:

- Added `@genqlient(alias)` directive to customize field names without requiring GraphQL aliases (fixes #367)
- Added `auto_camel_case` config option to automatically convert snake_case to camelCase in both field names and type names

### Bug fixes:

- fixed path resolution on Windows
- fixed documentation link in `introduction.md`
- upgraded version of alexflint/go-arg from 1.4.2 to 1.5.1
- fixed a typo in the struct + fragment error message
- avoid error when a subscription message is received without a subscription ID
- avoid closing subscription channels more than once, which could cause a panic in some cases

## v0.8.0

This release adds support for genqlient subscriptions; see the [documentation](subscriptions.md) for more, and thanks to @matthieu4294967296moineau for the original implementation and @HaraldNordgren for additional testing and improvements.

Note that genqlient now requires Go 1.22.5 or higher, and is tested through Go 1.23.3.

### Breaking changes:

- genqlient now forbids `omitempty: false` (including implicit behaviour) when using pointer on non-null input field.
- The error text for HTTP errors has changed slightly. If you were parsing it, switch to [`As`-ing to `graphql.HTTPError`](client_config.md#handling-errors).

### New features:

- genqlient now supports subscriptions; the websocket protocol is by default `graphql-transport-ws` but can be set to another value.  
  See the [documentation](subscriptions.md) for more details on how to use subscriptions.
- genqlient now supports double-star globs for schema and query files; see [`genqlient.yaml` docs](genqlient.yaml) for more.
- genqlient now generates slices containing all enum values for each enum type.
- genqlient now returns `Is`/`As`-able errors when the HTTP request returns a non-200 status.

### Bug fixes:

- omitempty validation:
  - allow `omitempty` on non-nullable input field, if the field has a default
  - allow `omitempty: false` on an input field, even when it is non-nullable
- don't do `omitempty` and `pointer` input types validation when `use_struct_reference` is used, as the generated type is often not compatible with validation logic.
- the `allow_broken_features` option, which no longer did anything, has been removed

## v0.7.0

In addition to several new features and bugfixes, along with this release comes reorganized [documentation](.) for genqlient. Note that genqlient now requires Go 1.20 or higher, and is tested through Go 1.22.

### New features:

- The new `optional: generic` allows using a generic type to represent optionality. See the [documentation](genqlient.yaml) for details.
- The new `optional: pointer_omitempty` allows using a pointer that is also annotated with `omitempty`. See the [documentation](genqlient.yaml) for details.
- For schemas with enum values that differ only in casing, it's now possible to disable smart-casing in genqlient.yaml; see the [documentation](genqlient.yaml) for `casing` for details.
- genqlient now supports .graphqls and .gql file extensions for schemas and queries.
- More accurately guess the package name for generated code (and warn if the config option -- now almost never needed -- looks wrong).

### Bug fixes:
- Negative pointer directives, i.e., `# @genqlient(pointer: false)` are now respected even in the when `optional: pointer` is set in the configuration file.
- Made name collisions between query/mutation arguments and local function variables less likely.
- Fix generation issue related to golang type implementation of complex graphql union fragments.
- Bind correctly to types in the same package as the generated code.
- Fix non-deterministic generated code when querying graphql interfaces via named fragments.

## v0.6.0

Version 0.6.0 includes some small features and bugfixes. Note that genqlient now requires Go 1.18 or higher, and is tested through Go 1.20.

### New features:

- You can now bind all types from a package in `genqlient.yaml` using the new `package_bindings` option.
- The graphql operation (query or mutation) as sent over the wire is now exposed via a constant in the generated genqlient .go file.

### Bug fixes:

- Fixed non-deterministic generated code when querying graphql interfaces.
- Fixed generated code when last component of package name is not a valid identifier (e.g. `"path/to/my-package"`).
- Fixed incorrect documentation of `for` directive.
- Fixed bug where `omitempty` JSON tags were not being correctly applied to `__premarshal` structs.

## v0.5.0

Version 0.5.0 adds several new configuration options and convenience features. Note that genqlient now requires Go 1.16 or higher, and is tested through Go 1.18.

### Breaking changes:

- The [`graphql.Client`](https://pkg.go.dev/github.com/Khan/genqlient/graphql#Client) interface now accepts two structs for the request and response, to allow future expansion, rather than several individual arguments.  Clients implementing the interface themselves will need to change the signature; clients who simply call `graphql.NewClient` are unaffected.

### New features:

- genqlient can now run as a portable binary (i.e. without a local checkout of the repository or `go run`).
- You can now enable `use_extensions` in the configuration file, to receive extensions returned by the GraphQL API server. Generated functions will return extensions as `map[string]interface{}`, if enabled.
- You can now use `graphql.NewClientUsingGet` to create a client that uses query parameters to pass the query to the GraphQL API server.
- In config files, `schema`, `operations`, and `generated` can now be absolute paths.
- You can now configure how nullable types are mapped to Go types in the configuration file. Specifically, you can set `optional: pointer` to have all nullable GraphQL arguments, input fields, and output fields map to pointers.

### Bug fixes:

- genqlient now explicitly rejects argument, operation, and type names which are Go keywords, rather than failing with an opaque error.
- genqlient now gives better error messages if it thinks your schema is invalid.

## v0.4.0

Version 0.4.0 adds several new configuration options, as well as additional methods to simplify the use of interfaces.

### Breaking changes:

- The `Config` fields `Schema` and `Operations` are now both of type `StringList`.  This does not affect configuration via `genqlient.yaml`, only via the Go API.
- The `typename` and `bind` options may no longer be combined; doing so will now result in an error.  In practice, any such use was likely in error (and the rules for which would win were confusing and undocumented).

### New features:

- genqlient now generates getter methods for all fields, even those which do not implement a genqlient-generated interface; this can be useful for callers who wish to define their own interface and have several unrelated genqlient types which have the same fields implement it.
- The new `struct_references` option automatically sets the `pointer` and `omitempty` options on fields of struct type; see the [`genqlient.yaml` documentation](genqlient.yaml) for details.
- genqlient config now accepts either a single or multiple files (or globs) for the `schema` and `operations` fields (previously it accepted only one `schema`, and required a list of `operations` files).
- genqlient now looks for its config file as `[.]genqlient.y[a]ml` in any ancestor directory, if unspecified, rather than only as `genqlient.yaml` in the current directory.
- The `typename` option can now be used on basic types (string, int, etc) as well as structs; this can be useful to have genqlient define new types like `type Language string` and use that type for specified fields.

### Bug fixes:

- In certain very rare cases involving duplicate fields in fragment spreads, genqlient would generate code that failed to compile due to duplicate methods not getting promoted; genqlient now generates correct types.  (See #126 for a more complete description.)
- genqlient no longer rejects schemas which include the implicitly declared types (`scalar String`, etc.) explicitly; this makes it easier to use schemas generate via introspection.

## v0.3.0

Version 0.3.0 adds several new configuration options, allowing simplification of generated types and configuration of input types, as well as marshalers for all genqlient-generated types.

### Breaking changes:

- Previously, `# @genqlient` directives applied to entire operations applied inconsistently to fields of input types used by those operations.  Specifically, `pointer: true`, when applied to the operation, would affect all input-field arguments, but `omitempty: true` would not.  Now, all options apply to fields of input types; this is a behavior change in the case of `omitempty`.

### New features:

- genqlient's types are now safe to JSON-marshal, which can be useful for putting them in a cache, for example.  See the [docs](client_config.md#marshaling) for details.
- The new `flatten` option in the `# @genqlient` directive allows for a simpler form of type-sharing using fragment spreads.  See the [docs](operations.md#sharing-types) for details.
- The new `for` option in the `# @genqlient` directive allows applying options to a particular field anywhere it appears in the query.  This is especially useful for fields of input types, for which there is otherwise no way to specify options; see the [documentation on handling nullable fields](operations.md#nullable-fields) for an example, and the [`# @genqlient` directive reference](genqlient_directive.graphql) for the full details.

### Bug fixes:

## v0.2.0

Version 0.2.0 adds several convenience features for using custom scalars, as well as many internal improvements and bug fixes.

### Breaking changes:

- The [`graphql.Client`](https://pkg.go.dev/github.com/Khan/genqlient/graphql#Client) interface now accepts `variables interface{}` (containing a JSON-marshalable value) rather than `variables map[string]interface{}`.  Clients implementing the interface themselves will need to change the signature; clients who simply call `graphql.NewClient` are unaffected.
- genqlient's handling of the `omitempty` option has changed to match that of `encoding/json`, from which it had inadvertently differed.  In particular, this means struct-typed arguments with `# @genqlient(omitempty: true)` will no longer be omitted if they are the zero value.  (Struct-pointers are still omitted if nil, so adding `pointer: true` will typically work fine.  It's also now possible to use a custom marshaler to explicitly map zero to null.)

### New features:

- The new `bindings.marshaler` and `bindings.unmarshaler` options in `genqlient.yaml` allow binding to a type without using its standard JSON serialization; see the [documentation](genqlient.yaml) for details.
- Multiple genqlient directives may now be applied to the same node, as long as they don't conflict; see the [directive documentation](genqlient_directive.graphql) for details.

### Bug fixes:

- The `omitempty` option now works correctly for struct- and map-typed variables, matching `encoding/json`, which is to say it never omits structs, and omits empty maps. (#43)
- Generated type-names now abbreviate across multiple components; for example if the path to a type is `(MyOperation, Outer, Outer, Inner, OuterInner)`, it will again be called `MyOperationOuterInner`.  (This regressed in a pre-v0.1.0 refactor.) (#109)
- Previously, interface fields with `# @genqlient(pointer: true)` would be unmarshaled to `(*MyInterface)(*<nil>)`, i.e. a pointer to the untyped-nil of the interface type.  Now they are unmarshaled as `(*MyInterface)(<nil>)`, i.e. a nil pointer of the pointer-to-interface type, as you would expect.

## v0.1.0

First open-sourced version.
