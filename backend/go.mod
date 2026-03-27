module github.com/aconiq/backend

go 1.25.0

require (
	github.com/Dadido3/go-typst v0.10.0
	github.com/MeKo-Christian/go-overpass v0.0.0-20251224010608-5fb9afa66cb9
	github.com/gogama/flatgeobuf v1.0.0
	github.com/google/flatbuffers v23.5.26+incompatible
	github.com/spf13/cobra v1.10.2
	modernc.org/sqlite v1.46.1
)

require github.com/wroge/wgs84 v1.1.7

require (
	github.com/meko-tech/go-absolute-database v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/cwbudde/go-citygml v0.1.0
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/smasher164/xid v0.1.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.35.0
	modernc.org/libc v1.67.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace github.com/cwbudde/go-citygml => ../../go-citygml

replace github.com/meko-tech/go-absolute-database => ../../go-absolute-database
