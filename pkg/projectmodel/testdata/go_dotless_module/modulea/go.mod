module modulea

go 1.25

require (
	github.com/external/pkg v1.2.3
	github.com/replaced/pkg v1.0.0
	github.com/excluded/pkg v1.0.0
)

replace github.com/replaced/pkg => github.com/replaced/pkg v1.0.1

exclude github.com/excluded/pkg v1.0.0
