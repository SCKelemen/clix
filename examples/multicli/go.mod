module multicli

go 1.24.0

toolchain go1.24.2

require clix v0.0.0

require (
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
)

replace clix => ../..
