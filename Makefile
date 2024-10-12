GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Names of the binaries to be built

# Default target

# Build the binaries
build:
	go build -o mp1_node eventordering.go

