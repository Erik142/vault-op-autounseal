export TOP := $(realpath $(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

ifeq (VERBOSE,TRUE)
	Q := $(empty)
else
	Q := @
endif

all: build

.PHONY: build
build:
	go mod download
	go build -v -o ./vault-op-autounseal

.PHONY: version_bump
version_bump:
	$(Q)$(TOP)/scripts/get_next_version.sh
