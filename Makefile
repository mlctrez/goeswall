BINARY := goeswall
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install clean

build:
	go build -o $(BINARY) .

install: build
	install -d $(INSTALL_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)

clean:
	rm -f $(BINARY)
