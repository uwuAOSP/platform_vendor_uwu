# SPDX-FileCopyrightText: 2024 The LineageOS Project
# SPDX-License-Identifier: Apache-2.0

$(call inherit-product, device/google/cuttlefish/vsoc_x86_64/tv/aosp_cf.mk)

include vendor/uwu/build/target/product/uwu_generic_tv_target.mk

TARGET_DISABLE_EPPE := true
TARGET_NO_KERNEL_OVERRIDE := true

# Overrides
PRODUCT_NAME := uwu_cf_tv_x86_64
PRODUCT_MODEL := uwuAOSP Cuttlefish TV built for x86_64
