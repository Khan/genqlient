Generate the schemas by getting a token from [GitHub](https://github.com/settings/tokens/new) (no scopes needed), then:
```
npm install -g graphql-introspection-json-to-sdl
curl -H "Authorization: bearer <your token>" https://api.github.com/graphql >example/schema.json
graphql-introspection-json-to-sdl example/schema.json >example/schema.graphql
```
TODO: something better
