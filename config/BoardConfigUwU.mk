# SPDX-FileCopyrightText: 2017-2024 The LineageOS Project
# SPDX-License-Identifier: Apache-2.0

# Recovery
BOARD_USES_FULL_RECOVERY_IMAGE ?= true

# Framework compatibility matrix
DEVICE_FRAMEWORK_COMPATIBILITY_MATRIX_FILE += vendor/uwu/config/framework_compatibility_matrix.xml

ifneq ($(BOARD_USES_SOONG_KERNEL),true)
include vendor/uwu/config/BoardConfigKernel.mk
endif

ifeq ($(BOARD_USES_QCOM_HARDWARE),true)
    include hardware/qcom-caf/common/BoardConfigQcom.mk
endif

include vendor/uwu/config/BoardConfigSoong.mk
