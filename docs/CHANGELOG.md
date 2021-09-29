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
