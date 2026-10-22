import json
from pathlib import Path
import shutil
import tempfile
import unittest

from prepare_release import prepare


class PrepareReleaseTests(unittest.TestCase):
    def test_syncs_release_tag_into_build_and_installers(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = Path(__file__).resolve().parent
            for name in ("config.yml", "darwin/Info.plist", "windows/info.json", "windows/nsis/project.nsi", "windows/nsis/wails_tools.nsh", "linux/nfpm/nfpm.yaml"):
                target = root / "build" / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(source / name, target)
            prepare(root, "v1.2.3-beta.1")
            self.assertIn('  version: "1.2.3-beta.1"', (root / "build/config.yml").read_text())
            self.assertEqual((root / "build/darwin/Info.plist").read_text().count("<string>1.2.3</string>"), 2)
            info = json.loads((root / "build/windows/info.json").read_text())
            self.assertEqual(info["fixed"]["file_version"], "1.2.3")
            self.assertEqual(info["info"]["0000"]["ProductVersion"], "1.2.3-beta.1")
            self.assertIn('INFO_PRODUCTVERSION "1.2.3"', (root / "build/windows/nsis/project.nsi").read_text())
            self.assertIn('version: "1.2.3-beta.1"', (root / "build/linux/nfpm/nfpm.yaml").read_text())

    def test_rejects_non_version_tags_before_writing(self):
        for tag in ("latest", "v1.2", "1.02.3", "v1.2.3/evil", "v1.2.3\n", "v1.2.3-..", "v1.2.3-01"):
            with self.assertRaises(ValueError):
                prepare(Path("/unused"), tag)


if __name__ == "__main__":
    unittest.main()
