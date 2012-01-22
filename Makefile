### VDED
# vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

PKGS= \
	--pkg gee-1.0 \
	--pkg libsoup-2.4 \
	--pkg json-glib-1.0 \
	--pkg posix

DAEMON_SOURCES= \
	vded.vala \
	gmetric.vala

CLIENT_SOURCES= \
	vde-client.vala

all: clean vded vde-client

clean:
	rm -f vded vde-client

vded:
	@echo "Building $@ ... "
	@valac $(PKGS) -o $@ $(DAEMON_SOURCES)

vde-client:
	@echo "Building $@ ... "
	@valac $(PKGS) -o $@ $(CLIENT_SOURCES)

vded.spec:
	sed -e "s/@VERSION@/`cat VERSION`/;" vded.spec.in > $@

