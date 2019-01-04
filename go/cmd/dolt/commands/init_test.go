package commands

import (
	"github.com/liquidata-inc/ld/dolt/go/libraries/env"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		Name          string
		Args          []string
		GlobalConfig  map[string]string
		ExpectSuccess bool
	}{
		{
			"Command Line name and email",
			[]string{"-name", "Bill Billerson", "-email", "bigbillieb@fake.horse"},
			map[string]string{},
			true,
		},
		{
			"Global config name and email",
			[]string{},
			map[string]string{
				env.UserNameKey:  "Bill Billerson",
				env.UserEmailKey: "bigbillieb@fake.horse",
			},
			true,
		},
		{
			"No Name",
			[]string{"-email", "bigbillieb@fake.horse"},
			map[string]string{},
			false,
		},
		{
			"No Email",
			[]string{"-name", "Bill Billerson"},
			map[string]string{},
			false,
		},
	}

	for _, test := range tests {
		dEnv := createUninitializedEnv()
		gCfg, _ := dEnv.Config.GetConfig(env.GlobalConfig)
		gCfg.SetStrings(test.GlobalConfig)

		result := Init("dolt init", test.Args, dEnv)

		if (result == 0) != test.ExpectSuccess {
			t.Error(test.Name, "- Expected success:", test.ExpectSuccess, "result:", result == 0)
		} else if test.ExpectSuccess {
			// succceeded as expected
			if !dEnv.HasLDDir() {
				t.Error(test.Name, "- .dolt dir should exist after initialization")
			}
		} else {
			// failed as expected
			if !dEnv.IsCWDEmpty() {
				t.Error(test.Name, "- CWD should be empty after failure to initialize... unless it wasn't empty to start with")
			}
		}
	}
}

func TestInitTwice(t *testing.T) {
	dEnv := createUninitializedEnv()
	result := Init("dolt init", []string{"-name", "Bill Billerson", "-email", "bigbillieb@fake.horse"}, dEnv)

	if result != 0 {
		t.Error("First init should succeed")
	}

	result = Init("dolt init", []string{"-name", "Bill Billerson", "-email", "bigbillieb@fake.horse"}, dEnv)

	if result == 0 {
		t.Error("First init should succeed")
	}
}

func TestInitWithNonEmptyDir(t *testing.T) {
	dEnv := createUninitializedEnv()
	dEnv.FS.WriteFile("file.txt", []byte("file contents."))
	result := Init("dolt init", []string{"-name", "Bill Billerson", "-email", "bigbillieb@fake.horse"}, dEnv)

	if result == 0 {
		t.Error("Init should fail if directory is not empty")
	}
}
