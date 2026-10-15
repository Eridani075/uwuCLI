/*
 * SPDX-FileCopyrightText: The uwuAOSP Project
 * SPDX-License-Identifier: Apache-2.0
 */

package uni

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type installModuleInfo struct {
	Class     []string `json:"class"`
	Installed []string `json:"installed"`
}

type adbDevice struct {
	serial string
	state  string
}

func installableAPKPaths(sourceRoot, productOut, module string) ([]string, error) {
	moduleInfoPath := filepath.Join(productOut, "module-info.json")
	data, err := os.ReadFile(moduleInfoPath)
	if err != nil {
		return nil, fmt.Errorf("read Soong module info %s: %w", moduleInfoPath, err)
	}
	var modules map[string]installModuleInfo
	if err := json.Unmarshal(data, &modules); err != nil {
		return nil, fmt.Errorf("parse Soong module info %s: %w", moduleInfoPath, err)
	}
	info, ok := modules[module]
	if !ok {
		return nil, fmt.Errorf("module %q is not present in %s", module, moduleInfoPath)
	}
	isApp := false
	for _, class := range info.Class {
		if class == "APPS" {
			isApp = true
			break
		}
	}
	if !isApp {
		return nil, fmt.Errorf("module %q is not an APK app module", module)
	}

	paths := make([]string, 0, len(info.Installed))
	for _, installed := range info.Installed {
		if !strings.EqualFold(filepath.Ext(installed), ".apk") {
			continue
		}
		path := installed
		if !filepath.IsAbs(path) {
			path = filepath.Join(sourceRoot, path)
		}
		path = filepath.Clean(path)
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("APK output for module %q is unavailable at %s: %w", module, path, err)
		}
		if !fileInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("APK output for module %q is not a regular file: %s", module, path)
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("module %q is an app but has no installed APK in product %s", module, productOut)
	}
	return paths, nil
}

func parseADBDevices(output string) []adbDevice {
	var devices []adbDevice
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "List" {
			continue
		}
		devices = append(devices, adbDevice{serial: fields[0], state: fields[1]})
	}
	return devices
}

func selectADBDevice(devices []adbDevice, requested string) (string, error) {
	var online []string
	var statuses []string
	for _, device := range devices {
		if device.state == "device" {
			online = append(online, device.serial)
		}
		statuses = append(statuses, device.serial+" ("+device.state+")")
	}
	if requested != "" {
		for _, serial := range online {
			if serial == requested {
				return requested, nil
			}
		}
		return "", fmt.Errorf("ANDROID_SERIAL %q is not an online device; devices: %s", requested, strings.Join(statuses, ", "))
	}
	switch len(online) {
	case 0:
		if len(statuses) == 0 {
			return "", fmt.Errorf("no online Android devices found; check `adb devices`")
		}
		return "", fmt.Errorf("no online Android devices found; devices: %s", strings.Join(statuses, ", "))
	case 1:
		return online[0], nil
	default:
		return "", fmt.Errorf("multiple Android devices are online; set ANDROID_SERIAL to one of: %s", strings.Join(online, ", "))
	}
}

func installTargetAPK(ctx context.Context, sourceRoot string, state State, module, requestedSerial string) error {
	paths, err := installableAPKPaths(sourceRoot, state.ProductOut, module)
	if err != nil {
		return err
	}
	adb, err := exec.LookPath("adb")
	if err != nil {
		return fmt.Errorf("find adb: %w", err)
	}
	deviceOutput, err := exec.CommandContext(ctx, adb, "devices").CombinedOutput()
	if err != nil {
		return fmt.Errorf("list Android devices: %w: %s", err, strings.TrimSpace(string(deviceOutput)))
	}
	if strings.TrimSpace(requestedSerial) == "" {
		requestedSerial = strings.TrimSpace(os.Getenv("ANDROID_SERIAL"))
	}
	serial, err := selectADBDevice(parseADBDevices(string(deviceOutput)), requestedSerial)
	if err != nil {
		return err
	}

	args := []string{"-s", serial, "install", "-r"}
	if len(paths) > 1 {
		args = []string{"-s", serial, "install-multiple", "-r"}
	}
	args = append(args, paths...)
	fmt.Printf("uni: installing %d APK file(s) for %s on %s\n", len(paths), module, serial)
	command := exec.CommandContext(ctx, adb, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("adb install failed: %w", err)
	}
	return nil
}
