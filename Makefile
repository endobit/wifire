RULESDIR=../builder
RULES=go
include $(RULESDIR)/rules.mk

build::
	go build $(GO_LDFLAGS) -o wifire ./cmd

clean::
	rm -f wifire
