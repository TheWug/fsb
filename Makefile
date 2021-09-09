prefix = /usr/local

all: fsb

.PHONY: fsb test

fsb: var
	go build ./daemon/fsb

test: var
	go test ./...

clean:
	git clean -fd

installall: installexec installconf

installexec: fsb
	install fsb $(DESTDIR)$(prefix)/bin/fsb
	install -D single.sh $(DESTDIR)$(prefix)/bin/fsb-util/convert-script.sh

installconf:
	mkdir -p $(DESTDIR)/etc/fsb
	install -D -o nobody -g nogroup -m 600 config/settings.json $(DESTDIR)/etc/fsb/settings.json
	install config/fsb.service $(DESTDIR)/etc/systemd/system/fsb.service

var:
	mkdir var
