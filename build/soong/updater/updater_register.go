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

package updater

import (
	"strings"

	"android/soong/android"
)

func init() {
	android.RegisterModuleType("uwu_updater_register", UpdaterRegisterFactory)
}

// uwu_updater_register generates the register.inc file that declares the
// RegisterDeviceExtensions() function, calling each device-specific
// Register_<lib>() extension registered via TARGET_RECOVERY_UPDATER_LIBS.
type updaterRegister struct {
	android.ModuleBase

	exportedIncludeDirs android.Paths
}

func UpdaterRegisterFactory() android.Module {
	m := &updaterRegister{}
	android.InitAndroidModule(m)
	return m
}

// These three methods satisfy genrule.SourceFileGenerator, which lets a
// cc module consume the generated register.inc as a generated header.
func (u *updaterRegister) GeneratedHeaderDirs() android.Paths {
	return u.exportedIncludeDirs
}

func (u *updaterRegister) GeneratedSourceFiles() android.Paths {
	return nil
}

func (u *updaterRegister) GeneratedDeps() android.Paths {
	return nil
}

func (u *updaterRegister) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	genDir := android.PathForModuleGen(ctx)
	u.exportedIncludeDirs = android.Paths{genDir}

	libs := ctx.Config().VendorConfig("recovery").String("target_recovery_updater_libs")
	libList := strings.Fields(libs)

	var b strings.Builder
	for _, lib := range libList {
		b.WriteString("extern void Register_" + lib + "(void);\n")
	}
	b.WriteString("\nvoid RegisterDeviceExtensions() {\n")
	for _, lib := range libList {
		b.WriteString("  Register_" + lib + "();\n")
	}
	b.WriteString("}\n")

	android.WriteFileRule(ctx, android.PathForModuleGen(ctx, "register.inc"), b.String())
}
