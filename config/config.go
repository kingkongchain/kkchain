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
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Consensus  *viper.Viper
	P2p        *viper.Viper
	Log        *viper.Viper
	configPath string
}

var AppConfig = new(Config)

//中文： 初始化程序所有配置文件
//english：init  all  config files of application
func Init(configPath string) {
	AppConfig.configPath = configPath
	AppConfig.Consensus = LoadConfig("consensus")
	AppConfig.P2p = LoadConfig("p2p")
	AppConfig.Log = LoadConfig("log")
}

//中文： 设置配置文件目录
//english：set config file path
func SetConfigPath(configPath string) {
	AppConfig.configPath = configPath
}

//中文：加载一个配置文件
//english：load one config file
func LoadConfig(configFileName string) *viper.Viper {
	configViper := viper.New()
	configViper.SetConfigName(configFileName) // name of config file (without extension)
	if AppConfig.configPath == "" {
		configViper.AddConfigPath("./config/")
	} else {
		configViper.AddConfigPath(AppConfig.configPath)
	}
	configViper.SetConfigType("toml")
	err := configViper.ReadInConfig() // Find and read the config file
	if err != nil {                   // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	return configViper
}
