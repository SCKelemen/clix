module multicli

go 1.27.0

require github.com/SCKelemen/clix/v2 v2.4.1

require (
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

replace github.com/SCKelemen/clix/v2 => ../..
