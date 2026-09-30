<!--
SPDX-FileCopyrightText: The uwuAOSP Project
SPDX-License-Identifier: Apache-2.0
-->

# uwuAOSP Expressive boot animation

Six approved shapes, 650 ms per target, stiffness 200, damping ratio 0.6,
50 degrees continuous rotation plus 90 degrees spring rotation per target.
Motion parameters follow Google's Material Components loading indicator.
Contours in `shapes.json` are traced from the supplied artwork; no wallpaper is bundled.

The native AOSP `dynamic_colors` shader reads `persist.bootanim.color1` through `color4`.
SystemUI's legacy theme controller and the Android theme service publish the same Monet
palettes used by Compose `dynamicLightColorScheme` / `dynamicDarkColorScheme` in uwuExt.
Colors are accent1/2/3 at tone 80 and accent1 at tone 65, on black.
Wallpaper, selected theme style, and preset colors follow the system theme.
Missing saved properties use the four fallback colors in `desc.txt`.
The very first boot therefore uses fallback colors until the system creates its palette.
Persisted properties survive reboot and are available before credential unlock.

RGB and alpha are **four color masks**, not a normal transparent image.
Do not run a premultiplying/alpha-removing image optimizer on these PNGs.
The intro transitions to saved colors once. The loop always uses saved colors, so there is
no repeated fallback-color flash. Three shape cycles close rotation seamlessly in 11.7 s.
`p` intro and `f` loop remain interruptible; the loop fades out in 12 frames (200 ms).
Small centered textures and `trim.txt` avoid full-screen texture allocation.

Regenerate with Python 3 + Pillow, independently of Android builds:

```sh
python3 vendor/uwu/packages/bootanimation/expressive/generate.py
python3 -m unittest discover -s vendor/uwu/packages/bootanimation/expressive -p 'test_*.py'
```

The product installs `bootanimation_uwu` to `/product/media/bootanimation.zip`.
Resolution uses `TARGET_BOOT_ANIMATION_RES`, then `TARGET_SCREEN_WIDTH`, then 1080.
Supported packages are 720, 1080 and 1440; other values select the 1080 package.
After building/flashing the framework, services, SystemUI and SELinux policy, change a
wallpaper/theme and verify `adb shell getprop persist.bootanim.color1` (and 2–4).
Reboot to test before unlock. No installation/reboot is performed by the generator.

References:
- https://source.android.com/docs/core/display/material
- https://android.googlesource.com/platform/frameworks/base/+/main/cmds/bootanimation/FORMAT.md
- https://github.com/material-components/material-components-android/blob/master/lib/java/com/google/android/material/loadingindicator/LoadingIndicatorAnimatorDelegate.java

Copyright 2026 UwUniverse. Motion parameters adapted from Material Components,
Copyright 2024 The Android Open Source Project. Apache License 2.0.
