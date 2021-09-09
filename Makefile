prefix = /usr/local
user = fsb

all: fsb

.PHONY: build test

build: var
	go build ./daemon/fsb

test: var
	go test ./...

clean:
	git clean -fd

installall: installsetup installexec installconf installservice

installexec: fsb
	install fsb $(DESTDIR)$(prefix)/bin/fsb
	install -D single.sh $(DESTDIR)$(prefix)/bin/fsb-util/convert-script.sh

installconf:
	mkdir -p $(DESTDIR)/etc/fsb
	install -D -o fsb -g fsb -m 600 config/settings.json $(DESTDIR)/etc/fsb/settings.json

installservice:
	install config/fsb.service $(DESTDIR)/etc/systemd/system/fsb.service

installsetup:
	if id -u fsb; then :; else adduser --system --no-create-home --home /var/fsb --group --disabled-login $(user); fi;

var:
	mkdir var
