"""Skip rules for station backup members."""

from __future__ import annotations

import unittest

from tests.support import ROOT  # noqa: F401

from niagara_git.classify import classify, noisy_category


class ClassifyTests(unittest.TestCase):
    def test_station_files_are_shown(self) -> None:
        for name in (
            "niagara_user_home/stations/demoPlant/config.bog",
            "niagara_user_home/stations/demoPlant/shared/px/AHU1.px",
            "niagara_user_home/stations/demoPlant/shared/nav/demoPlant.nav",
            "niagara_user_home/stations/demoPlant/shared/palette/demoPlant.palette",
        ):
            kind, display = classify(name)
            self.assertEqual(kind, "show", name)
            self.assertEqual(display, name)

    def test_history_alarm_logs_and_backups_are_noisy(self) -> None:
        cases = {
            "niagara_user_home/stations/demoPlant/history/AHU1_SupplyTemp.bin": "history",
            "niagara_user_home/stations/demoPlant/alarm/alarmData.bin": "alarm",
            "niagara_home/logs/station.out": "logs",
            "niagara_user_home/stations/demoPlant/config.backup.20200101_0000.bog": "config.backup",
            "stations/demoPlant/config_backup_200548_0042.bog": "config.backup",
            "shared/notes.log": "*.log",
        }
        for name, category in cases.items():
            kind, display = classify(name)
            self.assertEqual(kind, "noisy", name)
            self.assertEqual(noisy_category(display), category)

    def test_keystores_are_secret_even_inside_logs(self) -> None:
        kind, _display = classify("niagara_home/logs/keystore.jceks")
        self.assertEqual(kind, "secret")
        for name in (
            "niagara_user_home/security/keystore.jceks",
            "niagara_user_home/security/cacerts.jceks",
            "niagara_user_home/security/exemptions.tes",
            "niagara_user_home/certManagement/demo-client.pem",
            "exported/client.p12",
            "exported/client.bcfks",
        ):
            self.assertEqual(classify(name)[0], "secret", name)

    def test_config_bog_is_not_a_backup(self) -> None:
        self.assertEqual(classify("stations/demoPlant/config.bog")[0], "show")

    def test_unsafe_paths_are_not_normalized_away(self) -> None:
        self.assertEqual(classify("../outside.pem")[0], "unsafe")
        self.assertEqual(classify("/etc/passwd")[0], "unsafe")
        self.assertEqual(classify("C:/Users/secret.pem")[0], "unsafe")
        kind, display = classify("stations//demo/./config.bog")
        self.assertEqual(kind, "show")
        self.assertEqual(display, "stations/demo/config.bog")


if __name__ == "__main__":
    unittest.main()
