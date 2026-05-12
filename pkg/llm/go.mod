module github.com/amr/naqb/pkg/llm

go 1.26.3

require github.com/amr/naqb/pkg/config v0.0.0

require (
	github.com/amr/naqb/pkg/log v0.0.0
	github.com/anthropics/anthropic-sdk-go v1.37.0
	github.com/aws/aws-sdk-go-v2 v1.41.7
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16
	github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.50.4
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/smithy-go v1.25.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/sync v0.20.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/amr/naqb/pkg/config => ../config
	github.com/amr/naqb/pkg/log => ../log
)
