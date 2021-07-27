module github.com/thewug/fsb

go 1.16

replace github.com/thewug/dml => ./github.com/thewug/dml
replace github.com/thewug/gogram => ./github.com/thewug/gogram
replace github.com/thewug/reqtify => ./github.com/thewug/reqtify

require (
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/lib/pq v1.10.2
	github.com/thewug/dml v0.0.0-20210725230048-b9a5991ea54a
	github.com/thewug/gogram v1.0.0
	github.com/thewug/reqtify v0.0.0-20201109043318-8ad07a73cbbc
)
