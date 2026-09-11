// Copyright 2026 uwuAOSP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kernel

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/android"
	platformkernel "android/soong/kernel"
)

type configProperties struct {
	Defconfig     *string
	Fragments     []string
	Merge_at_once *bool
	Overrides     []string
	Lto           *string
}

type deviceTreeProperties struct {
	Enabled        *bool
	Qcom_merge     *bool
	Src            *string `android:"path"`
	Target         *string
	Image_name     *string
	Config         *string `android:"path"`
	Input_globs    []string
	Page_size      *int64
	Custom_command *string
}

type moduleSetProperties struct {
	Enabled              *bool
	Build_targets        []string
	External_module_root *string
	External_modules     []string
	Install_strip        *bool
	Module_aliases       []string

	System_dlkm_module_install_list []string
	System_dlkm_module_load_list    []string
	System_dlkm_module_blocklist    *string

	Vendor_dlkm_module_install_list []string
	Vendor_dlkm_module_load_list    []string
	Vendor_dlkm_module_blocklist    *string

	Vendor_ramdisk_module_install_list []string
	Vendor_ramdisk_module_load_list    []string
	Vendor_ramdisk_module_blocklist    *string

	Recovery_module_install_list []string
	Recovery_module_load_list    []string
	Recovery_module_blocklist    *string

	Auto_collect_deps  *bool
	Allow_missing_load *bool
}

type kernelProperties struct {
	// Kernel source directory relative to the Android source root.
	Kernel_dir *string

	// Optional prebuilt kernel. When set, Kbuild is skipped.
	Prebuilt         *string `android:"path"`
	Prebuilt_config  *string `android:"path"`
	Prebuilt_headers *string `android:"path"`
	Prebuilt_modules *string `android:"path"`

	Kernel_arch *string
	Image_name  *string

	Config  configProperties
	Dtb     deviceTreeProperties
	Dtbo    deviceTreeProperties
	Modules moduleSetProperties

	Clang_version   *string
	Clang_path      *string
	Rust_version    *string
	Autofdo_profile *string
	Clang_triple    *string
	Cross_compile   *string
	Cc              *string
	Ld              *string
	Rbe_wrapper     *string

	Make_command     *string
	Build_jobs       *int64
	Make_flags       []string
	Additional_flags []string
	Environment      []string

	// Optional source patterns relative to kernel_dir. Kbuild tracks source
	// dependencies itself; use this only for additional analysis-time inputs.
	Srcs []string
}

type kernelModule struct {
	android.ModuleBase
	properties kernelProperties

	kernelImage    android.Path
	dtbImage       android.OptionalPath
	dtboImage      android.OptionalPath
	config         android.OptionalPath
	modules        android.OptionalPath
	headerDirs     android.Paths
	headerDeps     android.Paths
	autofdoProfile android.OptionalPath

	installedKernel  android.InstallPath
	installedDtb     android.InstallPath
	androidMkEnabled bool
	installDir       android.InstallPath
}

func kernelFactory() android.Module {
	m := &kernelModule{}
	m.AddProperties(&m.properties)
	android.InitAndroidArchModule(m, android.DeviceSupported, android.MultilibFirst)
	android.AddLoadHook(m, m.createModuleInstallers)
	return m
}

func (m *kernelModule) createModuleInstallers(ctx android.LoadHookContext) {
	if !proptools.Bool(m.properties.Modules.Enabled) {
		return
	}
	type installer struct {
		name         string
		blocklist    *string
		loadLists    []string
		installLists []string
	}
	installers := []installer{
		{"system_dlkm", m.properties.Modules.System_dlkm_module_blocklist, m.properties.Modules.System_dlkm_module_load_list, m.properties.Modules.System_dlkm_module_install_list},
		{"vendor_dlkm", m.properties.Modules.Vendor_dlkm_module_blocklist, m.properties.Modules.Vendor_dlkm_module_load_list, m.properties.Modules.Vendor_dlkm_module_install_list},
		{"vendor_ramdisk", m.properties.Modules.Vendor_ramdisk_module_blocklist, m.properties.Modules.Vendor_ramdisk_module_load_list, m.properties.Modules.Vendor_ramdisk_module_install_list},
		{"recovery", m.properties.Modules.Recovery_module_blocklist, m.properties.Modules.Recovery_module_load_list, m.properties.Modules.Recovery_module_install_list},
	}
	hasSystemDlkm := len(m.properties.Modules.System_dlkm_module_install_list) > 0 &&
		len(m.properties.Modules.System_dlkm_module_load_list) > 0
	for _, item := range installers {
		if len(item.loadLists) == 0 && len(item.installLists) == 0 {
			continue
		}
		if len(item.loadLists) == 0 || len(item.installLists) == 0 {
			ctx.PropertyErrorf("modules", "%s modules require both module_install_list and module_load_list", item.name)
			continue
		}
		props := platformkernel.PrebuiltKernelModulesProperties{}
		props.Zip.Src = proptools.StringPtr(":" + ctx.ModuleName() + "{.modules}")
		props.Zip.Load_file = proptools.StringPtr("modules.load." + item.name)
		props.Zip.Install_file = proptools.StringPtr("modules.install." + item.name)
		if item.name == "vendor_dlkm" && hasSystemDlkm {
			props.System_dep = proptools.StringPtr(":" + ctx.ModuleName() + "_modules_system_dlkm{.modules.zip}")
		}
		if item.blocklist != nil {
			props.Zip.Blocklist_file = proptools.StringPtr("modules.blocklist." + item.name)
		}
		partitionProps, ok := kernelModulePartitionProps(ctx, item.name)
		if !ok {
			continue
		}
		name := ctx.ModuleName() + "_modules_" + item.name
		ctx.CreateModule(platformkernel.PrebuiltKernelModulesFactory,
			&struct{ Name *string }{Name: proptools.StringPtr(name)},
			&props,
			partitionProps)
	}
}

func kernelModulePartitionProps(ctx android.LoadHookContext, partition string) (interface{}, bool) {
	props := &struct {
		Soc_specific          *bool
		System_dlkm_specific  *bool
		Vendor_dlkm_specific  *bool
		Ramdisk               *bool
		Vendor_ramdisk        *bool
		Vendor_kernel_ramdisk *bool
		Recovery              *bool
	}{}
	switch partition {
	case "system":
	case "system_dlkm":
		props.System_dlkm_specific = proptools.BoolPtr(true)
	case "vendor":
		props.Soc_specific = proptools.BoolPtr(true)
	case "vendor_dlkm":
		props.Vendor_dlkm_specific = proptools.BoolPtr(true)
	case "ramdisk":
		props.Ramdisk = proptools.BoolPtr(true)
	case "vendor_ramdisk":
		props.Vendor_ramdisk = proptools.BoolPtr(true)
	case "vendor_kernel_ramdisk":
		props.Vendor_kernel_ramdisk = proptools.BoolPtr(true)
	case "recovery":
		props.Recovery = proptools.BoolPtr(true)
	default:
		ctx.PropertyErrorf("modules", "unsupported module partition %q", partition)
		return nil, false
	}
	return props, true
}

func (m *kernelModule) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	m.androidMkEnabled = ctx.Config().ProductVariables().BoardUsesSoongKernel
	if prebuilt := proptools.String(m.properties.Prebuilt); prebuilt != "" {
		m.generatePrebuilt(ctx, prebuilt)
	} else {
		m.generateSource(ctx)
	}
	if ctx.Failed() || m.kernelImage == nil {
		return
	}
	installDir := android.PathForModuleInPartitionInstall(ctx, "")
	m.installDir = installDir
	if ctx.Config().ProductVariables().BoardUsesSoongKernel {
		m.installedKernel = ctx.InstallFile(installDir, "kernel", m.kernelImage)
		if m.dtbImage.Valid() {
			m.installedDtb = ctx.InstallFile(installDir, "dtb.img", m.dtbImage.Path())
		}
	}

	ctx.SetOutputFiles(android.Paths{m.kernelImage}, "")
	if m.config.Valid() {
		ctx.SetOutputFiles(android.Paths{m.config.Path()}, ".config")
	}
	if m.dtbImage.Valid() {
		ctx.SetOutputFiles(android.Paths{m.dtbImage.Path()}, ".dtb")
	}
	if m.dtboImage.Valid() {
		ctx.SetOutputFiles(android.Paths{m.dtboImage.Path()}, ".dtbo")
	}
	if m.modules.Valid() {
		ctx.SetOutputFiles(android.Paths{m.modules.Path()}, ".modules")
	}
}

func (m *kernelModule) GeneratedHeaderDirs() android.Paths {
	return m.headerDirs
}

func (m *kernelModule) GeneratedSourceFiles() android.Paths {
	return nil
}

func (m *kernelModule) GeneratedDeps() android.Paths {
	return m.headerDeps
}

func (m *kernelModule) generatePrebuilt(ctx android.ModuleContext, prebuilt string) {
	if proptools.String(m.properties.Autofdo_profile) != "" {
		ctx.PropertyErrorf("autofdo_profile", "is only valid for source kernels")
	}
	if proptools.String(m.properties.Rbe_wrapper) != "" {
		ctx.PropertyErrorf("rbe_wrapper", "is only valid for source kernels")
	}
	if proptools.String(m.properties.Config.Defconfig) != "" || len(m.properties.Config.Fragments) > 0 || len(m.properties.Config.Overrides) > 0 {
		ctx.PropertyErrorf("config", "is not used for prebuilt kernels")
	}
	input := android.PathForModuleSrc(ctx, prebuilt)
	output := android.PathForModuleOut(ctx, "kernel", input.Base())
	ctx.Build(pctx, android.BuildParams{
		Rule:   android.CpRule,
		Input:  input,
		Output: output,
	})
	m.kernelImage = output
	if src := proptools.String(m.properties.Dtb.Src); src != "" {
		m.dtbImage = android.OptionalPathForPath(m.copyPrebuiltDeviceTree(ctx, "dtb", src, proptools.StringDefault(m.properties.Dtb.Image_name, "dtb.img")))
	}
	if src := proptools.String(m.properties.Dtbo.Src); src != "" {
		m.dtboImage = android.OptionalPathForPath(m.copyPrebuiltDeviceTree(ctx, "dtbo", src, proptools.StringDefault(m.properties.Dtbo.Image_name, "dtbo.img")))
	}
	if headers := proptools.String(m.properties.Prebuilt_headers); headers != "" {
		m.buildPrebuiltHeaders(ctx, headers)
	}
	if config := proptools.String(m.properties.Prebuilt_config); config != "" {
		m.config = android.OptionalPathForPath(m.copyPrebuiltFile(ctx, "kernel_build", config, ".config"))
	}
	if modules := proptools.String(m.properties.Prebuilt_modules); modules != "" {
		m.modules = android.OptionalPathForPath(m.copyPrebuiltFile(ctx, "kernel_modules", modules, "kernel_modules.zip"))
	}
}

func (m *kernelModule) copyPrebuiltDeviceTree(ctx android.ModuleContext, tag, src, name string) android.Path {
	return m.copyPrebuiltFile(ctx, tag, src, name)
}

func (m *kernelModule) copyPrebuiltFile(ctx android.ModuleContext, tag, src, name string) android.Path {
	input := android.PathForModuleSrc(ctx, src)
	output := android.PathForModuleOut(ctx, tag, name)
	ctx.Build(pctx, android.BuildParams{Rule: android.CpRule, Input: input, Output: output})
	return output
}

func (m *kernelModule) buildPrebuiltHeaders(ctx android.ModuleContext, archive string) {
	input := android.PathForModuleSrc(ctx, archive)
	headersOut := android.PathForModuleOut(ctx, "headers")
	stamp := android.PathForModuleOut(ctx, "headers.timestamp")
	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	cmd := rule.Command().Text("set -e; rm -rf").Text(headersOut.String())
	cmd.Text("&& mkdir -p").Text(headersOut.String())
	cmd.Text("&& gzip -d <").Input(input).Text("| tar -x -C").Text(headersOut.String())
	cmd.Text("&& vendor/uwu/build/tools/clean_headers.sh").Text(headersOut.String())
	cmd.Text("&& touch").Output(stamp)
	rule.Build("kernel_prebuilt_headers", "Prebuilt kernel UAPI headers")
	m.headerDeps = android.Paths{stamp}
	m.headerDirs = kernelHeaderDirs(ctx, headersOut)
}

func (m *kernelModule) generateSource(ctx android.ModuleContext) {
	kernelDir := filepath.Clean(proptools.String(m.properties.Kernel_dir))
	if kernelDir == "." || kernelDir == "" || filepath.IsAbs(kernelDir) || strings.HasPrefix(kernelDir, "..") {
		ctx.PropertyErrorf("kernel_dir", "must be a source-root-relative directory")
		return
	}
	arch := proptools.String(m.properties.Kernel_arch)
	if arch == "" {
		ctx.PropertyErrorf("kernel_arch", "must be set")
	}
	imageName := proptools.String(m.properties.Image_name)
	if imageName == "" {
		ctx.PropertyErrorf("image_name", "must be set for source builds")
	}
	defconfig := proptools.String(m.properties.Config.Defconfig)
	if defconfig == "" {
		ctx.PropertyErrorf("config.defconfig", "must be set for source builds")
	}
	if proptools.Bool(m.properties.Dtb.Qcom_merge) &&
		(!proptools.Bool(m.properties.Dtb.Enabled) || !proptools.Bool(m.properties.Dtbo.Enabled)) {
		ctx.PropertyErrorf("dtb.qcom_merge", "requires both dtb.enabled and dtbo.enabled")
	}
	if proptools.String(m.properties.Dtb.Src) != "" {
		ctx.PropertyErrorf("dtb.src", "is only valid for prebuilt kernels")
	}
	if proptools.String(m.properties.Dtbo.Src) != "" {
		ctx.PropertyErrorf("dtbo.src", "is only valid for prebuilt kernels")
	}
	if m.properties.Dtb.Config != nil {
		ctx.PropertyErrorf("dtb.config", "is only supported for dtbo outputs")
	}
	if m.properties.Dtb.Page_size != nil {
		ctx.PropertyErrorf("dtb.page_size", "is only supported for dtbo outputs")
	}
	if proptools.Bool(m.properties.Dtb.Qcom_merge) && m.properties.Dtbo.Target != nil {
		ctx.PropertyErrorf("dtbo.target", "is not supported with dtb.qcom_merge")
	}
	switch lto := proptools.String(m.properties.Config.Lto); lto {
	case "", "none", "thin", "full":
	default:
		ctx.PropertyErrorf("config.lto", "%q must be one of none, thin, or full", lto)
	}
	if proptools.String(m.properties.Prebuilt_headers) != "" {
		ctx.PropertyErrorf("prebuilt_headers", "is only valid for prebuilt kernels")
	}
	if proptools.String(m.properties.Prebuilt_config) != "" {
		ctx.PropertyErrorf("prebuilt_config", "is only valid for prebuilt kernels")
	}
	if proptools.String(m.properties.Prebuilt_modules) != "" {
		ctx.PropertyErrorf("prebuilt_modules", "is only valid for prebuilt kernels")
	}
	m.configureAutofdo(ctx, kernelDir, arch)
	if proptools.String(m.properties.Rbe_wrapper) != "" {
		if ctx.Config().Getenv("TOP") == "" {
			ctx.PropertyErrorf("rbe_wrapper", "requires TOP to be set")
		}
	}
	if ctx.Failed() {
		return
	}

	kernelSource := android.PathForSource(ctx, kernelDir)
	inputs := m.kernelInputs(ctx, kernelDir)
	sourceRoots := []string{kernelDir}
	if proptools.Bool(m.properties.Modules.Enabled) {
		if root := filepath.Clean(proptools.String(m.properties.Modules.External_module_root)); root != "." && root != "" {
			sourceRoots = append(sourceRoots, root)
		}
	}
	sourceStamp := m.buildSourceStamp(ctx, sourceRoots)
	buildInputs := append(android.Paths{sourceStamp}, inputs...)
	if proptools.String(m.properties.Rbe_wrapper) != "" {
		buildInputs = append(buildInputs, android.PathForSource(ctx, "vendor/uwu/build/tools/kernel_rbe_cc.sh"))
	}
	m.buildHeaders(ctx, kernelSource.String(), arch, buildInputs)
	configInputs := android.Paths{m.configPath(ctx, kernelDir, arch, defconfig)}
	for _, fragment := range m.properties.Config.Fragments {
		configInputs = append(configInputs, m.configPath(ctx, kernelDir, arch, fragment))
	}

	buildRoot := android.PathForModuleOut(ctx, "kernel_build")
	configOut := buildRoot.Join(ctx, ".config")
	m.config = android.OptionalPathForPath(configOut)
	top := ctx.Config().Getenv("TOP")
	absolutePath := func(path string) string {
		if filepath.IsAbs(path) || top == "" {
			return path
		}
		return filepath.Join(top, path)
	}

	configRule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	configCmd := configRule.Command().Text("set -e;")
	configCmd.Text("rm -rf").Text(buildRoot.String()).Text("&& mkdir -p").Text(buildRoot.String()).Text("&&")
	configCmd.Text("cp").Input(configInputs[0]).Text(configOut.String()).Text("&&")
	configCmd.Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, "olddefconfig"))
	if proptools.Bool(m.properties.Config.Merge_at_once) && len(configInputs) > 1 {
		fragments := configInputs[1:].Strings()
		for i, fragment := range fragments {
			fragments[i] = absolutePath(fragment)
		}
		configCmd.Text("&&").Textf("cd %s && %s/scripts/kconfig/merge_config.sh -m -O %s %s %s",
			absolutePath(buildRoot.String()), absolutePath(kernelSource.String()), absolutePath(buildRoot.String()), absolutePath(configOut.String()), strings.Join(fragments, " "))
		configCmd.Text("&&").Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, "olddefconfig"))
	} else {
		for _, fragment := range configInputs[1:] {
			configCmd.Text("&&").Textf("cd %s && %s/scripts/kconfig/merge_config.sh -m -O %s %s %s",
				absolutePath(buildRoot.String()), absolutePath(kernelSource.String()), absolutePath(buildRoot.String()), absolutePath(configOut.String()), absolutePath(fragment.String()))
			configCmd.Text("&&").Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, "olddefconfig"))
		}
	}
	m.applyLto(configCmd, kernelSource.String(), configOut.String())
	for _, override := range m.properties.Config.Overrides {
		configCmd.Text("&& printf '%s\\n'").Text(proptools.ShellEscape(override)).Text(">>").Text(configOut.String())
	}
	if len(m.properties.Config.Overrides) > 0 {
		configCmd.Text("&&").Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, "olddefconfig"))
	}
	configCmd.ImplicitOutput(configOut).Implicits(configInputs).Implicits(inputs)
	configRule.Build("kernel_config", "Kernel config")

	imageOut := android.PathForModuleOut(ctx, "kernel", imageName)
	actionInputs := append(android.Paths{}, buildInputs...)
	if m.autofdoProfile.Valid() {
		actionInputs = append(actionInputs, m.autofdoProfile.Path())
	}
	imageRule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	imageCmd := imageRule.Command().Text("set -e;").Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, imageName))
	imageCmd.Text("&& if [ -d").Text(filepath.Join(kernelSource.String(), "arch", arch, "boot", "dts")).Text("]; then")
	imageCmd.Text(m.makeInvocation(ctx, kernelSource.String(), buildRoot.String(), arch, "dtbs")).Text("; fi && mkdir -p").Text(filepath.Dir(imageOut.String()))
	imageCmd.Text("&& cp").Text(filepath.Join(buildRoot.String(), "arch", arch, "boot", imageName)).Output(imageOut)
	imageCmd.Implicit(configOut).Implicits(actionInputs)
	imageRule.Build("kernel_image", "Kernel image")
	m.kernelImage = imageOut

	treeDependency := android.Path(imageOut)
	if proptools.Bool(m.properties.Dtb.Qcom_merge) {
		dtb, dtbo := m.buildQcomDeviceTrees(ctx, kernelSource.String(), buildRoot.String(), arch, treeDependency, actionInputs)
		m.dtbImage = android.OptionalPathForPath(dtb)
		m.dtboImage = android.OptionalPathForPath(dtbo)
		treeDependency = dtbo
	} else if proptools.Bool(m.properties.Dtb.Enabled) {
		dtb := m.buildDeviceTree(ctx, kernelSource.String(), buildRoot.String(), arch, treeDependency, m.properties.Dtb, false, actionInputs)
		m.dtbImage = android.OptionalPathForPath(dtb)
		treeDependency = dtb
	}
	if proptools.Bool(m.properties.Dtbo.Enabled) {
		if proptools.Bool(m.properties.Dtb.Qcom_merge) {
			// The QCOM merge rule emits both images from one merged tree.
		} else {
			dtbo := m.buildDeviceTree(ctx, kernelSource.String(), buildRoot.String(), arch, treeDependency, m.properties.Dtbo, true, actionInputs)
			m.dtboImage = android.OptionalPathForPath(dtbo)
			treeDependency = dtbo
		}
	}
	if proptools.Bool(m.properties.Modules.Enabled) {
		m.modules = android.OptionalPathForPath(m.buildModules(ctx, kernelSource.String(), buildRoot.String(), arch, treeDependency, actionInputs))
	}
}

func (m *kernelModule) configureAutofdo(ctx android.ModuleContext, kernelDir, arch string) {
	profile := proptools.String(m.properties.Autofdo_profile)
	if profile == "none" {
		return
	}
	explicit := profile != ""
	if profile == "" {
		gkiArch := arch
		if arch == "arm64" {
			gkiArch = "aarch64"
		}
		profile = filepath.Join(kernelDir, "gki", gkiArch, "afdo", "kernel.afdo")
	}
	paths, err := ctx.GlobWithDeps(profile, nil)
	if err != nil {
		ctx.PropertyErrorf("autofdo_profile", "unable to find %q: %s", profile, err)
		return
	}
	if len(paths) == 0 {
		if explicit {
			ctx.PropertyErrorf("autofdo_profile", "%q does not exist", profile)
		}
		return
	}
	if len(paths) != 1 {
		ctx.PropertyErrorf("autofdo_profile", "%q must resolve to exactly one file", profile)
		return
	}
	m.autofdoProfile = android.OptionalPathForPath(android.PathForSource(ctx, paths[0]))
}

func (m *kernelModule) buildQcomDeviceTrees(ctx android.ModuleContext, source, buildRoot, arch string, dependency android.Path, inputs android.Paths) (android.Path, android.Path) {
	dtbName := proptools.StringDefault(m.properties.Dtb.Image_name, "dtb.img")
	dtboName := proptools.StringDefault(m.properties.Dtbo.Image_name, "dtbo.img")
	dtbOut := android.PathForModuleOut(ctx, "dtb", dtbName)
	dtboOut := android.PathForModuleOut(ctx, "dtbo", dtboName)
	baseDir := android.PathForModuleOut(ctx, "qcom_dt", "base")
	workDir := android.PathForModuleOut(ctx, "qcom_dt", "work")
	mergedDir := android.PathForModuleOut(ctx, "qcom_dt", "merged")
	target := proptools.StringDefault(m.properties.Dtb.Target, "dtbs")
	dtsDir := filepath.Join(buildRoot, "arch", arch, "boot", "dts", "vendor")
	mergeScript := android.PathForSource(ctx, "vendor/uwu/build/tools/merge_dtbs.py")

	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	cmd := rule.Command().Text("set -e; rm -rf").Text(baseDir.String()).Text(workDir.String()).Text(mergedDir.String())
	cmd.Text("&& mkdir -p").Text(baseDir.String()).Text(workDir.String()).Text(mergedDir.String())
	cmd.Text("&&").Text(m.makeInvocation(ctx, source, buildRoot, arch, target))
	cmd.Text("&& cp -a").Text(dtsDir + "/.").Text(workDir.String())
	cmd.Text("&& find").Text(workDir.String()).Text("-mindepth 2 -maxdepth 2 -type f \\( -name '*.dtb' -o -name '*.dtbo' \\) -exec mv -f {} ").Text(baseDir.String()).Text("\\;")
	cmd.Text("&& test -n \"$(find").Text(baseDir.String()).Text("-maxdepth 1 -type f -name '*.dtb' -print -quit)\"")
	cmd.Text("&& PATH=" + topRelativePath("out/host/linux-x86/bin") + ":$PATH python3").Input(mergeScript)
	cmd.Text("--base").Text(baseDir.String()).Text("--techpack").Text(workDir.String()).Text("--out").Text(mergedDir.String())
	cmd.Text("&& files=$(")
	appendFindPatterns(cmd, mergedDir.String(), m.properties.Dtb.Input_globs, "*.dtb")
	cmd.Text("); test -n \"$files\" && cat $files >").Output(dtbOut)
	cmd.Text("&& files=$(")
	appendFindPatterns(cmd, mergedDir.String(), m.properties.Dtbo.Input_globs, "*.dtbo")
	cmd.Text("); test -n \"$files\" && prebuilts/kernel-build-tools/linux-x86/bin/mkdtboimg create").Output(dtboOut)
	cmd.Textf("--page_size=%d $files", proptools.IntDefault(m.properties.Dtbo.Page_size, 4096))
	cmd.Implicit(dependency).Implicits(inputs)
	for _, tool := range []string{"fdtget", "fdtput", "fdtoverlay", "fdtoverlaymerge", "ufdt_apply_overlay"} {
		cmd.Implicit(android.PathForArbitraryOutput(ctx, "host", "linux-x86", "bin", tool))
	}
	rule.Build("kernel_qcom_dt", "Kernel QCOM merged DT images")
	return dtbOut, dtboOut
}

func appendFindPatterns(cmd *android.RuleBuilderCommand, dir string, patterns []string, fallback string) {
	if len(patterns) == 0 {
		patterns = []string{fallback}
	}
	cmd.Text("find").Text(dir).Text("-type f \\(")
	for i, pattern := range patterns {
		if i > 0 {
			cmd.Text("-o")
		}
		cmd.Text("-name").Text(proptools.ShellEscape(pattern))
	}
	cmd.Text("\\) | sort -u")
}

func (m *kernelModule) buildSourceStamp(ctx android.ModuleContext, sourceRoots []string) android.Path {
	var files android.Paths
	var dirs []string
	for _, root := range sourceRoots {
		if filepath.IsAbs(root) || root == "." || root == ".." || strings.HasPrefix(root, ".."+string(filepath.Separator)) {
			ctx.PropertyErrorf("modules.external_module_root", "must be a source-root-relative directory")
			continue
		}
		entries, err := ctx.GlobWithDeps(filepath.Join(root, "*"), nil)
		if err != nil {
			ctx.ModuleErrorf("unable to track kernel source directory %q: %s", root, err)
			continue
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry, "/") {
				dirs = append(dirs, strings.TrimSuffix(entry, "/"))
			} else {
				files = append(files, android.PathForSource(ctx, entry))
			}
		}
	}

	outDir := android.PathForModuleOut(ctx, "source_deps")
	stamp := outDir.Join(ctx, "source.timestamp")
	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled().
		Nsjail(outDir, android.PathForModuleOut(ctx, "source_deps_sandbox")).
		DirDepsFile(outDir.Join(ctx, "source.d"))
	cmd := rule.Command().Text("touch").Output(stamp).Implicits(files)
	for _, dir := range dirs {
		cmd.ImplicitDirectory(android.DirectoryPathForSource(ctx, dir))
	}
	rule.Build("kernel_source_deps", "Kernel source dependencies")
	return stamp
}

func (m *kernelModule) buildHeaders(ctx android.ModuleContext, source, arch string, inputs android.Paths) {
	buildOut := android.PathForModuleOut(ctx, "headers_build")
	headersOut := android.PathForModuleOut(ctx, "headers")
	stamp := android.PathForModuleOut(ctx, "headers.timestamp")
	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	cmd := rule.Command().Text("set -e; rm -rf").Text(buildOut.String()).Text(headersOut.String())
	cmd.Text("&& mkdir -p").Text(buildOut.String()).Text(headersOut.String())
	cmd.Text("&&").Text(m.makeInvocation(ctx, source, buildOut.String(), arch, "INSTALL_HDR_PATH="+topRelativePath(headersOut.Join(ctx, "usr").String())+" headers_install"))
	cmd.Text("&& vendor/uwu/build/tools/clean_headers.sh").Text(headersOut.String())
	cmd.Text("&& touch").Output(stamp).Implicits(inputs)
	rule.Build("kernel_headers", "Kernel UAPI headers")
	m.headerDeps = android.Paths{stamp}
	m.headerDirs = kernelHeaderDirs(ctx, headersOut)
}

func kernelHeaderDirs(ctx android.ModuleContext, headersOut android.Path) android.Paths {
	var dirs android.Paths
	for _, dir := range []string{
		"usr/audio/include/uapi",
		"usr/include",
		"usr/include/audio",
		"usr/include/audio/include/uapi",
		"usr/techpack/audio/include",
	} {
		dirs = append(dirs, headersOut.Join(ctx, dir))
	}
	return dirs
}

type kernelHeadersDependencyTag struct {
	blueprint.BaseDependencyTag
}

var kernelHeadersDepTag kernelHeadersDependencyTag

const legacyKernelHeadersModule = "generated_kernel_includes_legacy"

type kernelHeadersModule struct {
	android.ModuleBase
	headerDirs android.Paths
	headerDeps android.Paths
}

func kernelHeadersFactory() android.Module {
	m := &kernelHeadersModule{}
	android.InitAndroidArchModule(m, android.DeviceSupported, android.MultilibBoth)
	return m
}

func (m *kernelHeadersModule) DepsMutator(ctx android.BottomUpMutatorContext) {
	module := ctx.Config().VendorConfig("uwuVarsPlugin").String("SOONG_KERNEL_MODULE")
	if module == "" {
		module = legacyKernelHeadersModule
	}
	if module == legacyKernelHeadersModule {
		ctx.AddDependency(ctx.Module(), kernelHeadersDepTag, module)
	} else {
		ctx.AddFarVariationDependencies(ctx.Config().AndroidFirstDeviceTarget.Variations(), kernelHeadersDepTag, module)
	}
}

func (m *kernelHeadersModule) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	ctx.VisitDirectDepsProxyWithTag(kernelHeadersDepTag, func(dep android.ModuleProxy) {
		commonInfo := android.OtherModulePointerProviderOrDefault(ctx, dep, android.CommonModuleInfoProvider)
		if commonInfo.GeneratedSource == nil {
			ctx.ModuleErrorf("%q does not provide generated kernel headers", ctx.OtherModuleName(dep))
			return
		}
		m.headerDirs = append(m.headerDirs, commonInfo.GeneratedSource.GeneratedHeaderDirs...)
		m.headerDeps = append(m.headerDeps, commonInfo.GeneratedSource.GeneratedDeps...)
	})
}

func (m *kernelHeadersModule) GeneratedHeaderDirs() android.Paths {
	return m.headerDirs
}

func (m *kernelHeadersModule) GeneratedSourceFiles() android.Paths {
	return nil
}

func (m *kernelHeadersModule) GeneratedDeps() android.Paths {
	return m.headerDeps
}

func (m *kernelModule) kernelInputs(ctx android.ModuleContext, kernelDir string) android.Paths {
	patterns := m.properties.Srcs
	var inputs android.Paths
	for _, pattern := range patterns {
		paths, err := ctx.GlobWithDeps(filepath.Join(kernelDir, pattern), []string{
			filepath.Join(kernelDir, ".git", "**/*"),
		})
		if err != nil {
			ctx.PropertyErrorf("srcs", "unable to glob %q: %s", pattern, err)
			continue
		}
		for _, path := range paths {
			if !strings.HasSuffix(path, "/") {
				inputs = append(inputs, android.PathForSource(ctx, path))
			}
		}
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].String() < inputs[j].String() })
	return android.FirstUniquePaths(inputs)
}

func (m *kernelModule) configPath(ctx android.ModuleContext, kernelDir, arch, config string) android.Path {
	if strings.Contains(config, "/") {
		return android.PathForSource(ctx, config)
	}
	configArch := arch
	if arch == "x86_64" {
		configArch = "x86"
	}
	return android.PathForSource(ctx, kernelDir, "arch", configArch, "configs", config)
}

func (m *kernelModule) makeInvocation(ctx android.ModuleContext, source, out, arch, target string) string {
	makeCommand := proptools.StringDefault(m.properties.Make_command, "prebuilts/build-tools/linux-x86/bin/make")
	top := ctx.Config().Getenv("TOP")
	absoluteToolPath := func(path string) string {
		if filepath.IsAbs(path) || top == "" {
			return path
		}
		return filepath.Join(top, path)
	}
	makeCommand = absoluteToolPath(makeCommand)
	jobs := (runtime.NumCPU() + 2) * 3 / 2
	if m.properties.Build_jobs != nil {
		jobs = int(*m.properties.Build_jobs)
	}
	if jobs < 1 {
		ctx.PropertyErrorf("build_jobs", "must be greater than zero")
		jobs = 1
	}
	source = absoluteToolPath(source)
	clangPath := proptools.String(m.properties.Clang_path)
	if clangPath == "" {
		version := proptools.String(m.properties.Clang_version)
		if version == "" {
			version = ctx.Config().Getenv("LLVM_AOSP_PREBUILTS_VERSION")
		}
		version = proptools.StringDefault(&version, "clang-stable")
		clangPath = filepath.Join("prebuilts/clang/host/linux-x86", version)
	}
	rustVersion := proptools.String(m.properties.Rust_version)
	if rustVersion == "" {
		rustVersion = ctx.Config().Getenv("RUST_AOSP_PREBUILTS_VERSION")
	}
	paths := []string{
		filepath.Join(clangPath, "bin"),
		"prebuilts/build-tools/linux-x86/bin",
		"prebuilts/kernel-build-tools/linux-x86/bin",
		"prebuilts/tools-lineage/linux-x86/bin",
	}
	if rustVersion != "" {
		paths = append(paths, filepath.Join("prebuilts/rust-toolchain/linux-x86", rustVersion, "bin"))
	}
	for i, path := range paths {
		paths[i] = absoluteToolPath(path)
	}
	flags := []string{
		"LLVM=1", "LLVM_IAS=1",
		"DTC_EXT=" + absoluteToolPath("out/host/linux-x86/bin/dtc"),
		"LZ4=" + absoluteToolPath("prebuilts/kernel-build-tools/linux-x86/bin/lz4"),
		"LEX=" + absoluteToolPath("prebuilts/build-tools/linux-x86/bin/flex"),
		"YACC=" + absoluteToolPath("prebuilts/build-tools/linux-x86/bin/bison"),
		"M4=" + absoluteToolPath("prebuilts/build-tools/linux-x86/bin/m4"),
		"PAHOLE=" + absoluteToolPath("prebuilts/kernel-build-tools/linux-x86/bin/pahole"),
		"LIBCLANG_PATH=" + absoluteToolPath(filepath.Join(clangPath, "lib")),
	}
	flags = append(flags, m.properties.Make_flags...)
	flags = append(flags, m.properties.Additional_flags...)
	triple := proptools.String(m.properties.Clang_triple)
	if triple == "" {
		switch arch {
		case "arm64":
			triple = "aarch64-linux-gnu-"
		case "arm":
			triple = "arm-linux-gnu-"
		case "x86", "x86_64":
			triple = "x86_64-linux-gnu-"
		}
	}
	if triple != "" {
		flags = append(flags, "CLANG_TRIPLE="+triple)
	}
	if cross := proptools.String(m.properties.Cross_compile); cross != "" {
		flags = append(flags, "CROSS_COMPILE="+cross)
	}
	compiler := proptools.StringDefault(m.properties.Cc, "clang")
	environment := []string{"PATH=" + strings.Join(paths, ":") + ":$PATH"}
	if rbeWrapper := proptools.String(m.properties.Rbe_wrapper); rbeWrapper != "" {
		compiler = absoluteToolPath("vendor/uwu/build/tools/kernel_rbe_cc.sh") + " " + compiler
		environment = append(environment,
			"RBE_exec_root="+proptools.ShellEscape(top),
			"KERNEL_RBE_WRAPPER="+proptools.ShellEscape(rbeWrapper),
		)
	}
	flags = append(flags,
		"CC="+compiler,
		"LD="+proptools.StringDefault(m.properties.Ld, "ld.lld"),
		"PERL5LIB="+absoluteToolPath("prebuilts/tools-lineage/common/perl-base"),
	)
	if m.autofdoProfile.Valid() {
		flags = append(flags, "CLANG_AUTOFDO_PROFILE="+m.autofdoProfile.Path().String())
	}
	for _, assignment := range m.properties.Environment {
		name, value, ok := strings.Cut(assignment, "=")
		if !ok || name == "" {
			ctx.PropertyErrorf("environment", "%q must be an environment assignment", assignment)
			continue
		}
		environment = append(environment, name+"="+proptools.ShellEscape(value))
	}
	return fmt.Sprintf("%s %s -j%d -C %s O=%s ARCH=%s %s %s",
		strings.Join(environment, " "), makeCommand, jobs, source, absoluteToolPath(out), arch,
		strings.Join(proptools.ShellEscapeList(flags), " "), target)
}

func topRelativePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return "$PWD/" + path
}

func (m *kernelModule) applyLto(cmd *android.RuleBuilderCommand, source, config string) {
	lto := proptools.String(m.properties.Config.Lto)
	if lto == "" {
		return
	}
	var args string
	switch lto {
	case "none":
		args = "-d LTO_CLANG -e LTO_NONE -d LTO_CLANG_THIN -d LTO_CLANG_FULL -d THINLTO"
	case "thin":
		args = "-e LTO_CLANG -d LTO_NONE -e LTO_CLANG_THIN -d LTO_CLANG_FULL -e THINLTO"
	case "full":
		args = "-e LTO_CLANG -d LTO_NONE -d LTO_CLANG_THIN -e LTO_CLANG_FULL -d THINLTO"
	default:
		return
	}
	cmd.Text("&&").Text(filepath.Join(source, "scripts", "config")).Text("--file").Text(config).Text(args)
}

func (m *kernelModule) buildDeviceTree(ctx android.ModuleContext, source, buildRoot, arch string, dependency android.Path, props deviceTreeProperties, overlay bool, inputs android.Paths) android.Path {
	name := "dtb.img"
	target := "dtbs"
	tag := "dtb"
	if overlay {
		name = "dtbo.img"
		target = "dtbo.img"
		tag = "dtbo"
	}
	name = proptools.StringDefault(props.Image_name, name)
	target = proptools.StringDefault(props.Target, target)
	output := android.PathForModuleOut(ctx, tag, name)
	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	cmd := rule.Command().Text("set -e;")
	if custom := proptools.String(props.Custom_command); custom != "" {
		expanded, err := android.Expand(custom, func(name string) (string, error) {
			switch name {
			case "kernelDir":
				return source, nil
			case "kernelOut":
				return buildRoot, nil
			case "out":
				return output.String(), nil
			default:
				return "", fmt.Errorf("unknown variable $(%s)", name)
			}
		})
		if err != nil {
			ctx.PropertyErrorf("%s.custom_command", tag, "%s", err)
			return output
		}
		cmd.Text(expanded)
	} else {
		cmd.Text(m.makeInvocation(ctx, source, buildRoot, arch, target)).Text("&& mkdir -p").Text(filepath.Dir(output.String())).Text("&&")
		candidate := filepath.Join(buildRoot, "arch", arch, "boot", name)
		cmd.Text("if [ -f").Text(candidate).Text("]; then cp").Text(candidate).Text(output.String()).Text("; else")
		pattern := "*.dtb"
		if overlay {
			pattern = "*.dtbo"
		}
		if len(props.Input_globs) > 0 {
			pattern = props.Input_globs[0]
		}
		patterns := []string{pattern}
		if len(props.Input_globs) > 1 {
			patterns = append(patterns, props.Input_globs[1:]...)
		}
		cmd.Text("files=$(")
		for i, configured := range patterns {
			if i > 0 {
				cmd.Text(";")
			}
			cmd.Textf("find %s/arch/%s/boot/dts -type f -name %s", buildRoot, arch, proptools.ShellEscape(configured))
		}
		cmd.Text("| sort -u); test -n \"$files\";")
		if props.Config != nil {
			cmd.Text("prebuilts/kernel-build-tools/linux-x86/bin/mkdtboimg cfg_create").Text(output.String()).Input(android.PathForModuleSrc(ctx, *props.Config)).Text("-d").Text(filepath.Join(buildRoot, "arch", arch, "boot", "dts"))
		} else if overlay {
			cmd.Text("prebuilts/kernel-build-tools/linux-x86/bin/mkdtboimg create").Text(output.String()).Textf("--page_size=%d $files", proptools.IntDefault(props.Page_size, 4096))
		} else {
			cmd.Text("cat $files >").Text(output.String())
		}
		cmd.Text("; fi")
	}
	cmd.ImplicitOutput(output).Implicit(dependency).Implicits(inputs)
	if props.Config != nil {
		cmd.Implicit(android.PathForModuleSrc(ctx, *props.Config))
	}
	rule.Build("kernel_"+tag, "Kernel "+strings.ToUpper(tag))
	return output
}

func (m *kernelModule) buildModules(ctx android.ModuleContext, source, buildRoot, arch string, dependency android.Path, inputs android.Paths) android.Path {
	staging := android.PathForModuleOut(ctx, "modules_staging")
	output := android.PathForModuleOut(ctx, "kernel_modules.zip")
	rule := android.NewRuleBuilder(pctx, ctx).SandboxDisabled()
	cmd := rule.Command().Text("set -e; rm -rf").Text(staging.String()).Text("&& mkdir -p").Text(staging.String()).Text("&&")
	targets := m.properties.Modules.Build_targets
	if len(targets) == 0 {
		targets = []string{"modules"}
	}
	for _, target := range targets {
		cmd.Text(m.makeInvocation(ctx, source, buildRoot, arch, target)).Text("&&")
	}
	strip := "0"
	if proptools.BoolDefault(m.properties.Modules.Install_strip, true) {
		strip = "1"
	}
	cmd.Text(m.makeInvocation(ctx, source, buildRoot, arch, fmt.Sprintf("INSTALL_MOD_PATH=%s INSTALL_MOD_STRIP=%s modules_install", topRelativePath(staging.String()), strip)))
	if len(m.properties.Modules.External_modules) > 0 {
		root := filepath.Clean(proptools.String(m.properties.Modules.External_module_root))
		if root == "." || root == "" || filepath.IsAbs(root) || strings.HasPrefix(root, "..") {
			ctx.PropertyErrorf("modules.external_module_root", "must be set to a source-root-relative directory")
		} else {
			relativeRoot, err := filepath.Rel(filepath.Clean(source), root)
			if err != nil {
				ctx.PropertyErrorf("modules.external_module_root", "%s", err)
			} else {
				for _, entry := range m.properties.Modules.External_modules {
					parts := strings.Split(entry, ":")
					if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] != "kbuild") {
						ctx.PropertyErrorf("modules.external_modules", "%q must be path or path:kbuild", entry)
						continue
					}
					module := filepath.Clean(parts[0])
					if filepath.IsAbs(module) || strings.HasPrefix(module, "..") {
						ctx.PropertyErrorf("modules.external_modules", "%q must be relative to external_module_root", entry)
						continue
					}
					moduleRoot := filepath.Join(root, module)
					moduleM := filepath.Join(relativeRoot, module)
					if len(parts) == 2 {
						cmd.Text("&&").Text(m.makeInvocation(ctx, source, buildRoot, arch, "M="+moduleM+" modules"))
						cmd.Text("&&").Text(m.makeInvocation(ctx, source, buildRoot, arch, fmt.Sprintf("M=%s INSTALL_MOD_PATH=%s INSTALL_MOD_STRIP=%s modules_install", moduleM, topRelativePath(staging.String()), strip)))
					} else {
						common := fmt.Sprintf("M=%s KERNEL_SRC=%s OUT_DIR=%s", moduleM, topRelativePath(source), topRelativePath(buildRoot))
						cmd.Text("&&").Text(m.makeInvocation(ctx, moduleRoot, buildRoot, arch, common+" modules"))
						cmd.Text("&&").Text(m.makeInvocation(ctx, moduleRoot, buildRoot, arch, fmt.Sprintf("%s INSTALL_MOD_PATH=%s INSTALL_MOD_STRIP=%s KERNEL_UAPI_HEADERS_DIR=%s modules_install", common, topRelativePath(staging.String()), strip, topRelativePath(buildRoot))))
					}
				}
			}
		}
	}
	for _, alias := range m.properties.Modules.Module_aliases {
		parts := strings.Split(alias, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || filepath.Base(parts[1]) != parts[1] {
			ctx.PropertyErrorf("modules.module_aliases", "%q must be old.ko:new.ko", alias)
			continue
		}
		cmd.Text("&& set -- $(find").Text(staging.String()).Text("-type f -path").Text(proptools.ShellEscape("*/" + parts[0])).Text("); test \"$#\" -eq 1")
		cmd.Text("&& mv \"$1\" \"$(dirname \"$1\")/").Text(parts[1]).Text("\"")
	}
	flat := android.PathForModuleOut(ctx, "modules_flat")
	cmd.Text("&& rm -rf").Text(flat.String()).Text("&& mkdir -p").Text(flat.String())
	cmd.Text("&& find").Text(staging.String()).Text("-type f -name '*.ko' -print0 | while IFS= read -r -d '' module; do name=$(basename \"$module\"); test ! -e").Text(flat.String()).Text("/\"$name\" || { echo \"duplicate kernel module $name\" >&2; exit 1; }; cp \"$module\"").Text(flat.String()).Text("/\"$name\"; done")
	moduleSets := []struct {
		name        string
		installList []string
		loadList    []string
		blocklist   *string
	}{
		{"system_dlkm", m.properties.Modules.System_dlkm_module_install_list, m.properties.Modules.System_dlkm_module_load_list, m.properties.Modules.System_dlkm_module_blocklist},
		{"vendor_dlkm", m.properties.Modules.Vendor_dlkm_module_install_list, m.properties.Modules.Vendor_dlkm_module_load_list, m.properties.Modules.Vendor_dlkm_module_blocklist},
		{"vendor_ramdisk", m.properties.Modules.Vendor_ramdisk_module_install_list, m.properties.Modules.Vendor_ramdisk_module_load_list, m.properties.Modules.Vendor_ramdisk_module_blocklist},
		{"recovery", m.properties.Modules.Recovery_module_install_list, m.properties.Modules.Recovery_module_load_list, m.properties.Modules.Recovery_module_blocklist},
	}
	for _, set := range moduleSets {
		if len(set.installList) == 0 && len(set.loadList) == 0 {
			continue
		}
		installFile := "modules.install." + set.name
		loadFile := "modules.load." + set.name
		m.writeModuleListFile(ctx, cmd, flat.String(), installFile, set.installList)
		m.writeModuleLoadFile(ctx, cmd, flat.String(), set.name, set.loadList)
		cmd.Text("&& missing=$(comm -23 <(sort -u").Text(filepath.Join(flat.String(), loadFile)).Text(") <(sort -u").Text(filepath.Join(flat.String(), installFile)).Text(")); test -z \"$missing\" || { echo \"modules in load list but not install list: $missing\" >&2; exit 1; }")
		m.writeModuleBlocklist(ctx, cmd, flat.String(), set.name, set.blocklist)
	}
	cmd.Text("&& prebuilts/build-tools/linux-x86/bin/soong_zip -C").Text(flat.String()).Text("-D").Text(flat.String()).Text("-o").Output(output)
	cmd.Implicit(dependency).Implicits(inputs)
	rule.Build("kernel_modules", "Kernel modules")
	return output
}

func (m *kernelModule) writeModuleBlocklist(ctx android.ModuleContext, cmd *android.RuleBuilderCommand, dir, name string, src *string) {
	if src == nil {
		return
	}
	cmd.Text("&& cp").Input(android.PathForSource(ctx, *src)).Text(filepath.Join(dir, "modules.blocklist."+name))
}

func (m *kernelModule) writeModuleLoadFile(ctx android.ModuleContext, cmd *android.RuleBuilderCommand, dir, name string, loadFiles []string) {
	path := filepath.Join(dir, "modules.load."+name)
	cmd.Text("&& : >").Text(path)
	m.appendModuleFiles(ctx, cmd, path, loadFiles)
	m.validateModuleFile(cmd, dir, path, true)
	cmd.Text("&& awk '!seen[$0]++'").Text(path).Text(">").Text(path + ".tmp").Text("&& mv").Text(path + ".tmp").Text(path)
}

func (m *kernelModule) writeModuleListFile(ctx android.ModuleContext, cmd *android.RuleBuilderCommand, dir, name string, moduleFiles []string) {
	path := filepath.Join(dir, name)
	cmd.Text("&& : >").Text(path)
	m.appendModuleFiles(ctx, cmd, path, moduleFiles)
	m.validateModuleFile(cmd, dir, path, false)
	if proptools.Bool(m.properties.Modules.Auto_collect_deps) && len(moduleFiles) > 0 {
		collectScript := android.PathForSource(ctx, "lineage/scripts/collect-kernel-module-deps/collect-kernel-module-deps.py")
		cmd.Text("&& python3").Input(collectScript).Text("--non-interactive").Text(dir)
		cmd.Text("$(cat").Text(path).Text(")")
		cmd.Text(">>").Text(path)
	}
	cmd.Text("&& sort -u -o").Text(path).Text(path)
}

func (m *kernelModule) appendModuleFiles(ctx android.ModuleContext, cmd *android.RuleBuilderCommand, output string, srcs []string) {
	for _, src := range srcs {
		cmd.Text("&& awk '{ sub(/#.*/, \"\"); if (NF) { name=$NF; sub(/^.*\\//, \"\", name); print name } }'").Input(android.PathForSource(ctx, src)).Text(">>").Text(output)
	}
}

func (m *kernelModule) validateModuleFile(cmd *android.RuleBuilderCommand, dir, path string, allowMissing bool) {
	cmd.Text("&& while IFS= read -r module; do test -z \"$module\" || test -f").Text(filepath.Join(dir, "\"$module\""))
	if allowMissing && proptools.Bool(m.properties.Modules.Allow_missing_load) {
		cmd.Text("|| echo \"warning: kernel module $module was not found\" >&2")
	} else {
		cmd.Text("|| { echo \"kernel module $module was not found\" >&2; exit 1; }")
	}
	cmd.Text("; done <").Text(path)
}

func (m *kernelModule) AndroidMkEntries() []android.AndroidMkEntries {
	if m.kernelImage == nil || !m.androidMkEnabled {
		return nil
	}
	return []android.AndroidMkEntries{{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(m.kernelImage),
		ExtraEntries: []android.AndroidMkExtraEntriesFunc{
			func(ctx android.AndroidMkExtraEntriesContext, entries *android.AndroidMkEntries) {
				entries.SetString("LOCAL_MODULE_PATH", m.installDir.String())
				entries.SetString("LOCAL_INSTALLED_MODULE_STEM", "kernel")
			},
		},
	}}
}
