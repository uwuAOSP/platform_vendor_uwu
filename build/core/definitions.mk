# Include board/platform macros
include vendor/uwu/build/core/utils.mk

ifneq ($(strip $(AB_OTA_RADIO_PARTITIONS)),)
INSTALLED_RADIOIMAGE_TARGET += $(addprefix $(PRODUCT_OUT)/,$(addsuffix .img,$(AB_OTA_RADIO_PARTITIONS)))
endif
