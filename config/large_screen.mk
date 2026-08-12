$(call inherit-product, $(SRC_TARGET_DIR)/product/large_screen_common.mk)

# Freeform window management
PRODUCT_COPY_FILES += \
    frameworks/native/data/etc/android.software.freeform_window_management.xml:$(TARGET_COPY_OUT_PRODUCT)/etc/permissions/android.software.freeform_window_management.xml

# Large-screen overlays
PRODUCT_PACKAGE_OVERLAYS += \
    vendor/uwu/overlay/tablet \
    vendor/uwu/overlay/tablet-desktop
