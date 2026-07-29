# Inherit mobile full common Lineage stuff
$(call inherit-product, vendor/uwu/config/common_mobile_full.mk)

# Define tablet-specific variables
TARGET_IS_TABLET := true
WITH_GMS_COMMS_SUITE := false

# Inherit tablet common Lineage stuff
$(call inherit-product, vendor/uwu/config/tablet.mk)

$(call inherit-product, vendor/uwu/config/wifionly.mk)
