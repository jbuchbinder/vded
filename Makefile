# VDED - Vector Delta Engine Daemon
# https://github.com/jbuchbinder/vded
#
# vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

VERSION=$(shell cat VERSION)

dist: tar

tar:
	rm -rf vded-$(VERSION)
	git clone git://github.com/jbuchbinder/vded.git vded-$(VERSION)
	rm vded-$(VERSION)/.git -rf
	cat vded-$(VERSION)/vded.spec.in | sed -e "s/@VERSION@/$(VERSION)/g" > vded-$(VERSION)/vded.spec
	tar czvf vded-$(VERSION).tar.gz vded-$(VERSION)

rpm: tar
	sudo rpmbuild -ta vded-$(VERSION).tar.gz
	rm -rf vded-$(VERSION)

