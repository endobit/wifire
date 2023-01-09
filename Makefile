RULES=go
include builder.mk

build::
	go build $(GO_LDFLAGS) -o wifire ./cmd

clean::
	rm -f wifire
