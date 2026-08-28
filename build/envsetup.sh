CLANG_VERSION=$(${ANDROID_BUILD_TOP}/vendor/uwu/tools/get_clang_version.py)
export LLVM_AOSP_PREBUILTS_VERSION="${CLANG_VERSION}"

RUST_VERSION=$(grep 'RustDefaultVersion =' ${ANDROID_BUILD_TOP}/build/soong/rust/config/global.go | awk '{print $3}' | awk -F '"' '{print $2}')
export RUST_AOSP_PREBUILTS_VERSION="${RUST_VERSION}"

function brunch()
{
    if breakfast "$@"; then
        m otapackage
    else
        echo "No such item in brunch menu. Try 'breakfast'"
        return 1
    fi
}

function breakfast()
{
    local target=$1
    local variant=$2
    local aosp_target_release=aosp_current

    if [ $# -eq 0 ]; then
        echo "Usage: breakfast <device|lunch-target> [variant]" >&2
        return 1
    elif [[ "$target" =~ ^.+-.+-(user|userdebug|eng)$ ]]; then
        # A complete lunch target already includes its release and variant.
        lunch "$target"
    else
        if [[ "$target" =~ -(user|userdebug|eng)$ ]]; then
            variant=${target##*-}
            target=${target%-*}
        fi
        if [ -z "$variant" ]; then
            variant="userdebug"
        fi

        lunch "uwu_$target" "$aosp_target_release" "$variant"
    fi
}

function lineageremote()
{
    if ! git rev-parse --git-dir &> /dev/null
    then
        echo ".git directory not found. Please run this from the root directory of the Android repository you wish to set up."
        return 1
    fi
    git remote rm lineage 2> /dev/null
    local REMOTE=$(git config --get remote.github.projectname)
    local LINEAGE="true"
    if [ -z "$REMOTE" ]
    then
        REMOTE=$(git config --get remote.aosp.projectname)
        LINEAGE="false"
    fi
    if [ -z "$REMOTE" ]
    then
        REMOTE=$(git config --get remote.clo.projectname)
        LINEAGE="false"
    fi

    if [ $LINEAGE = "false" ]
    then
        local PROJECT=$(echo $REMOTE | sed -e "s#platform/#android/#g; s#/#_#g")
        local PFX="LineageOS/"
    else
        local PROJECT=$REMOTE
    fi

    local LINEAGE_USER=$(git config --get review.review.lineageos.org.username)
    if [ -z "$LINEAGE_USER" ]
    then
        git remote add lineage ssh://review.lineageos.org:29418/$PFX$PROJECT
    else
        git remote add lineage ssh://$LINEAGE_USER@review.lineageos.org:29418/$PFX$PROJECT
    fi
    echo "Remote 'lineage' created"
}

function aospremote()
{
    local T=`git rev-parse --show-toplevel 2> /dev/null`
    if [ -z "$T" ]
    then
        echo "Git repository not found. Please run this from the directory of the Android repository you wish to set up."
        return 1
    fi
    git remote rm aosp 2> /dev/null

    if [ -f "$T/.gitupstream" ]; then
        local REMOTE=$(cat "$T/.gitupstream" | cut -d ' ' -f 1)
        git remote add aosp ${REMOTE}
    else
        local PROJECT=$(pwd -P | sed -e "s#$ANDROID_BUILD_TOP\/##; s#-caf.*##; s#\/default##")
        # Google moved the repo location in Oreo
        if [ $PROJECT = "build/make" ]
        then
            PROJECT="build"
        fi
        if (echo $PROJECT | grep -qv "^device")
        then
            local PFX="platform/"
        fi
        git remote add aosp https://android.googlesource.com/$PFX$PROJECT
    fi
    echo "Remote 'aosp' created"
}

# Return success if adb is up and not in recovery
function _adb_connected {
    {
        if [[ "$(adb get-state)" == device ]]
        then
            return 0
        fi
    } 2>/dev/null

    return 1
};

# Credit for color strip sed: http://goo.gl/BoIcm
function dopush()
{
    echo "dopush is temporarily unavailable." >&2
    return 1
    local func=$1
    local TCPIPPORT ret is_gnu_sed LOC CHKPERM RELOUT
    local stop_n_start TARGET FILE OLDPERM OLDOWN OLDGRP
    shift

    adb start-server # Prevent unexpected starting server message from adb get-state in the next line
    if ! _adb_connected; then
        echo "No device is online. Waiting for one..."
        echo "Please connect USB and/or enable USB debugging"
        until _adb_connected; do
            sleep 1
        done
        echo "Device Found."
    fi

    if [ "$FORCE_PUSH" = "true" ] || { [ -n "$UWU_BUILD" ] && [ "$(adb shell getprop ro.uwu.device | tr -d '\r')" = "$UWU_BUILD" ]; };
    then
    # retrieve IP and PORT info if we're using a TCP connection
    TCPIPPORT=$(adb devices \
        | egrep '^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]):[0-9]+[^0-9]+' \
        | head -1 | awk '{print $1}')
    adb root &> /dev/null
    sleep 0.3
    if [ -n "$TCPIPPORT" ]
    then
        # adb root just killed our connection
        # so reconnect...
        adb connect "$TCPIPPORT"
    fi
    adb wait-for-device &> /dev/null
    adb remount &> /dev/null

    mkdir -p "$OUT"
    if [ -n "${ZSH_VERSION:-}" ]; then
        "$func" "$@" | tee "$OUT/.log"
        ret=${pipestatus[1]}
    else
        "$func" "$@" | tee "$OUT/.log"
        ret=${PIPESTATUS[0]}
    fi
    if [ $ret -ne 0 ]; then
        rm -f "$OUT/.log"
        return $ret
    fi

    is_gnu_sed=`sed --version | head -1 | grep -c GNU`

    # Install: <file>
    if [ $is_gnu_sed -gt 0 ]; then
        LOC="$(cat $OUT/.log | sed -r -e 's/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[m|K]//g' -e 's/^\[ {0,2}[0-9]{1,3}% [0-9]{1,6}\/[0-9]{1,6}( [0-9]{0,2}?h?[0-9]{0,2}?m?[0-9]{0,2}s remaining)?\] +//' \
            | grep '^Install: ' | cut -d ':' -f 2)"
    else
        LOC="$(cat $OUT/.log | sed -E "s/"$'\E'"\[([0-9]{1,3}((;[0-9]{1,3})*)?)?[m|K]//g" -E "s/^\[ {0,2}[0-9]{1,3}% [0-9]{1,6}\/[0-9]{1,6}( [0-9]{0,2}?h?[0-9]{0,2}?m?[0-9]{0,2}s remaining)?\] +//" \
            | grep '^Install: ' | cut -d ':' -f 2)"
    fi

    # Copy: <file>
    if [ $is_gnu_sed -gt 0 ]; then
        LOC="$LOC $(cat $OUT/.log | sed -r -e 's/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[m|K]//g' -e 's/^\[ {0,2}[0-9]{1,3}% [0-9]{1,6}\/[0-9]{1,6}( [0-9]{0,2}?h?[0-9]{0,2}?m?[0-9]{0,2}s remaining)?\] +//' \
            | grep '^Copy: ' | cut -d ':' -f 2)"
    else
        LOC="$LOC $(cat $OUT/.log | sed -E "s/"$'\E'"\[([0-9]{1,3}((;[0-9]{1,3})*)?)?[m|K]//g" -E 's/^\[ {0,2}[0-9]{1,3}% [0-9]{1,6}\/[0-9]{1,6}( [0-9]{0,2}?h?[0-9]{0,2}?m?[0-9]{0,2}s remaining)?\] +//' \
            | grep '^Copy: ' | cut -d ':' -f 2)"
    fi

    # If any files are going to /data, push an octal file permissions reader to device
    if [ -n "$(echo $LOC | egrep '(^|\s)/data')" ]; then
        CHKPERM="/data/local/tmp/chkfileperm.sh"
(
cat <<'EOF'
#!/system/bin/sh
FILE=$@
if [ -e $FILE ]; then
    ls -l $FILE | awk '{k=0;for(i=0;i<=8;i++)k+=((substr($1,i+2,1)~/[rwx]/)*2^(8-i));if(k)printf("%0o ",k);print}' | cut -d ' ' -f1
fi
EOF
) > $OUT/.chkfileperm.sh
        echo "Pushing file permissions checker to device"
        adb push $OUT/.chkfileperm.sh $CHKPERM
        adb shell chmod 755 $CHKPERM
        rm -f $OUT/.chkfileperm.sh
    fi

    RELOUT=$(echo $OUT | sed "s#^${ANDROID_BUILD_TOP}/##")

    stop_n_start=false
    for TARGET in $(echo $LOC | tr " " "\n" | sed "s#.*${RELOUT}##" | sort | uniq); do
        # Make sure file is in $OUT/{system,system_ext,data,odm,oem,product,vendor}
        case $TARGET in
            /system/*|/system_ext/*|/data/*|/odm/*|/oem/*|/product/*|/vendor/*)
                # Get out file from target (i.e. /system/bin/adb)
                FILE=$OUT$TARGET
            ;;
            *) continue ;;
        esac

        case $TARGET in
            /data/*)
                # fs_config only sets permissions and se labels for files pushed to /system
                if [ -n "$CHKPERM" ]; then
                    OLDPERM=$(adb shell $CHKPERM $TARGET)
                    OLDPERM=$(echo $OLDPERM | tr -d '\r' | tr -d '\n')
                    OLDOWN=$(adb shell ls -al "$TARGET" | awk '{print $3}')
                    OLDGRP=$(adb shell ls -al "$TARGET" | awk '{print $4}')
                fi
                echo "Pushing: $TARGET"
                adb push $FILE $TARGET
                if [ -n "$OLDPERM" ]; then
                    echo "Setting file permissions: $OLDPERM, $OLDOWN":"$OLDGRP"
                    adb shell chown "$OLDOWN":"$OLDGRP" $TARGET
                    adb shell chmod "$OLDPERM" $TARGET
                else
                    echo "$TARGET did not exist previously, you should set file permissions manually"
                fi
                adb shell restorecon "$TARGET"
            ;;
            */SystemUI.apk|*/framework/*)
                # Only need to stop services once
                if ! $stop_n_start; then
                    adb shell stop
                    stop_n_start=true
                fi
                echo "Pushing: $TARGET"
                adb push $FILE $TARGET
            ;;
            *)
                echo "Pushing: $TARGET"
                adb push $FILE $TARGET
            ;;
        esac
    done
    if [ -n "$CHKPERM" ]; then
        adb shell rm $CHKPERM
    fi
    if $stop_n_start; then
        adb shell start
    fi
    rm -f $OUT/.log
    return 0
    else
        echo "The connected device does not appear to be $UWU_BUILD, run away!"
    fi
}

alias mmp='dopush mm'
alias mmmp='dopush mmm'
alias mkap='dopush m'

function repopick() {
    local T
    T=$(gettop) || return
    "$T/lineage/scripts/repopick/repopick.py" "$@"
}

function sort-blobs-list() {
    local T
    T=$(gettop) || return
    "$T/tools/extract-utils/sort-blobs-list.py" "$@"
}

function build_kernel() {
    echo "build_kernel is temporarily unavailable." >&2
    return 1
    if [[ "${SKIP_KERNEL_BUILD}" == "true" || "${SKIP_KERNEL_BUILD}" == "1" ]]; then
        echo "Skipping kernel build"
        return
    fi
    local lineage_version="lineage-$(_get_build_var_cached PRODUCT_VERSION_MAJOR).$(_get_build_var_cached PRODUCT_VERSION_MINOR)"

    local target_kernel_device="$(_get_build_var_cached TARGET_KERNEL_DEVICE)"
    local target_kernel_dir="${ANDROID_BUILD_TOP}/$(_get_build_var_cached TARGET_KERNEL_DIR)"
    local target_kernel_source="$(_get_build_var_cached TARGET_KERNEL_PLATFORM_SOURCE)"

    local KERNEL_BUILD_TOP="${ANDROID_BUILD_TOP}/out-kernel/${target_kernel_source}"

    # Make sure we have the kernel source folder structure in place
    if [ ! -d "${KERNEL_BUILD_TOP}/.repo" ]; then
        echo "Kernel source ${KERNEL_BUILD_TOP} is missing, preparing folder structure"

        # Copy .repo/repo from Android tree to allow nested `repo init`
        mkdir -p "${KERNEL_BUILD_TOP}/.repo"
        cp -R "${ANDROID_BUILD_TOP}/.repo/repo" "${KERNEL_BUILD_TOP}/.repo/repo"

        # Allow custom .repo/project-objects dir
        if [ -n "${KERNEL_REPO_PROJECT_OBJECTS_DIR}" ]; then
            if [ ! -d "${KERNEL_REPO_PROJECT_OBJECTS_DIR}" ]; then
                mkdir "${KERNEL_REPO_PROJECT_OBJECTS_DIR}"
            fi
            ln -sf "${KERNEL_REPO_PROJECT_OBJECTS_DIR}" "${KERNEL_BUILD_TOP}/.repo/project-objects"
        fi

        # Allow custom .repo/projects dir
        if [ -n "${KERNEL_REPO_PROJECTS_DIR}" ]; then
            if [ ! -d "${KERNEL_REPO_PROJECTS_DIR}" ]; then
                mkdir "${KERNEL_REPO_PROJECTS_DIR}"
            fi
            ln -sf "${KERNEL_REPO_PROJECTS_DIR}" "${KERNEL_BUILD_TOP}/.repo/projects"
        fi

        # Mark as out dir to prevent build system from scanning it
        touch "${KERNEL_BUILD_TOP}/.out-dir"
    fi

    # Init, sync, remove previous build output & build kernel
    pushd "${KERNEL_BUILD_TOP}" > /dev/null
    if [[ "${SKIP_KERNEL_SYNC}" != "true" && "${SKIP_KERNEL_SYNC}" != "1" ]]; then
        echo "Syncing ${KERNEL_BUILD_TOP}"
        local target_kernel_manifest=$(echo android_kernel_${target_kernel_source}_manifest | tr / _)
        local repo_init_args=("-b" "${lineage_version}")
        if [ -n "${LINEAGE_MIRROR}" ]; then
            repo_init_args+=("--reference" "${LINEAGE_MIRROR}")
        fi
        if [ -n "${REPO_VERSION}" ]; then
            repo_init_args+=("--repo-rev" "${REPO_VERSION}")
        fi

        yes | repo init -u https://github.com/LineageOS/${target_kernel_manifest}.git ${repo_init_args[@]} || [ $? -eq 141 ]
        if [ $? -ne 0 ]; then
            echo "Kernel source repo init failed"
            popd > /dev/null
            return 1
        fi
        if ! repo sync --detach --force-sync; then
            echo "Kernel source repo sync failed"
            popd > /dev/null
            return 1
        fi
    fi
    if [ -d "${KERNEL_BUILD_TOP}/out/${target_kernel_device}/dist" ]; then
        rm -rf "${KERNEL_BUILD_TOP}/out/${target_kernel_device}/dist"
    fi
    if ! ./build_"${target_kernel_device}".sh; then
        popd > /dev/null
        return 1
    fi
    popd > /dev/null

    # Remove previous kernel prebuilts
    if [ -d "${target_kernel_dir}" ]; then
        local find_args=("-maxdepth" "1" "-type" "f" "!" "-name" ".gitignore")
        # Some kernels don't generate the module lists, in which case they're
        # checked in next to the prebuilts. Don't delete what won't come back.
        if [[ "$(_get_build_var_cached TARGET_PROVIDES_STATIC_MODULE_LISTS)" == "true" ]]; then
            find_args+=("!" "-name" "*.modules.load*" "!" "-name" "*.modules.blocklist")
        fi
        find "${target_kernel_dir}" "${find_args[@]}" -delete
    fi

    # Copy the new kernel prebuilts
    mkdir -p "${target_kernel_dir}"
    cp -a "${KERNEL_BUILD_TOP}/out/${target_kernel_device}/dist/"* "${target_kernel_dir}/"
    chmod -x "${target_kernel_dir}/"*
    echo "Kernel build output copied to ${target_kernel_dir}/"
}
