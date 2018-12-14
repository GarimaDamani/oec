package conf

import (
	"testing"
	"os"
	"github.com/stretchr/testify/assert"
)

var gitCloneCalled = false
var testConfFilePath = "maridConf.json"
var mockConfFileContent = `{
	"apiKey": "ApiKey",
    "actionMappings": {
        "Create": {
            "filePath": "/path/to/runbook.bin",
            "source": "local",
            "environmentVariables": [
                "e1=v1", "e2=v2"
            ]
        },
        "Close": {
            "source": "github",
            "repoOwner": "testAccount",
            "repoName": "testRepo",
            "repoFilePath": "marid/testConfig.json",
            "repoToken": "testToken",
            "environmentVariables": [
                "e1=v1", "e2=v2"
            ]
        }
    }
}`

func mockGitClone(tempDir string, url string, privateKeyFilePath string, passPhrase string) error {
	gitCloneCalled = true
	var tmpDir = os.TempDir()
	directoryName, err := parseDirectoryNameFromUrl(url)

	if err != nil {
		return err
	}

	os.MkdirAll(tmpDir + string(os.PathSeparator) + directoryName, 0755)
	testFile, err := os.OpenFile(tmpDir + string(os.PathSeparator) + directoryName + string(os.PathSeparator) +
		testConfFilePath, os.O_CREATE|os.O_WRONLY, 0755)

	if err != nil {
		return err
	}

	testFile.WriteString(mockConfFileContent)
	testFile.Close()

	return nil
}

func TestReadConfigurationFromGit(t *testing.T) {
	repoName := "repo"
	url := "https://github.com/someaccount/" + repoName + ".git"
	privateKeyFilePath := "dummypath"
	passPhrase := "passPhrase"

	oldGitCloneMethod := gitCloneFunc
	defer func() { gitCloneFunc = oldGitCloneMethod}()
	gitCloneFunc = mockGitClone
	config, err := readConfigurationFromGit(url, testConfFilePath, privateKeyFilePath, passPhrase)

	if err != nil {
		t.Error("Could not read from Marid configuration. Error: " + err.Error())
	}

	assert.True(t, gitCloneCalled, "readConfigurationFromGit function did not call the method gitClone.")

	assert.Equal(t, mockConf, config,
		"Actual config and expected config are not the same.")
	var repoDir = os.TempDir() + string(os.PathSeparator) + repoName

	if _, err := os.Stat(repoDir + string(os.PathSeparator) + testConfFilePath); !os.IsNotExist(err) {
		t.Error("Marid configuration file still exists.")
	}

	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("Cloned repository folder still exists.")
	}
}

func TestRemoveLocalRepoEvenIfErrorOccurs(t *testing.T) {
	var repoName = "repo"
	_, err := readConfigurationFromGit("https://github.com/someaccount/"+repoName+".git", testConfFilePath,
		"dummypath", "passPhrase")
	var repoDir = os.TempDir() + string(os.PathSeparator) + repoName

	if err == nil {
		t.Error("Error should be returned.")
	}

	if _, err := os.Stat(repoDir + string(os.PathSeparator) + testConfFilePath); !os.IsNotExist(err) {
		t.Error("Marid configuration file still exists.")
	}

	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("Cloned repository folder still exists.")
	}
}

func TestParseDirectoryNameFromUrl(t *testing.T) {
	repoName := "repo"
	url := "https://github.com/abc/" + repoName + ".git"
	actual, err := parseDirectoryNameFromUrl(url)

	if err != nil {
		t.Error("Error occurred while parsing directory name from URL [" + url + "]. Error: " + err.Error())
	}

	if actual != repoName {
		t.Errorf("Parsed repo name wrong. Expected: %s, Actual: %s", repoName, actual)
	}

	url = "sacma_sapan"
	actual, err = parseDirectoryNameFromUrl(url)
	assert.Error(t, err, "Did not throw an error altough URL was wrong.")

	if err.Error() != url + " is not a valid Git URL." {
		t.Error("Error message was wrong.")
	}
}
