### VDED
# vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

PKGS= \
	--pkg gee-1.0 \
	--pkg libsoup-2.4 \
	--pkg json-glib-1.0 \
	--pkg posix

DAEMON_SOURCES= \
	vded.vala

all: clean vded

clean:
	rm -f vded

vded:
	@echo "Building $@ ... "
	@valac $(PKGS) -o $@ $(DAEMON_SOURCES)

