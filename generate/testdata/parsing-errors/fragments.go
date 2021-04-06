package parsing_errors

var _ = `# @genqlient
	query myBadQuery(varMissingDollar: String) {
	  field(arg: $varMissingDollar)
	}
`
