package parsing

const MyFragment = `
	# @genqlient
	fragment MyFragment on MyType {
		myFragmentField
		...NestedFragment
	}
`

var _ = `
	# @genqlient
	fragment NestedFragment on MyType {
		myOtherFragmentField
	}
`

const MyQuery = `
	# @genqlient
	query MyQuery {
		myField
		myOtherField {
		...MyFragment
		}
	}
`

func query(s string) {}

func MyMutation() {
	query(`
		# @genqlient
		mutation MyMutation {
			myField
			myOtherField {
			...MyFragment
			}
		}
	`)
}

const (
	NotAString = 1
	NotAQuery  = `query
		writing with GraphQL is fun!`
)
