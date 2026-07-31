# Inherit mobile full common Lineage stuff
$(call inherit-product, vendor/uwu/config/common_mobile_full.mk)

# Inherit tablet common Lineage stuff
$(call inherit-product, vendor/uwu/config/tablet.mk)

$(call inherit-product, vendor/uwu/config/wifionly.mk)
