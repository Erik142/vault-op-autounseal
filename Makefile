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
	go build -v -o $(TOP)/vault-onepassword-controller $(TOP)/cmd/autounseal.go

.PHONY: version_bump
version_bump:
	$(Q)$(TOP)/scripts/bump_deployment_version.sh
	$(Q)$(TOP)/scripts/get_next_version.sh
