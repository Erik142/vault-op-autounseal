export TOP := $(realpath $(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

ifeq (VERBOSE,TRUE)
	Q := $(empty)
else
	Q := @
endif

.PHONY: version_bump
version_bump:
	$(Q)$(TOP)/scripts/get_next_version.sh
