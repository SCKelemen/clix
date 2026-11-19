module multicli

go 1.25.0

require github.com/SCKelemen/clix/v2 v2.4.1

require (
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.41.0 // indirect
)

replace github.com/SCKelemen/clix/v2 => ../..
