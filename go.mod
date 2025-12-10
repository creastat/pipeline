module github.com/creastat/pipeline

go 1.25.5

require (
	github.com/creastat/infra v0.0.0
	github.com/creastat/providers v0.0.0
	github.com/creastat/storage v0.0.0
	github.com/gorilla/websocket v1.5.3
	pgregory.net/rapid v1.1.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.33.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	golang.org/x/sys v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/creastat/infra => ../infra
	github.com/creastat/providers => ../providers
	github.com/creastat/storage => ../storage
)
