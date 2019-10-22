package keyidentifier

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolide/kit/logutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type spec struct {
	KeyPath                   string
	ExpectedFingerprintSHA256 string
	ExpectedFingerprintMD5    string
	Encrypted                 bool
	Bits                      int
	Type                      string
	Format                    string
	Source                    string
}

// TestIdentifyFiles walks the testdata directory, and tests each
// file.
func TestIdentifyFiles(t *testing.T) {
	kIdentifier, err := New(WithLogger(logutil.NewCLILogger(true)))
	require.NoError(t, err)

	testFiles := []string{}

	err = filepath.Walk("testdata/specs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "failure to access path in filepath.Walk")
		}

		if info.IsDir() {
			return nil
		}

		// all json files in testdata/specs are assumed to be valid test specifications
		if strings.HasSuffix(path, ".json") {
			testFiles = append(testFiles, path)
			return nil
		}

		return nil
	})
	require.NoError(t, err, "filepath.Walk")
	for _, specPath := range testFiles {
		testIdentifyFile(t, kIdentifier, specPath)
	}
}

func testIdentifyFile(t *testing.T, kIdentifer *KeyIdentifier, specFilePath string) {
	// load the json file
	data, err := ioutil.ReadFile(specFilePath)
	require.NoError(t, err, "reading spec file")
	var expected spec
	err = json.Unmarshal(data, &expected)
	require.NoError(t, err, "parsing json spec file: %s", specFilePath)
	keyPath := "testdata/specs/" + expected.KeyPath

	actual, err := kIdentifer.IdentifyFile(keyPath)
	require.NoError(t, err, "path to unparseable key: %s", keyPath)

	// Key type. It's not wholly clear how we want to identify
	// these. Right now, we do it this way. But it might change.
	switch expected.Type {
	case "rsa":
		expected.Type = "ssh-rsa"
	case "dsa":
		expected.Type = "ssh-dss"
	case "ed25519":
		expected.Type = "ssh-ed25519"
		// ed25519 is always the new format
		if actual.Format == "openssh" || actual.Format == "openssh-new" {
			expected.Format = "openssh-new"
			actual.Format = "openssh-new"
		}
	default:
	}

	// The elliptic keys don't always have a clear file format, so don't
	// compare that in this test.
	if expected.Type == "ecdsa" && actual.Format == "" {
		expected.Format = ""
	}

	// The elliptic types carry more detail than we need. So whomp down
	// how we test. eg `ecdsa-sha2-nistp256` becomes `ecdsa` for testing
	if strings.HasPrefix(actual.Type, "ecdsa-") {
		expected.Type = "ecdsa"
		actual.Type = "ecdsa"
	}

	// Test correct 'bits' reporting
	// there are several key types/formats that we don't retrieve 'bits' from yet
	switch {
	case (actual.Format == "openssh" && *actual.Encrypted):
	case (expected.Type == "ecdsa" && *actual.Encrypted):
	case (actual.Format == "openssh-new"):
	case (actual.Format == "putty"):
	case (actual.Format == "sshcom"):
	default:
		require.Equal(t, expected.Bits, actual.Bits, "unexpected 'Bits' value, path: %s", expected.KeyPath)
	}

	// test correct fingerprint reporting. limited support for now
	if actual.Format == "openssh-new" {
		if expected.Source != "putty" {
			require.Equal(t, expected.ExpectedFingerprintSHA256, actual.FingerprintSHA256,
				"unexpected sha256 fingerprint, path: %s", expected.KeyPath)
		}
		require.Equal(t, expected.ExpectedFingerprintMD5, actual.FingerprintMD5,
			"unexpected md5 fingerprint, path: %s", expected.KeyPath)
	}

	require.Equal(t, expected.Format, actual.Format, "unexpected key format, path: %s", expected.KeyPath)
	require.Equal(t, expected.Type, actual.Type, "unexpected key type, path: %s", expected.KeyPath)
	require.Equal(t, expected.Encrypted, *actual.Encrypted, "unexpected encrypted boolean, path: %s", expected.KeyPath)
}
