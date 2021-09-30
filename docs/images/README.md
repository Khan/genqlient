# genqlient logo

`genqlient-orig.svg` is the original logo, in editable SVG.  To create `genqlient.svg`, I:
- converted all text to paths
- added media queries to make the paths white if the user is in dark mode (see issue #17)

For the social preview image (in GitHub settings), I ran:
```sh
convert -resize 2560x1280 -density 600 -gravity center \
  -extent 2560x1280 docs/images/genqlient.{svg,png}
```
