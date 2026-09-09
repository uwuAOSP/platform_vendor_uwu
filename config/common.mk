# Device trees must declare these capabilities before inheriting this file:
#   UWU_DEVICE_TYPE := phone | tablet | foldable
#   UWU_SUPPORTS_TELEPHONY := true | false
ifeq ($(strip $(UWU_DEVICE_TYPE)),)
$(error UWU_DEVICE_TYPE must be set to phone, tablet, or foldable)
endif
ifneq ($(words $(UWU_DEVICE_TYPE)),1)
$(error UWU_DEVICE_TYPE must contain exactly one value)
endif
ifeq ($(filter $(UWU_DEVICE_TYPE),phone tablet foldable),)
$(error Invalid UWU_DEVICE_TYPE: $(UWU_DEVICE_TYPE))
endif

ifeq ($(strip $(UWU_SUPPORTS_TELEPHONY)),)
$(error UWU_SUPPORTS_TELEPHONY must be set to true or false)
endif
ifneq ($(words $(UWU_SUPPORTS_TELEPHONY)),1)
$(error UWU_SUPPORTS_TELEPHONY must contain exactly one value)
endif
ifeq ($(filter $(UWU_SUPPORTS_TELEPHONY),true false),)
$(error Invalid UWU_SUPPORTS_TELEPHONY: $(UWU_SUPPORTS_TELEPHONY))
endif

WITH_GMS_COMMS_SUITE := $(UWU_SUPPORTS_TELEPHONY)

# Allow vendor/extra to override any property by setting it first
$(call inherit-product-if-exists, vendor/extra/product.mk)

# uwuAOSP components
PRODUCT_PACKAGES += \
    uwuSettingsExt \
    LyricFetchExt \
    uwuClock \
    CatShare \
    uwuAICore \
    uwuPrism

# Platform UI sounds not provided by Pixel Sounds
PRODUCT_COPY_FILES += \
    frameworks/base/data/sounds/effects/ogg/KeypressInvalid.ogg:$(TARGET_COPY_OUT_PRODUCT)/media/audio/ui/KeypressInvalid.ogg \
    frameworks/base/data/sounds/effects/ogg/Trusted.ogg:$(TARGET_COPY_OUT_PRODUCT)/media/audio/ui/Trusted.ogg

# Mobile apps
PRODUCT_PACKAGES += \
    Launcher3Overlay

PRODUCT_DEXPREOPT_SPEED_APPS += \
    Launcher3QuickStep

ifneq ($(PRODUCT_NO_CAMERA),true)
PRODUCT_PACKAGES += \
    Aperture
endif

# Charger animation
PRODUCT_PACKAGES += \
    pixel_charger_animation \
    pixel_charger_animation_vendor

# Media
PRODUCT_PRODUCT_PROPERTIES += \
    media.recorder.show_manufacturer_and_model=true

# TextClassifier
PRODUCT_PACKAGES += \
    libtextclassifier_annotator_en_model \
    libtextclassifier_annotator_universal_model \
    libtextclassifier_actions_suggestions_universal_model \
    libtextclassifier_lang_id_model

PRODUCT_ARTIFACT_PATH_REQUIREMENT_ALLOWED_LIST += \
    system/etc/textclassifier/actions_suggestions.universal.model \
    system/etc/textclassifier/lang_id.model \
    system/etc/textclassifier/textclassifier.en.model \
    system/etc/textclassifier/textclassifier.universal.model

# Themes
PRODUCT_PACKAGES += \
    ThemesStub

# Exclude repos from bp scanning
PRODUCT_SOURCE_ROOT_DIRS += -kernel/platform
PRODUCT_SOURCE_ROOT_DIRS += -prebuilts/misc/protobuf_vendorcompat

# Call recording
ifeq ($(WITH_GMS_COMMS_SUITE),true)
PRODUCT_COPY_FILES += \
    vendor/uwu/config/permissions/com.google.android.apps.dialer.call_recording_audio.features.xml:$(TARGET_COPY_OUT_PRODUCT)/etc/permissions/com.google.android.apps.dialer.call_recording_audio.features.xml
endif

ifeq ($(PRODUCT_GMS_CLIENTID_BASE),)
PRODUCT_PRODUCT_PROPERTIES += \
    ro.com.google.clientidbase=android-google
else
PRODUCT_PRODUCT_PROPERTIES += \
    ro.com.google.clientidbase=$(PRODUCT_GMS_CLIENTID_BASE)
endif

ifneq ($(TARGET_BUILD_VARIANT),user)
# Disable ADB authentication
PRODUCT_SYSTEM_EXT_PROPERTIES += ro.adb.secure=0
endif

# Disable extra StrictMode features
PRODUCT_PRODUCT_PROPERTIES += persist.sys.strictmode.disable=true

# uwuAOSP init rc file
PRODUCT_COPY_FILES += \
    vendor/uwu/prebuilt/common/etc/init/init.uwu-system_ext.rc:$(TARGET_COPY_OUT_SYSTEM_EXT)/etc/init/init.uwu-system_ext.rc

# Enable SIP+VoIP on all targets
PRODUCT_COPY_FILES += \
    frameworks/native/data/etc/android.software.sip.voip.xml:$(TARGET_COPY_OUT_PRODUCT)/etc/permissions/android.software.sip.voip.xml

# Enable wireless Xbox 360 controller support
PRODUCT_COPY_FILES += \
    frameworks/base/data/keyboards/Vendor_045e_Product_028e.kl:$(TARGET_COPY_OUT_PRODUCT)/usr/keylayout/Vendor_045e_Product_0719.kl

# Component overrides
PRODUCT_PACKAGES += \
    uwu-component-overrides.xml

# Enforce privapp-permissions whitelist
PRODUCT_PRODUCT_PROPERTIES += \
    ro.control_privapp_permissions=enforce

# Do not include art debug targets
PRODUCT_ART_TARGET_INCLUDE_DEBUG_BUILD := false

# Strip the local variable table and the local variable type table to reduce
# the size of the system image. This has no bearing on stack traces, but will
# leave less information available via JDWP.
PRODUCT_MINIMIZE_JAVA_DEBUG_INFO := true

# Enable whole-program R8 Java optimizations for SystemUI and system_server,
# but also allow explicit overriding for testing and development.
SYSTEM_OPTIMIZE_JAVA ?= true
SYSTEMUI_OPTIMIZE_JAVA ?= true

# Disable vendor restrictions
PRODUCT_RESTRICT_VENDOR_FILES := false

ifneq ($(TARGET_DISABLE_EPPE),true)
# Require all requested packages to exist
$(call enforce-product-packages-exist-internal,$(lastword $(_include_stack)),product_manifest.xml rild Calendar android.hidl.memory@1.0-impl.vendor vndk_apex_snapshot_package)
endif

# Bootanimation
ifeq ($(strip $(TARGET_SCREEN_WIDTH)),)
    $(warning "TARGET_SCREEN_WIDTH is undefined, assuming 1080p")
else
    $(call soong_config_set,vendor_pixel,bootanimation_res,$(TARGET_SCREEN_WIDTH))
endif

PRODUCT_PACKAGES += \
    bootanimation_pixel

# Face Unlock
TARGET_FACE_UNLOCK_SUPPORTED ?= true

ifeq ($(TARGET_FACE_UNLOCK_SUPPORTED),true)
PRODUCT_PACKAGES += \
    ParanoidSense

PRODUCT_SYSTEM_EXT_PROPERTIES += \
    ro.face.sense_service=true

PRODUCT_COPY_FILES += \
    frameworks/native/data/etc/android.hardware.biometrics.face.xml:$(TARGET_COPY_OUT_SYSTEM)/etc/permissions/android.hardware.biometrics.face.xml
endif

# Pixel compatibility resources
PRODUCT_COPY_FILES += \
    vendor/uwu/config/permissions/privapp-permissions-lineagehw.xml:$(TARGET_COPY_OUT_SYSTEM_EXT)/etc/permissions/privapp-permissions-lineagehw.xml \
    vendor/uwu/prebuilt/common/etc/sysconfig/pixel_2016_exclusive.xml:$(TARGET_COPY_OUT_PRODUCT)/etc/sysconfig/pixel_2016_exclusive.xml

PRODUCT_PACKAGE_OVERLAYS += \
    vendor/uwu/overlay/device-config

$(call inherit-product, vendor/uwu/config/extra_tools.mk)

# GMS
include vendor/uwu/config/pixel.mk

# Overlay
PRODUCT_PACKAGES += \
    FrameworkOverlayUwU \
    SettingsOverlayUwU

ifeq ($(WITH_GMS_COMMS_SUITE),true)
PRODUCT_PACKAGES += \
    GoogleDialerOverlayUwU
endif

# OverlayFS
PRODUCT_PACKAGES_DEBUG += \
    disable-overlays

# Storage manager
PRODUCT_PRODUCT_PROPERTIES += \
    ro.storage_manager.enabled=true

# Root
ifneq ($(TARGET_BUILD_VARIANT),user)
ifeq ($(WITH_SU),true)
PRODUCT_PACKAGES += \
    su

PRODUCT_ARTIFACT_PATH_REQUIREMENT_ALLOWED_LIST += \
    system/xbin/su
endif
endif

# SystemUI
PRODUCT_DEXPREOPT_SPEED_APPS += \
    SystemUI

PRODUCT_PRODUCT_PROPERTIES += \
    dalvik.vm.systemuicompilerfilter=speed

ifeq ($(TARGET_BUILD_VARIANT),userdebug)
PRODUCT_PRODUCT_PROPERTIES += \
    debug.sf.enable_transaction_tracing=false
endif

# SetupWizard
PRODUCT_ENFORCE_RRO_EXCLUDED_OVERLAYS += vendor/uwu/overlay/no-rro
PRODUCT_PACKAGE_OVERLAYS += \
    vendor/uwu/overlay/common \
    vendor/uwu/overlay/no-rro

PRODUCT_PACKAGES += \
    NetworkStackOverlay \
    PermissionControllerOverlay

# Form factor
ifeq ($(UWU_DEVICE_TYPE),phone)
PRODUCT_PRODUCT_PROPERTIES += \
    ro.support_one_handed_mode?=true
else ifeq ($(UWU_DEVICE_TYPE),tablet)
TARGET_IS_TABLET := true
$(call inherit-product, vendor/uwu/config/large_screen.mk)
else ifeq ($(UWU_DEVICE_TYPE),foldable)
PRODUCT_PRODUCT_PROPERTIES += \
    ro.support_one_handed_mode?=true
$(call inherit-product, vendor/uwu/config/large_screen.mk)
endif

# Connectivity
ifeq ($(UWU_SUPPORTS_TELEPHONY),true)
$(call inherit-product, vendor/uwu/config/telephony.mk)
else
PRODUCT_PACKAGES += \
    EmergencyInfo
PRODUCT_PACKAGE_OVERLAYS += vendor/uwu/overlay/wifionly
endif

include vendor/uwu/config/version.mk

-include vendor/uwu-priv/keys/keys.mk

-include $(WORKSPACE)/build_env/image-auto-bits.mk
