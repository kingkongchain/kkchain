/*
MIT License

Copyright (c) 2018 invin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package klogging_test

import (
	"testing"

	"github.com/invin/kkchain/config"
	"github.com/invin/kkchain/klogging"
	logging "github.com/op/go-logging"
	"github.com/spf13/viper"
)

func TestLog(t *testing.T) {
	config.Init("../config/")
	klogging.Init(config.AppConfig.Log.GetString("log.level"), config.AppConfig.Log.GetString("log.format"))
	var testLogger = logging.MustGetLogger("test")
	testLogger.Debugf("debug %s", "secret")
	testLogger.Info("info")
	testLogger.Notice("notice")
	testLogger.Warning("warning")
	testLogger.Error("err")
	testLogger.Critical("crit")
}

func BenchmarkLog(b *testing.B) {
	config.Init("../config/")
	klogging.Init(config.AppConfig.Log.GetString("log.level"), config.AppConfig.Log.GetString("log.format"))
	var testLogger = logging.MustGetLogger("test")
	testLogger.Debugf("debug %s", "secret")
	testLogger.Info("info")
	testLogger.Notice("notice")
	testLogger.Warning("warning")
	testLogger.Error("err")
	testLogger.Critical("crit")
}

func TestLoggingLevelDefault(t *testing.T) {
	viper.Reset()

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func TestLoggingLevelOtherThanDefault(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "warning")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, logging.WARNING)
}

func TestLoggingLevelForSpecificModule(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "core=info")

	klogging.LoggingInit("")

	assertModuleLoggingLevel(t, "core", logging.INFO)
}

func TestLoggingLeveltForMultipleModules(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "core=warning:test=debug")

	klogging.LoggingInit("")

	assertModuleLoggingLevel(t, "core", logging.WARNING)
	assertModuleLoggingLevel(t, "test", logging.DEBUG)
}

func TestLoggingLevelForMultipleModulesAtSameLevel(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "core,test=warning")

	klogging.LoggingInit("")

	assertModuleLoggingLevel(t, "core", logging.WARNING)
	assertModuleLoggingLevel(t, "test", logging.WARNING)
}

func TestLoggingLevelForModuleWithDefault(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "info:test=warning")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, logging.INFO)
	assertModuleLoggingLevel(t, "test", logging.WARNING)
}

func TestLoggingLevelForModuleWithDefaultAtEnd(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "test=warning:info")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, logging.INFO)
	assertModuleLoggingLevel(t, "test", logging.WARNING)
}

func TestLoggingLevelForUnknownCommandGoesToDefault(t *testing.T) {
	viper.Reset()

	klogging.LoggingInit("unknown command")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func TestLoggingLevelInvalid(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "invalidlevel")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func TestLoggingLevelInvalidModules(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "core=invalid")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func TestLoggingLevelInvalidEmptyModule(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "=warning")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func TestLoggingLevelInvalidModuleSyntax(t *testing.T) {
	viper.Reset()
	viper.Set("logging_level", "type=warn=again")

	klogging.LoggingInit("")

	assertDefaultLoggingLevel(t, klogging.DefaultLoggingLevel())
}

func assertDefaultLoggingLevel(t *testing.T, expectedLevel logging.Level) {
	assertModuleLoggingLevel(t, "", expectedLevel)
}

func assertModuleLoggingLevel(t *testing.T, module string, expectedLevel logging.Level) {
	assertEquals(t, expectedLevel, logging.GetLevel(module))
}

func assertEquals(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		t.Errorf("Expected: %v, Got: %v", expected, actual)
	}
}
