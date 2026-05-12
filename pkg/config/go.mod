module github.com/amr/naqb/pkg/config

go 1.26.3

require github.com/amr/naqb/pkg/log v0.0.0

require gopkg.in/yaml.v3 v3.0.1

require (
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/amr/naqb/pkg/log => ../log
