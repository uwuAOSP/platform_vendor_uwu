PRODUCT_VERSION_MAJOR = 17
PRODUCT_VERSION_MINOR = 0

ifeq ($(UWU_VERSION_APPEND_TIME_OF_DAY),true)
    UWU_BUILD_DATE := $(shell date -u +%Y%m%d_%H%M%S)
else
    UWU_BUILD_DATE := $(shell date -u +%Y%m%d)
endif

# Set UWU_BUILDTYPE from the env RELEASE_TYPE, for jenkins compat

ifndef UWU_BUILDTYPE
    ifdef RELEASE_TYPE
        # Starting with "UWU_" is optional
        RELEASE_TYPE := $(shell echo $(RELEASE_TYPE) | sed -e 's|^UWU_||g')
        UWU_BUILDTYPE := $(RELEASE_TYPE)
    endif
endif

# Filter out random types, so it'll reset to UNOFFICIAL
ifeq ($(filter RELEASE NIGHTLY SNAPSHOT EXPERIMENTAL,$(UWU_BUILDTYPE)),)
    UWU_BUILDTYPE := UNOFFICIAL
    UWU_EXTRAVERSION :=
endif

ifeq ($(UWU_BUILDTYPE), UNOFFICIAL)
    ifneq ($(TARGET_UNOFFICIAL_BUILD_ID),)
        UWU_EXTRAVERSION := -$(TARGET_UNOFFICIAL_BUILD_ID)
    endif
endif

UWU_VERSION_SUFFIX := $(UWU_BUILD_DATE)-$(UWU_BUILDTYPE)$(UWU_EXTRAVERSION)-$(UWU_BUILD)

# Internal version
UWU_VERSION := $(PRODUCT_VERSION_MAJOR).$(PRODUCT_VERSION_MINOR)-$(UWU_VERSION_SUFFIX)

# Display version
UWU_DISPLAY_VERSION := $(PRODUCT_VERSION_MAJOR)-$(UWU_VERSION_SUFFIX)

# uwuAOSP version properties
PRODUCT_PRODUCT_PROPERTIES += \
    ro.uwu.version=$(UWU_VERSION) \
    ro.uwu.display.version=$(UWU_DISPLAY_VERSION) \
    ro.uwu.build.version=$(PRODUCT_VERSION_MAJOR).$(PRODUCT_VERSION_MINOR) \
    ro.uwu.releasetype=$(UWU_BUILDTYPE) \
    ro.uwu.device=$(UWU_BUILD)

ifneq ($(strip $(UWU_MAINTAINER)),)
PRODUCT_PRODUCT_PROPERTIES += \
    ro.uwu.maintainer=$(strip $(UWU_MAINTAINER))
endif
