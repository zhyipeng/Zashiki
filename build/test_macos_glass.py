from pathlib import Path
import re
import shlex
import shutil
import subprocess
import unittest


ROOT = Path(__file__).resolve().parent.parent


@unittest.skipUnless(shutil.which("task"), "requires go-task")
class MacOSGlassBuildTests(unittest.TestCase):
    def test_native_builds_enable_webview_transparency_and_preserve_extra_tags(self):
        for dev in ("true", "false"):
            for extra in ("", "custom_feature"):
                with self.subTest(dev=dev, extra=extra):
                    result = subprocess.run(
                        ["task", "--dry", "darwin:build", f"DEV={dev}", f"EXTRA_TAGS={extra}"],
                        cwd=ROOT, capture_output=True, text=True, check=True,
                    )
                    command = next(
                        line.split("] ", 1)[1]
                        for line in (result.stdout + result.stderr).splitlines()
                        if "] go build " in line
                    )
                    args = shlex.split(command)
                    tags = args[args.index("-tags") + 1].split(",")
                    self.assertIn("private_mac_apis", tags)
                    self.assertEqual("production" in tags, dev != "true")
                    if extra:
                        self.assertIn(extra, tags)

    def test_cross_build_enables_private_apis_only_on_macos(self):
        dockerfile = (ROOT / "build/docker/Dockerfile.cross").read_text()
        tag_script = re.search(r'TAGS="production"\n(.*?)\nCOMPILER=', dockerfile, re.S)
        self.assertIsNotNone(tag_script)
        for platform in ("darwin", "windows", "linux"):
            with self.subTest(platform=platform):
                result = subprocess.run(
                    ["sh", "-c", 'GOOS="$1"; EXTRA_TAGS=custom_feature; TAGS=production\n'
                     + tag_script.group(1) + '\nprintf "%s" "$TAGS"', "test", platform],
                    capture_output=True, text=True, check=True,
                )
                tags = result.stdout.split(",")
                self.assertEqual("private_mac_apis" in tags, platform == "darwin")
                self.assertIn("production", tags)
                self.assertIn("custom_feature", tags)


if __name__ == "__main__":
    unittest.main()
