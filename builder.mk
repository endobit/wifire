RULESDIR=.builder
include $(RULESDIR)/rules.mk

$(RULESDIR):
	git clone https://github.com/endobit/builder.git $@

$(RULESDIR)/rules.mk: $(RULESDIR)

nuke::
	rm -rf $(RULESDIR)


