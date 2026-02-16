BUILDER=./.builder
RULES=go
GOEXPERIMENT=jsonv2
include $(BUILDER)/rules.mk
$(BUILDER)/rules.mk:
	-go run endobit.io/builder@latest init

build::
	$(GO_BUILD) -o wifire ./cmd

clean::
	-rm wifire
