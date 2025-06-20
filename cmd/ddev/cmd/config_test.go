package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ddev/ddev/pkg/ddevapp"
	"github.com/ddev/ddev/pkg/exec"
	"github.com/ddev/ddev/pkg/fileutil"
	"github.com/ddev/ddev/pkg/globalconfig"
	"github.com/ddev/ddev/pkg/nodeps"
	"github.com/ddev/ddev/pkg/testcommon"
	copy2 "github.com/otiai10/copy"
	asrt "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// TestCmdConfigHooks tests that pre-config and post-config hooks run
func TestCmdConfigHooks(t *testing.T) {
	// Change to the first DevTestSite for the duration of this test.
	site := TestSites[0]
	defer site.Chdir()()
	assert := asrt.New(t)

	app, err := ddevapp.NewApp(site.Dir, true)
	assert.NoError(err)
	app.Hooks = map[string][]ddevapp.YAMLTask{"post-config": {{"exec-host": "touch hello-post-config-" + app.Name}}, "pre-config": {{"exec-host": "touch hello-pre-config-" + app.Name}}}
	err = app.WriteConfig()
	assert.NoError(err)
	// Make sure we get rid of this for other uses
	defer func() {
		app.Hooks = nil
		_ = app.WriteConfig()
	}()

	_, err = exec.RunCommand(DdevBin, []string{"config", "--project-type=" + app.Type})
	assert.NoError(err)

	assert.FileExists("hello-pre-config-" + app.Name)
	assert.FileExists("hello-post-config-" + app.Name)
	err = os.Remove("hello-pre-config-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-config-" + app.Name)
	assert.NoError(err)
}

// TestConfigDescribeLocation tries out the --show-config-location flag.
func TestConfigDescribeLocation(t *testing.T) {
	assert := asrt.New(t)

	origDir, _ := os.Getwd()
	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	err := os.Chdir(tmpDir)
	assert.NoError(err)

	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		out, err := exec.RunHostCommand(DdevBin, "delete", "-Oy", t.Name())
		assert.NoError(err, "output=%s", out)
		_ = os.RemoveAll(tmpDir)
	})
	_, err = exec.RunHostCommand(DdevBin, "config", "--docroot=.", "--project-name="+t.Name())
	assert.NoError(err)

	// Now see if we can detect it
	out, err := exec.RunHostCommand(DdevBin, "config", "--show-config-location")
	assert.NoError(err)
	assert.Contains(string(out), tmpDir)

	// Now try it in a directory that doesn't have a config
	tmpDir = testcommon.CreateTmpDir(t.Name())
	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		_ = os.RemoveAll(tmpDir)
	})
	err = os.Chdir(tmpDir)
	assert.NoError(err)

	out, err = exec.RunHostCommand(DdevBin, "config", "--show-config-location")
	assert.Error(err)
	assert.Contains(string(out), "No project configuration currently exists")
}

// TestConfigWithSitenameFlagDetectsDocroot tests that the docroot is detected when
// flags like --project-name are passed.
func TestConfigWithSitenameFlagDetectsDocroot(t *testing.T) {
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	testDocrootName := "web"
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()
	// Create a document root folder.

	expectedFile := filepath.Join(tmpdir, testDocrootName, "misc/ahah.js")
	err := os.MkdirAll(filepath.Dir(expectedFile), 0777)
	assert.NoError(err)

	// Create index.php that defines docroot
	_, err = os.OpenFile(filepath.Join(tmpdir, testDocrootName, "index.php"), os.O_RDONLY|os.O_CREATE, 0666)
	assert.NoError(err)

	// Create the misc/ahah.js that signals drupal6
	_, err = os.OpenFile(expectedFile, os.O_RDONLY|os.O_CREATE, 0666)
	assert.NoError(err)

	// Create a config
	args := []string{"config", "--project-name", t.Name(), "--php-version=7.2"}
	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	t.Cleanup(func() {
		_, _ = exec.RunCommand(DdevBin, []string{"delete", "-Oy", t.Name()})
	})
	assert.Contains(out, fmt.Sprintf("Configuring a '%s' project named '%s' with docroot '", nodeps.AppTypeDrupal6, t.Name()))
}

// TestConfigSetValues sets all available configuration values using command flags, then confirms that the
// values have been correctly written to the config file.
func TestConfigSetValues(t *testing.T) {
	assert := asrt.New(t)

	projectName := strings.ToLower(t.Name())
	origDir, _ := os.Getwd()
	_, _ = exec.RunHostCommand(DdevBin, "stop", "--unlist", projectName)

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(tmpDir)

	var err error

	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		out, err := exec.RunHostCommand(DdevBin, "delete", "-Oy", projectName)
		assert.NoError(err, "output=%s", out)
		_ = os.RemoveAll(tmpDir)
	})

	_ = os.Chdir(tmpDir)

	// Build config args
	docroot := "web"
	projectType := nodeps.AppTypePHP
	phpVersion := nodeps.PHP81
	routerHTTPPort := "81"
	routerHTTPSPort := "444"
	hostDBPort := "60001"
	hostWebserverPort := "60002"
	hostHTTPSPort := "60003"
	xdebugEnabled := true
	noProjectMount := true
	composerRoot := "composer-root"
	composerVersion := "2.0.0-RC2"
	additionalHostnamesSlice := []string{"abc", "123", "xyz"}
	additionalHostnames := strings.Join(additionalHostnamesSlice, ",")
	additionalFQDNsSlice := []string{"abc.com", "123.pizza", "xyz.co.uk"}
	additionalFQDNs := strings.Join(additionalFQDNsSlice, ",")
	omitContainersSlice := []string{"ddev-ssh-agent"}
	omitContainers := strings.Join(omitContainersSlice, ",")
	webimageExtraPackagesSlice := []string{"php-bcmath", "php7.3-tidy"}
	webimageExtraPackages := strings.Join(webimageExtraPackagesSlice, ",")
	dbimageExtraPackagesSlice := []string{"netcat", "ncdu"}
	dbimageExtraPackages := strings.Join(dbimageExtraPackagesSlice, ",")

	uploadDirsSlice := []string{"custom", "config", "path"}
	webserverType := nodeps.WebserverApacheFPM
	webImage := "custom-web-image"
	webWorkingDir := "/custom/web/dir"
	dbWorkingDir := "/custom/db/dir"
	mailpitHTTPPort := "5001"
	projectTLD := "nowhere.example.com"
	useDNSWhenPossible := false
	timezone := "America/Chicago"
	webEnv := "SOMEENV=some+val"
	nodejsVersion := "16"
	defaultContainerTimeout := 300

	args := []string{
		"config",
		"--project-name", projectName,
		"--docroot", docroot,
		"--project-type", projectType,
		"--php-version", phpVersion,
		"--composer-root", composerRoot,
		"--composer-version", composerVersion,
		"--router-http-port", routerHTTPPort,
		"--router-https-port", routerHTTPSPort,
		fmt.Sprintf("--xdebug-enabled=%t", xdebugEnabled),
		fmt.Sprintf("--no-project-mount=%t", noProjectMount),
		"--additional-hostnames", additionalHostnames,
		"--additional-fqdns", additionalFQDNs,
		"--upload-dirs=" + strings.Join(uploadDirsSlice, ","),
		"--webserver-type", webserverType,
		"--web-image", webImage,
		"--web-working-dir", webWorkingDir,
		"--db-working-dir", dbWorkingDir,
		"--omit-containers", omitContainers,
		"--host-db-port", hostDBPort,
		"--host-webserver-port", hostWebserverPort,
		"--host-https-port", hostHTTPSPort,
		"--webimage-extra-packages", webimageExtraPackages,
		"--dbimage-extra-packages", dbimageExtraPackages,
		"--mailpit-http-port", mailpitHTTPPort,
		"--project-tld", projectTLD,
		"--web-environment", webEnv,
		"--nodejs-version", nodejsVersion,
		"--default-container-timeout", strconv.FormatInt(int64(defaultContainerTimeout), 10),
		fmt.Sprintf("--use-dns-when-possible=%t", useDNSWhenPossible),
		"--timezone", timezone,
		"--disable-upload-dirs-warning",
	}

	out, err := exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err, "error running ddev %v: %v, output=%s", args, err, out)

	// The second run of the config should not change the unspecified options,
	// using the auto option here should not change the config at all
	out, err = exec.RunHostCommand(DdevBin, "config", "--auto")
	require.NoError(t, err, "error running ddev config --auto: '%s'", out)

	configFile := filepath.Join(tmpDir, ".ddev", "config.yaml")
	configContents, err := os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read '%s'", configFile)

	app := &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal '%s'", configFile)

	assert.Equal(projectName, app.Name)
	assert.Equal(docroot, app.Docroot)
	assert.Equal(projectType, app.Type)
	assert.Equal(phpVersion, app.PHPVersion)
	assert.Equal(composerRoot, app.ComposerRoot)
	assert.Equal(composerVersion, app.ComposerVersion)
	assert.Equal(routerHTTPPort, app.RouterHTTPPort)
	assert.Equal(routerHTTPSPort, app.RouterHTTPSPort)
	assert.Equal(hostWebserverPort, app.HostWebserverPort)
	assert.Equal(hostDBPort, app.HostDBPort)
	assert.Equal(xdebugEnabled, app.XdebugEnabled)
	assert.Equal(noProjectMount, app.NoProjectMount)
	assert.Equal(additionalHostnamesSlice, app.AdditionalHostnames)
	assert.Equal(additionalFQDNsSlice, app.AdditionalFQDNs)
	assert.Equal(uploadDirsSlice, app.GetUploadDirs())
	assert.Equal(webserverType, app.WebserverType)
	assert.Equal(webImage, app.WebImage)
	assert.Equal(webWorkingDir, app.WorkingDir["web"])
	assert.Equal(dbWorkingDir, app.WorkingDir["db"])
	assert.Equal(webimageExtraPackagesSlice, app.WebImageExtraPackages)
	assert.Equal(dbimageExtraPackagesSlice, app.DBImageExtraPackages)
	assert.Equal(mailpitHTTPPort, app.GetMailpitHTTPPort())
	assert.Equal(useDNSWhenPossible, app.UseDNSWhenPossible)
	assert.Equal(projectTLD, app.ProjectTLD)
	assert.Equal(timezone, app.Timezone)
	require.NotEmpty(t, app.WebEnvironment)
	assert.Equal(webEnv, app.WebEnvironment[0])
	assert.Equal(nodejsVersion, app.NodeJSVersion)
	assert.Equal(strconv.Itoa(defaultContainerTimeout), app.DefaultContainerTimeout)
	assert.Equal(true, app.DisableUploadDirsWarning)

	// Test that container images, working dirs and Composer root dir can be unset with default flags
	args = []string{
		"config",
		"--composer-root-default",
		"--web-image-default",
		"--web-working-dir-default",
		"--db-working-dir-default",
		`--omit-containers=""`,
		`--additional-hostnames=""`,
		`--additional-fqdns=""`,
		`--webimage-extra-packages=""`,
		`--dbimage-extra-packages=""`,
		`--upload-dirs=""`,
		`--web-environment=""`,
	}

	out, err = exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err, "args=%v, output=%s", args, out)

	configContents, err = os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(app.ComposerRoot, "")
	assert.Equal(app.WebImage, "")
	assert.Equal(len(app.WorkingDir), 0)
	assert.Empty(app.AdditionalHostnames)
	assert.Empty(app.AdditionalFQDNs)
	assert.Empty(app.DBImageExtraPackages)
	assert.Empty(app.OmitContainers)
	assert.Empty(app.UploadDirs)
	assert.Empty(app.WebEnvironment)
	assert.Empty(app.WebImageExtraPackages)

	// Test that all container images and working dirs can each be unset with single default images flag
	args = []string{
		"config",
		"--web-image", webImage,
		"--web-working-dir", webWorkingDir,
		"--db-working-dir", dbWorkingDir,
	}

	_, err = exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err)

	args = []string{
		"config",
		"--image-defaults",
		"--working-dir-defaults",
	}

	_, err = exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err)

	configContents, err = os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(app.WebImage, "")
	assert.Equal(len(app.WorkingDir), 0)

	// Test that variables can be appended to the web environment
	args = []string{
		"config",
		"--web-environment-add", webEnv,
	}

	_, err = exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err)

	configContents, err = os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(1, len(app.WebEnvironment))
	assert.Equal([]string{webEnv}, app.WebEnvironment)

	args = []string{
		"config",
		"--web-environment-add", "SPACES=with spaces,FOO=bar,BAR=baz",
	}

	_, err = exec.RunHostCommand(DdevBin, args...)
	require.NoError(t, err)

	configContents, err = os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(4, len(app.WebEnvironment))
	assert.Equal("BAR=baz", app.WebEnvironment[0])
	assert.Equal("FOO=bar", app.WebEnvironment[1])
	assert.Equal("SPACES=with spaces", app.WebEnvironment[3])
	assert.Equal(webEnv, app.WebEnvironment[2])
}

// TestConfigCreateDocroot sets the docroot, then confirms that the
// value have been correctly written to the config file and docroot is created.
func TestConfigCreateDocroot(t *testing.T) {
	assert := asrt.New(t)

	projectName := strings.ToLower(t.Name())
	origDir, _ := os.Getwd()
	_, _ = exec.RunHostCommand(DdevBin, "stop", "--unlist", projectName)

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(tmpDir)

	var err error

	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		out, err := exec.RunHostCommand(DdevBin, "delete", "-Oy", projectName)
		assert.NoError(err, "output=%s", out)
		_ = os.RemoveAll(tmpDir)
	})

	_ = os.Chdir(tmpDir)

	configFile := filepath.Join(tmpDir, ".ddev", "config.yaml")
	require.NoError(t, err, "Unable to read '%s'", configFile)

	// test docroot locations
	testMatrix := []struct {
		description string
		input       string
		expected    string
		error       string
	}{
		{"empty docroot", "", "", ""},
		{"dot docroot", ".", ".", ""},
		{"fail for outside approot", "../somewhere-else", "", "must remain inside the project"},
		{"fail for absolute path", "//test", "", "cannot be an absolute path"},
		{"dot with slash docroot", "./", "./", ""},
		{"dot with slash and dir docroot", "./test", "./test", ""},
		{"subdir docroot", "test/dir", "test/dir", ""},
		{"dir docroot", "test", "test", ""},
		{"dot with slash and subdir docroot", "./test/dir", "./test/dir", ""},
	}

	for _, tc := range testMatrix {
		t.Run(tc.description, func(t *testing.T) {
			args := []string{
				"config",
				"--docroot", tc.input,
			}

			out, err := exec.RunHostCommand(DdevBin, args...)
			if tc.error != "" {
				require.Error(t, err)
				require.Contains(t, out, tc.error)
				return
			}
			require.NoError(t, err)

			configContents, err := os.ReadFile(configFile)
			require.NoError(t, err, "Unable to read %s: %v", configFile, err)

			app := &ddevapp.DdevApp{}
			err = yaml.Unmarshal(configContents, app)
			require.NoError(t, err, "Could not unmarshal %s: %v", configFile, err)

			require.Equal(t, tc.expected, app.Docroot)

			// Confirm that the docroot is created
			require.DirExists(t, filepath.Join(tmpDir, tc.expected))
		})
	}
}

// TestConfigInvalidProjectname tests to make sure that invalid projectnames
// are not accepted and valid names are accepted.
func TestConfigInvalidProjectname(t *testing.T) {
	var err error
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create an existing docroot
	docroot := "web"
	if err = os.MkdirAll(filepath.Join(tmpdir, docroot), 0755); err != nil {
		t.Errorf("Could not create docroot %s in %s", docroot, tmpdir)
	}

	// Test some valid project names
	for _, projName := range []string{"no-spaces-but-hyphens", "UpperAndLower", "should.work.with.dots"} {
		args := []string{
			"config",
			"--project-name", projName,
		}

		out, err := exec.RunCommand(DdevBin, args)
		assert.NoError(err)
		assert.NotContains(out, "is not a valid project name")
		assert.Contains(out, "You may now run 'ddev start'")
		args = []string{
			"stop",
			"--unlist", projName,
		}
		_, _ = exec.RunCommand(DdevBin, args)

		_ = os.Remove(filepath.Join(tmpdir, ".ddev", "config.yaml"))
	}

	// Test some invalid project names.
	for _, projName := range []string{"with spaces", "with_underscores", "no,commas-will-make-it"} {
		args := []string{
			"config",
			"--project-name", projName,
		}

		out, err := exec.RunCommand(DdevBin, args)
		assert.Error(err)
		assert.Contains(out, fmt.Sprintf("%s is not a valid project name", projName))
		assert.NotContains(out, "You may now run 'ddev start'")
		_ = os.Remove(filepath.Join(tmpdir, ".ddev", "config.yaml"))
	}
}

// TestCmdConfigHasAllowedLocation tests to ensure that 'ddev config'
// can't be accidentally run in the home directory, while also verifying that
// 'ddev config' can run in a subdirectory, except when configuring a new project.
func TestCmdConfigHasAllowedLocation(t *testing.T) {
	var err error
	assert := asrt.New(t)

	origDir, _ := os.Getwd()
	// Make sure we're not allowed to config in home directory.
	home, _ := os.UserHomeDir()
	err = os.Chdir(home)
	require.NoError(t, err)
	out, err := exec.RunHostCommand(DdevBin, "config", "--project-type=php")
	require.Error(t, err)
	require.Contains(t, out, "not allowed in")

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	// Create a project
	_, err = exec.RunHostCommand(DdevBin, "config", "--project-type=php", "--project-name="+t.Name())
	require.NoError(t, err)

	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		_, err = exec.RunHostCommand(DdevBin, "delete", "-Oy", t.Name())
		assert.NoError(err)
		_, err = exec.RunHostCommand(DdevBin, "delete", "-Oy", t.Name()+"-subdir")
		assert.NoError(err)
		_ = os.RemoveAll(tmpDir)
	})

	subdirName := t.Name() + fileutil.RandomFilenameBase()
	subdir := filepath.Join(tmpDir, subdirName)
	err = os.Mkdir(subdir, 0777)
	require.NoError(t, err)
	err = os.Chdir(subdir)
	require.NoError(t, err)

	// Make sure that 'ddev config' in a subdir doesn't give an error for existing project
	_, err = exec.RunHostCommand(DdevBin, "config", "--web-environment-add=FOO=BAR")
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(subdir, ".ddev/config.yaml"))
	require.FileExists(t, filepath.Join(tmpDir, ".ddev/config.yaml"))
	configFileContents, err := os.ReadFile(filepath.Join(tmpDir, ".ddev/config.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(configFileContents), "FOO=BAR")

	// Make sure that 'ddev config' in a subdir gives an error for new projects
	out, err = exec.RunHostCommand(DdevBin, "config", "--project-type=php", "--project-name="+t.Name()+"-subdir")
	require.Error(t, err)
	require.Contains(t, out, fmt.Sprintf("project root '%s' already contains a project", tmpDir))
	require.Contains(t, out, "You may want to remove the existing project")
	require.NoFileExists(t, filepath.Join(subdir, ".ddev/config.yaml"))

	// Create a dummy config file to make it look like it's an existing project
	err = os.MkdirAll(filepath.Join(subdir, ".ddev"), 0777)
	require.NoError(t, err)
	_, err = os.Create(filepath.Join(subdir, ".ddev/config.yaml"))
	require.NoError(t, err)

	// Make sure that 'ddev config' in the subdir works fine for existing projects
	out, err = exec.RunHostCommand(DdevBin, "config", "--project-type=php", "--project-name="+t.Name()+"-subdir")
	require.NoError(t, err)
	require.Contains(t, out, "'"+t.Name()+"-subdir'")
	require.Contains(t, out, "Configuration complete")

	// Make sure that 'ddev config' in the parent directory works fine for existing projects
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	out, err = exec.RunHostCommand(DdevBin, "config", "--auto")
	require.NoError(t, err)
	require.Contains(t, out, "'"+t.Name()+"'")
	require.Contains(t, out, "Configuration complete")
}

// TestConfigDatabaseVersion checks to make sure that both
// ddev config --database behaves correctly,
func TestConfigDatabaseVersion(t *testing.T) {
	assert := asrt.New(t)

	origDir, _ := os.Getwd()
	versionsToTest := nodeps.GetValidDatabaseVersions()
	if os.Getenv("GOTEST_SHORT") != "" {
		versionsToTest = []string{"mariadb:10.11", "mysql:8.0", "postgres:17"}
	}

	// Create a temporary directory and switch to it.
	testDir := testcommon.CreateTmpDir(t.Name())
	err := os.Chdir(testDir)
	require.NoError(t, err)

	err = globalconfig.RemoveProjectInfo(t.Name())
	assert.NoError(err)

	out, err := exec.RunHostCommand(DdevBin, "config", "--project-name", t.Name())
	assert.NoError(err, "Failed running ddev config --project-name: %s", out)

	err = globalconfig.ReadGlobalConfig()
	require.NoError(t, err)

	app, err := ddevapp.GetActiveApp("")
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.Chdir(origDir)
		assert.NoError(err)
		_ = os.RemoveAll(testDir)
	})

	_, err = app.ReadConfig(false)
	assert.NoError(err)
	assert.Equal(nodeps.MariaDB, app.Database.Type)
	assert.Equal(nodeps.MariaDBDefaultVersion, app.Database.Version)

	// Verify behavior with no existing config.yaml. It should
	// add a database into the config and nothing else
	for _, dbTypeVersion := range versionsToTest {
		_ = app.Stop(true, false)
		parts := strings.Split(dbTypeVersion, ":")
		err = os.RemoveAll(filepath.Join(testDir, ".ddev"))
		assert.NoError(err)
		out, err := exec.RunHostCommand(DdevBin, "config", "--database="+dbTypeVersion, "--project-name="+t.Name())
		require.NoError(t, err, "Failed to run ddev config --database %s: %s", dbTypeVersion, out)
		assert.Contains(out, "You may now run 'ddev start'")

		// First test the bare explicit values found in the config.yaml,
		// without the NewApp adjustments
		app := &ddevapp.DdevApp{}
		assert.NoError(err)
		err = app.LoadConfigYamlFile(filepath.Join(testDir, ".ddev", "config.yaml"))
		assert.NoError(err)
		assert.Equal(parts[0], app.Database.Type)
		assert.Equal(parts[1], app.Database.Version)

		// Now use NewApp() to load, so that we get the full logic of that function.
		app, err = ddevapp.NewApp(testDir, false)
		assert.NoError(err)
		t.Cleanup(func() {
			err = app.Stop(true, false)
			assert.NoError(err)
		})
		_, err = app.ReadConfig(false)
		assert.NoError(err)
		assert.Equal(parts[0], app.Database.Type)
		assert.Equal(parts[1], app.Database.Version)
		err = app.Stop(true, false)
		assert.NoError(err)
	}
}

// TestConfigUpdate verifies that ddev config --update does the right things updating default
// config, and does not do the wrong things.
func TestConfigUpdate(t *testing.T) {
	var err error
	origDir, _ := os.Getwd()

	// Create a temporary directory and switch to it.
	testDir := testcommon.CreateTmpDir(t.Name())

	t.Cleanup(func() {
		app, _ := ddevapp.NewApp(testDir, false)
		_ = app.Stop(true, false)
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(testDir)
	})
	tests := map[string]struct {
		input             string
		baseExpectation   ddevapp.DdevApp
		configExpectation ddevapp.DdevApp
	}{
		"drupal11-composer": {
			baseExpectation:   ddevapp.DdevApp{Type: nodeps.AppTypePHP, PHPVersion: nodeps.PHPDefault, Docroot: "", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
			configExpectation: ddevapp.DdevApp{Type: nodeps.AppTypeDrupal11, PHPVersion: nodeps.PHPDefault, Docroot: "web", CorepackEnable: true, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
		},
		"drupal11-git": {
			baseExpectation:   ddevapp.DdevApp{Type: nodeps.AppTypePHP, PHPVersion: nodeps.PHPDefault, Docroot: "", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
			configExpectation: ddevapp.DdevApp{Type: nodeps.AppTypeDrupal11, PHPVersion: nodeps.PHPDefault, Docroot: "", CorepackEnable: true, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
		},
		"drupal10-composer": {
			baseExpectation:   ddevapp.DdevApp{Type: nodeps.AppTypePHP, PHPVersion: nodeps.PHPDefault, Docroot: "", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
			configExpectation: ddevapp.DdevApp{Type: nodeps.AppTypeDrupal10, PHPVersion: nodeps.PHPDefault, Docroot: "web", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
		},
		"craftcms": {
			baseExpectation:   ddevapp.DdevApp{Type: nodeps.AppTypePHP, PHPVersion: nodeps.PHPDefault, Docroot: "", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MariaDB, Version: nodeps.MariaDBDefaultVersion}},
			configExpectation: ddevapp.DdevApp{Type: nodeps.AppTypeCraftCms, PHPVersion: nodeps.PHPDefault, Docroot: "web", CorepackEnable: false, Database: ddevapp.DatabaseDesc{Type: nodeps.MySQL, Version: "8.0"}},
		},
	}

	for testName, expectation := range tests {
		t.Run(testName, func(t *testing.T) {
			// Delete existing
			_ = globalconfig.RemoveProjectInfo(t.Name())
			// Delete filesystem from existing
			_ = os.RemoveAll(testDir)

			err = os.MkdirAll(testDir, 0755)
			require.NoError(t, err)
			_ = os.Chdir(testDir)
			require.NoError(t, err)

			// Copy testdata in from source
			testSource := filepath.Join(origDir, "testdata", t.Name())
			err = copy2.Copy(testSource, testDir)
			require.NoError(t, err)

			// Start with an existing config.yaml and verify
			app, err := ddevapp.NewApp("", false)
			require.NoError(t, err)
			_ = app.Stop(true, false)

			// Original values should match
			checkValues(t, testName, expectation.baseExpectation, app)

			// ddev config --update and verify
			out, err := exec.RunHostCommand(DdevBin, "config", "--update")
			require.NoError(t, err, "failed to run ddev config --update: %v output=%s", err, out)

			// Load the newly-created app to inspect it
			app, err = ddevapp.NewApp("", false)
			require.NoError(t, err)

			// Updated values should match
			checkValues(t, testName, expectation.configExpectation, app)
		})
	}
}

// checkValues compares several values of the expected and actual apps to make sure they're the same
func checkValues(t *testing.T, name string, expectation ddevapp.DdevApp, app *ddevapp.DdevApp) {
	assert := asrt.New(t)

	reflectedExpectation := reflect.ValueOf(expectation)
	reflectedApp := reflect.ValueOf(*app)

	for _, member := range []string{"Type", "PHPVersion", "Docroot", "CorepackEnable", "Database"} {
		fieldExpectation := reflectedExpectation.FieldByName(member)
		if fieldExpectation.IsValid() {
			fieldValueExpectation := fieldExpectation.Interface()
			fieldValueApp := reflectedApp.FieldByName(member).Interface()
			assert.Equal(fieldValueExpectation, fieldValueApp, "%s: field %s does not match", name, member)
		}
	}
}

// TestConfigGitignore checks that our gitignore is ignoring the right things.
func TestConfigGitignore(t *testing.T) {
	assert := asrt.New(t)

	origDir, _ := os.Getwd()

	tmpXdgConfigHomeDir := testcommon.CopyGlobalDdevDir(t)
	globalDdevDir := globalconfig.GetGlobalDdevDir()

	// Create a temporary directory and switch to it.
	testDir := testcommon.CreateTmpDir(t.Name())

	err := os.Chdir(testDir)
	require.NoError(t, err)

	_, err = exec.RunHostCommand(DdevBin, "config", "--auto")
	assert.NoError(err)
	t.Cleanup(func() {
		_, err = exec.RunHostCommand(DdevBin, "delete", "-Oy")
		assert.NoError(err)
		err = os.Chdir(origDir)
		assert.NoError(err)
		testcommon.ResetGlobalDdevDir(t, tmpXdgConfigHomeDir)
		_ = os.RemoveAll(testDir)
	})

	_, err = exec.RunHostCommand("git", "init")
	assert.NoError(err)
	_, err = exec.RunHostCommand("git", "add", ".")
	assert.NoError(err)
	out, err := exec.RunHostCommand("git", "status")
	assert.NoError(err)

	// git status should have one new file, config.yaml
	assert.Contains(out, "new file:   .ddev/config.yaml")
	// .ddev/config.yaml should be the only new file, remove it and check
	out = strings.ReplaceAll(out, "new file:   .ddev/config.yaml", "")
	assert.NotContains(out, "new file:")

	_, err = exec.RunHostCommand("bash", "-c", fmt.Sprintf(`touch "%s" "%s"`, filepath.Join(globalDdevDir, "commands", "web", t.Name()), filepath.Join(globalDdevDir, "homeadditions", t.Name())))
	assert.NoError(err)
	if err != nil {
		out, err = exec.RunHostCommand("bash", "-c", fmt.Sprintf(`ls -l "%s" && ls -lR "%s" "%s"`, globalDdevDir, filepath.Join(globalDdevDir, "commands"), filepath.Join(globalDdevDir, "homeadditions")))
		assert.NoError(err)
		t.Logf("Contents of global .ddev: \n=====\n%s\n====", out)
	}

	_, err = exec.RunHostCommand(DdevBin, "start", "-y")
	assert.NoError(err)
	statusOut, err := exec.RunHostCommand("bash", "-c", "git status")
	assert.NoError(err)
	_, err = exec.RunHostCommand("bash", "-c", "git status | grep 'Untracked files'")
	assert.Error(err, "Untracked files were found where we didn't expect them: %s", statusOut)
}

// TestDrupalAppTypeUsage validates that `drupal` project type gets used properly
// * It should be accepted, but turned into latest stable drupal version when `ddev config`
// * "drupal" in config.yaml type should be interpreted as latets stable drupal by ddev list and describe
// * `ddev config --auto` should respect `drupal` as project type but convert it to latest stable
func TestDrupalAppTypeUsage(t *testing.T) {
	origDir, _ := os.Getwd()

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(tmpDir)

	out, err := exec.RunCommand(DdevBin, []string{"delete", "-Oy", t.Name()})
	require.NoError(t, err, "ddev delete -Oy failed: %v", out)

	t.Cleanup(func() {
		_ = os.Chdir(origDir)
		out, _ = exec.RunCommand(DdevBin, []string{"delete", "-Oy", t.Name()})
		t.Logf("ddev delete -Oy %s output=%s", t.Name(), out)
		_ = os.RemoveAll(tmpDir)
	})

	// Create a config
	args := []string{"config", "--project-name=" + t.Name(), "--project-type=drupal"}
	_, err = exec.RunCommand(DdevBin, args)
	require.NoError(t, err)

	app, err := ddevapp.NewApp(tmpDir, true)
	require.NoError(t, err)

	require.Equal(t, nodeps.AppTypeDrupalLatestStable, app.Type)
	err = app.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = app.Stop(true, false)
	})
	desc, err := app.Describe(true)
	require.NoError(t, err)
	require.Equal(t, nodeps.AppTypeDrupalLatestStable, desc["type"])

	err = app.Stop(true, false)
	require.NoError(t, err)

	// Just read the config and verify that the project type is the explicit drupal version
	app, err = ddevapp.NewApp(tmpDir, true)
	require.NoError(t, err)
	require.Equal(t, nodeps.AppTypeDrupalLatestStable, app.Type)

	err = app.Stop(true, false)
	require.NoError(t, err)

	// Now try type = "drupal" in the config.yaml and verify that we actually get latest stable
	// legacy projects from late DDEV v1.23.x will have this.
	app.Type = `drupal`
	err = app.WriteConfig()
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)

	// Even though the config.yaml says "drupal", we'll report latest stable here
	require.Equal(t, nodeps.AppTypeDrupalLatestStable, app.Type)

	// And app.Describe will show it as latest stable, even though type is set to drupal in config.yaml
	desc, err = app.Describe(true)
	require.NoError(t, err)
	require.Equal(t, nodeps.AppTypeDrupalLatestStable, desc["type"])

	// ddev config --auto should respect what is already in config.yaml
	// But it will turn it into latest drupal stable instead of `drupal`
	_, err = exec.RunHostCommand(DdevBin, "config", "--auto")
	require.NoError(t, err)

	_, err = app.ReadConfig(false)
	require.NoError(t, err)
	require.Equal(t, nodeps.AppTypeDrupalLatestStable, app.Type)
}

// TestHasConfigNameOverride tests that `ddev config` has name override from `.ddev/config.*.yaml` files
// and new name is not written to .ddev/config.yaml
func TestHasConfigNameOverride(t *testing.T) {
	assert := asrt.New(t)

	projectName := strings.ToLower(t.Name())
	origDir, _ := os.Getwd()
	_, _ = exec.RunHostCommand(DdevBin, "stop", "--unlist", projectName)

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(tmpDir)

	var err error

	t.Cleanup(func() {
		err = os.Chdir(origDir)
		assert.NoError(err)
		out, err := exec.RunHostCommand(DdevBin, "delete", "-Oy", projectName)
		assert.NoError(err, "output=%s", out)
		_ = os.RemoveAll(tmpDir)
	})

	_ = os.Chdir(tmpDir)

	// Create a dummy config file to make it look like it's an existing project
	// Otherwise DDEV will not read the override
	err = os.MkdirAll(filepath.Join(tmpDir, ".ddev"), 0777)
	require.NoError(t, err)
	configFile := filepath.Join(tmpDir, ".ddev/config.yaml")
	configFileLocal := filepath.Join(tmpDir, ".ddev/config.local.yaml")
	_, err = os.Create(configFile)
	require.NoError(t, err)

	// Create an override for the project name
	projectNameOverride := projectName + "-override"
	err = os.WriteFile(configFileLocal, []byte(`name: `+projectNameOverride), 0644)
	require.NoError(t, err)

	out, err := exec.RunHostCommand(DdevBin, "config", "--auto")
	require.NoError(t, err, "error running ddev config --auto: '%s'", out)

	configContents, err := os.ReadFile(configFile)
	require.NoError(t, err, "Unable to read '%s'", configFile)
	configLocalContents, err := os.ReadFile(configFileLocal)
	require.NoError(t, err, "Unable to read '%s'", configFileLocal)

	app := &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	require.NoError(t, err, "Could not unmarshal '%s'", configFile)

	// app.Name inside .ddev/config.yaml should be empty
	require.Equal(t, "", app.Name)

	err = yaml.Unmarshal(configLocalContents, app)
	require.NoError(t, err, "Could not unmarshal '%s'", configFileLocal)

	// app.Name inside .ddev/config.local.yaml should have the expected value
	require.Equal(t, projectNameOverride, app.Name)
}
