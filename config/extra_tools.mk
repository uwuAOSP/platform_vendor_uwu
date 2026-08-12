# Shell tools
PRODUCT_PACKAGES += \
    bash \
    curl \
    htop \
    nano \
    nano_recovery \
    vim

PRODUCT_ARTIFACT_PATH_REQUIREMENT_ALLOWED_LIST += \
    system/bin/curl \
    system/%/libzstd.so

# fastbootd
ifneq ($(TARGET_DISABLE_FASTBOOTD),true)
PRODUCT_PACKAGES += \
    fastbootd
endif

# OpenSSH
PRODUCT_PACKAGES_DEBUG += \
    scp \
    sftp \
    ssh \
    sshd \
    sshd_config \
    ssh-keygen \
    start-ssh

ifneq ($(TARGET_BUILD_VARIANT),user)
PRODUCT_COPY_FILES += \
    vendor/uwu/prebuilt/common/etc/init/init.openssh.rc:$(TARGET_COPY_OUT_PRODUCT)/etc/init/init.openssh.rc
endif

# File and process utilities
PRODUCT_PACKAGES += \
    rsync \
    unrar \
    zstd

PRODUCT_PACKAGES_DEBUG += \
    adb_root \
    getcap \
    setcap \
    procmem

ifneq ($(TARGET_BUILD_VARIANT),user)
PRODUCT_ARTIFACT_PATH_REQUIREMENT_ALLOWED_LIST += \
    system/bin/getcap \
    system/bin/setcap \
    system/bin/procmem
endif
