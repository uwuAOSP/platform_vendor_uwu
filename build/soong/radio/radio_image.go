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

package radio

import (
	"github.com/google/blueprint/proptools"

	"android/soong/android"
)

var pctx = android.NewPackageContext("uwu/soong/radio")

func init() {
	android.RegisterModuleType("uwu_prebuilt_image", PrebuiltImageFactory)
}

type prebuiltImageProperties struct {
	// Path to the prebuilt image relative to the module directory.
	Src *string `android:"path"`

	// Optional expected SHA-1 of the source file. When set, the build
	// fails if the actual digest does not match.
	Sha1 *string
}

type prebuiltImage struct {
	android.ModuleBase
	properties prebuiltImageProperties

	outputPath android.Path
	installDir android.InstallPath
}

func PrebuiltImageFactory() android.Module {
	m := &prebuiltImage{}
	m.AddProperties(&m.properties)
	android.InitAndroidArchModule(m, android.DeviceSupported, android.MultilibFirst)
	return m
}

func (p *prebuiltImage) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	if p.properties.Src == nil {
		ctx.PropertyErrorf("src", "Source cannot be empty")
		return
	}

	srcPath := android.PathForModuleSrc(ctx, proptools.String(p.properties.Src))
	outputPath := android.PathForModuleOut(ctx, ctx.ModuleName()+".img")

	builder := android.NewRuleBuilder(pctx, ctx)

	if p.properties.Sha1 != nil {
		expected := proptools.String(p.properties.Sha1)
		checkCmd := builder.Command()
		checkCmd.Textf(`test "$(sha1sum %s | cut -d' ' -f1)" = "%s"`,
			checkCmd.PathForInput(srcPath), expected).
			Implicit(srcPath).
			Text("&& cp").Text(checkCmd.PathForInput(srcPath)).Output(outputPath)
	} else {
		builder.Command().Text("cp").Text(srcPath.String()).Implicit(srcPath).Output(outputPath)
	}
	builder.Build("copy_prebuilt_image", "copy prebuilt image")

	p.outputPath = outputPath
	p.installDir = android.PathForModuleInPartitionInstall(ctx, "")
	ctx.SetOutputFiles(android.Paths{outputPath}, "")
}

func (p *prebuiltImage) AndroidMkEntries() []android.AndroidMkEntries {
	if p.outputPath == nil {
		return nil
	}
	return []android.AndroidMkEntries{{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(p.outputPath),
		ExtraEntries: []android.AndroidMkExtraEntriesFunc{
			func(ctx android.AndroidMkExtraEntriesContext, entries *android.AndroidMkEntries) {
				entries.SetString("LOCAL_MODULE_PATH", p.installDir.String())
				entries.SetString("LOCAL_INSTALLED_MODULE_STEM", p.outputPath.Base())
			},
		},
	}}
}
