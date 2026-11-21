package main

import "testing"

func TestClassifyStationFilesAreShown(t *testing.T) {
	names := []string{
		"niagara_user_home/stations/demoPlant/config.bog",
		"niagara_user_home/stations/demoPlant/shared/px/AHU1.px",
		"niagara_user_home/stations/demoPlant/shared/nav/demoPlant.nav",
		"niagara_user_home/stations/demoPlant/shared/palette/demoPlant.palette",
	}
	for _, name := range names {
		got, display := classify(name)
		if got != kindShow || display != name {
			t.Fatalf("%s classified as %s %q", name, got, display)
		}
	}
}

func TestClassifyNoise(t *testing.T) {
	cases := map[string]string{
		"niagara_user_home/stations/demoPlant/history/AHU1_SupplyTemp.bin":     "history",
		"niagara_user_home/stations/demoPlant/alarm/alarmData.bin":             "alarm",
		"niagara_home/logs/station.out":                                        "logs",
		"niagara_user_home/stations/demoPlant/config.backup.20200101_0000.bog": "config.backup",
		"stations/demoPlant/config_backup_200548_0042.bog":                     "config.backup",
		"shared/notes.log": "*.log",
	}
	for name, category := range cases {
		got, display := classify(name)
		if got != kindNoisy {
			t.Fatalf("%s classified as %s", name, got)
		}
		if noisyCategory(display) != category {
			t.Fatalf("%s category %s, want %s", name, noisyCategory(display), category)
		}
	}
}

func TestClassifySecretsWinOverLogs(t *testing.T) {
	if got, _ := classify("niagara_home/logs/keystore.jceks"); got != kindSecret {
		t.Fatalf("keystore in logs classified as %s", got)
	}
	for _, name := range []string{
		"niagara_user_home/security/keystore.jceks",
		"niagara_user_home/security/cacerts.jceks",
		"niagara_user_home/security/exemptions.tes",
		"niagara_user_home/certManagement/demo-client.pem",
		"exported/client.p12",
		"exported/client.bcfks",
	} {
		if got, _ := classify(name); got != kindSecret {
			t.Fatalf("%s classified as %s", name, got)
		}
	}
}

func TestClassifyConfigBogIsNotABackup(t *testing.T) {
	if got, _ := classify("stations/demoPlant/config.bog"); got != kindShow {
		t.Fatal(got)
	}
}

func TestClassifyUnsafePaths(t *testing.T) {
	for _, name := range []string{"../outside.pem", "/etc/passwd", "C:/Users/secret.pem"} {
		if got, _ := classify(name); got != kindUnsafe {
			t.Fatalf("%s classified as %s", name, got)
		}
	}
	got, display := classify("stations//demo/./config.bog")
	if got != kindShow || display != "stations/demo/config.bog" {
		t.Fatalf("normalized to %s %q", got, display)
	}
}
