# build glue for widgetd
VERSION := 1.4.2
TARBALL  = widgetd-$(VERSION).tar.gz

all: build

build:
	$(CC) -DWIDGETD_VERSION=\"$(VERSION)\" -o widgetd main.c

dist: build
	tar czf $(TARBALL) widgetd README.md
