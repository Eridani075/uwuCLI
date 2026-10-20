# SPDX-FileCopyrightText: The uwuAOSP Project
# SPDX-License-Identifier: Apache-2.0

"""Regression tests for radio conversion without invoking the interactive CLI."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class VendorRadioTest(unittest.TestCase):
    def test_partition_names_are_separate_from_namespaced_modules(self):
        source = Path(__file__).with_name("uwu").read_text()
        functions = source[source.index("vendor_root_from_device_dir() {"):
                           source.index("radio_usage() {")]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            vendor = root / "vendor/oneplus/hummer"
            (vendor / "radio").mkdir(parents=True)
            (vendor / "Android.mk").write_text(
                "$(call add-radio-file-sha1-checked,radio/pvmfw.img,abc)\n"
                "$(call add-radio-file-sha1-checked,radio/modem.img,def)\n"
            )
            board = vendor / "BoardConfigVendor.mk"
            board.write_text("AB_OTA_PARTITIONS += \\\n    pvmfw \\\n    modem\n")
            product = vendor / "hummer-vendor.mk"
            product.write_text("# Existing vendor configuration\n")
            script = """set -Eeuo pipefail
SOURCE_ROOT="$TEST_ROOT"
GREEN='' YELLOW=''
relative_path() { printf '%s' "${1#"$SOURCE_ROOT/"}"; }
status_line() { :; }
msg() { :; }
""" + functions + '\nvendor_convert_radio "$SOURCE_ROOT/device/oneplus/hummer"\n'
            subprocess.run(["bash", "-c", script], check=True,
                           env={**os.environ, "TEST_ROOT": tmp})
            bp = (vendor / "radio/Android.bp").read_text()
            self.assertIn("soong_namespace {}", bp)
            for partition in ("pvmfw", "modem"):
                self.assertIn(f'name: "uwu_radio_{partition}"', bp)
                self.assertIn(f'filename: "{partition}.img"', bp)
                self.assertIn(
                    f"$(call soong_config_set,uwuRadio,{partition},"
                    f"//vendor/oneplus/hummer/radio:uwu_radio_{partition})",
                    board.read_text(),
                )
                self.assertIn(f"    uwu_radio_{partition}", product.read_text())
            self.assertIn("AB_OTA_RADIO_PARTITIONS += \\\n    pvmfw \\\n    modem\n", board.read_text())
            self.assertIn("PRODUCT_SOONG_NAMESPACES += vendor/oneplus/hummer/radio", product.read_text())
            self.assertNotIn("soong_config_set", bp)
            self.assertFalse((vendor / "Android.mk").exists())
            before = {p: p.read_bytes() for p in (board, product, vendor / "radio/Android.bp")}
            subprocess.run(["bash", "-c", script], check=True,
                           env={**os.environ, "TEST_ROOT": tmp})
            self.assertEqual(before, {p: p.read_bytes() for p in before})


if __name__ == "__main__":
    unittest.main()
