/*
 * SPDX-FileCopyrightText: The uwuAOSP Project
 * SPDX-License-Identifier: Apache-2.0
 */

package uni

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallableAPKPathsUsesAppModuleInfo(t *testing.T) {
	root := t.TempDir()
	productOut := filepath.Join(root, "out", "target", "product", "test")
	apk := filepath.Join(productOut, "system_ext", "priv-app", "SystemUI", "SystemUI.apk")
	if err := os.MkdirAll(filepath.Dir(apk), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(apk, []byte("apk"), 0600); err != nil {
		t.Fatal(err)
	}
	modules := map[string]installModuleInfo{
		"SystemUI": {
			Class: []string{"APPS"},
			Installed: []string{
				"out/target/product/test/system_ext/priv-app/SystemUI/SystemUI.apk",
				"out/target/product/test/system_ext/priv-app/SystemUI/oat/arm64/SystemUI.odex",
			},
		},
	}
	data, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(productOut, "module-info.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	paths, err := installableAPKPaths(root, productOut, "SystemUI")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{apk}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("APK paths = %v, want %v", paths, want)
	}
}

func TestInstallableAPKPathsRejectsNonAppAndMissingInstalledAPK(t *testing.T) {
	root := t.TempDir()
	productOut := filepath.Join(root, "product")
	modules := map[string]installModuleInfo{
		"Library":      {Class: []string{"JAVA_LIBRARIES"}},
		"NotInstalled": {Class: []string{"APPS"}},
	}
	data, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(productOut, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(productOut, "module-info.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"Library", "NotInstalled", "Missing"} {
		if _, err := installableAPKPaths(root, productOut, module); err == nil {
			t.Fatalf("module %q unexpectedly resolved to an installable APK", module)
		}
	}
}

func TestSelectADBDevice(t *testing.T) {
	devices := []adbDevice{
		{serial: "offline-device", state: "offline"},
		{serial: "device-a", state: "device"},
		{serial: "unauthorized-device", state: "unauthorized"},
	}
	if got, err := selectADBDevice(devices, ""); err != nil || got != "device-a" {
		t.Fatalf("selectADBDevice = %q, %v; want device-a", got, err)
	}
	if got, err := selectADBDevice(devices, "device-a"); err != nil || got != "device-a" {
		t.Fatalf("selectADBDevice with explicit serial = %q, %v", got, err)
	}
	if _, err := selectADBDevice(append(devices, adbDevice{serial: "device-b", state: "device"}), ""); err == nil {
		t.Fatal("multiple online devices were accepted without a selected serial")
	}
	if _, err := selectADBDevice(devices, "offline-device"); err == nil {
		t.Fatal("offline device was accepted")
	}
	if _, err := selectADBDevice(nil, ""); err == nil {
		t.Fatal("empty device list was accepted")
	}
}
