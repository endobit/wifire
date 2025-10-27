BUILDER=./.builder
RULES=go
include $(BUILDER)/rules.mk
$(BUILDER)/rules.mk:
	-go run endobit.io/builder@latest init

build::
	GOEXPERIMENT=jsonv2 $(GO_BUILD) -o wifire ./cmd

clean::
	-rm wifire
