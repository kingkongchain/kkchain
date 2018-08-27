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

package klogging

import (
	"os"
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// A logger to log logging logs!
var loggingLogger = logging.MustGetLogger("logging")

// The default logging level, in force until LoggingInit() is called or in
// case of configuration errors.
var loggingDefaultLevel = logging.INFO

// LoggingInit is a 'hook' called at the beginning of command processing to
// parse logging-related options specified either on the command-line or in
// config files.  Command-line options take precedence over config file
// options, and can also be passed as suitably-named environment variables. To
// change module logging levels at runtime call `logging.SetLevel(level,
// module)`.  To debug this routine include logging=debug as the first
// term of the logging specification.
func LoggingInit(command string) {
	// Parse the logging specification in the form
	//     [<module>[,<module>...]=]<level>[:[<module>[,<module>...]=]<level>...]
	defaultLevel := loggingDefaultLevel
	var err error
	spec := viper.GetString("logging_level")
	if spec == "" {
		spec = viper.GetString("logging." + command)
	}
	if spec != "" {
		fields := strings.Split(spec, ":")
		for _, field := range fields {
			split := strings.Split(field, "=")
			switch len(split) {
			case 1:
				// Default level
				defaultLevel, err = logging.LogLevel(field)
				if err != nil {
					loggingLogger.Warningf("Logging level '%s' not recognized, defaulting to %s : %s", field, loggingDefaultLevel, err)
					defaultLevel = loggingDefaultLevel // NB - 'defaultLevel' was overwritten
				}
			case 2:
				// <module>[,<module>...]=<level>
				if level, err := logging.LogLevel(split[1]); err != nil {
					loggingLogger.Warningf("Invalid logging level in '%s' ignored", field)
				} else if split[0] == "" {
					loggingLogger.Warningf("Invalid logging override specification '%s' ignored - no module specified", field)
				} else {
					modules := strings.Split(split[0], ",")
					for _, module := range modules {
						logging.SetLevel(level, module)
						loggingLogger.Debugf("Setting logging level for module '%s' to %s", module, level)
					}
				}
			default:
				loggingLogger.Warningf("Invalid logging override '%s' ignored; Missing ':' ?", field)
			}
		}
	}
	// Set the default logging level for all modules
	logging.SetLevel(defaultLevel, "")
	loggingLogger.Debugf("Setting default logging level to %s for command '%s'", defaultLevel, command)
}

// DefaultLoggingLevel returns the fallback value for loggers to use if parsing fails
func DefaultLoggingLevel() logging.Level {
	return loggingDefaultLevel
}

// Initiate 'leveled' logging to stderr.
func Init(level string, logFormat string) {
	var err error
	loggingDefaultLevel, err = logging.LogLevel(level)
	if err != nil {
		loggingLogger.Warningf("Logging level '%s' not recognized, defaulting to %s : %s", level, loggingDefaultLevel, err)
	}

	format := logging.MustStringFormatter(logFormat)
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter).SetLevel(loggingDefaultLevel, "")
}
