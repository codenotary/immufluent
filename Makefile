immufluent: *.go go.*
	CGO_ENABLED=0 go build -o $@