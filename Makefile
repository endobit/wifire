BUILDER=./.builder
RULES=go
include $(BUILDER)/rules.mk
$(BUILDER)/rules.mk:
	-go run github.com/endobit/builder@latest init

build::
	$(GO_BUILD) -o wifire ./cmd

clean::
	-rm wifire
