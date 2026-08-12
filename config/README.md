# Product configuration

Device products use a single configuration entry point. Declare the
device capabilities before inheriting it:

```make
UWU_DEVICE_TYPE := phone
UWU_SUPPORTS_TELEPHONY := true
$(call inherit-product, vendor/uwu/config/common.mk)
```

Both variables are required. Product configuration fails when either variable
is missing or has an unsupported value.

## Form factors

`UWU_DEVICE_TYPE` accepts one of:

- `phone`: enables one-handed mode support.
- `tablet`: enables tablet and large-screen features.
- `foldable`: enables one-handed mode and large-screen features.

## Connectivity

`UWU_SUPPORTS_TELEPHONY` accepts one of:

- `true`: includes telephony packages, APNs, messaging, and mobile-data
  properties.
- `false`: disables the GMS communications suite and applies Wi-Fi-only
  packages and overlays.

Device trees should inherit the appropriate AOSP base product separately. For
example, a telephony device normally inherits `full_base_telephony.mk` before
the uwuAOSP configuration.
