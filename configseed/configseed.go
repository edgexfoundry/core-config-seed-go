/*******************************************************************************
 * Copyright 2017 Samsung Electronics All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 *******************************************************************************/
package configseed

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/magiconair/properties"
	"gopkg.in/yaml.v2"
)

// URL to check a Consul agent status.
const CONSUL_STATUS_PATH string = "/v1/agent/self"

// Configuration struct used to parse the JSON configuration file.
type ConfigurationStruct struct {
	ConfigPath                   string
	GlobalPrefix                 string
	ConsulProtocol               string
	ConsulHost                   string
	ConsulPort                   int
	IsReset                      bool
	FailLimit                    int
	FailWaittime                 int
	AcceptablePropertyExtensions []string
	YamlExtensions               []string
}

// Map to cover key/value.
type ConfigProperties map[string]string

// Hook the functions in the other packages for the tests.
var (
	consulDefaultConfig = consulapi.DefaultConfig
	consulNewClient     = consulapi.NewClient
	consulDeleteTree    = (*consulapi.KV).DeleteTree
	consulPut           = (*consulapi.KV).Put
	consulKeys          = (*consulapi.KV).Keys
	httpGet             = http.Get
)

// Configuration data for the config-seed service.
var configuration ConfigurationStruct = ConfigurationStruct{}

// Logger for the config-seed service.
var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

// Run the Config-Seed application.
func RunApplication(configFilePath string, bannerFilePath string) {
	printBanner(bannerFilePath)

	// Load configuration data
	if err := loadConfigurationFile(configFilePath); err != nil {
		logger.Println(err.Error())
		return
	}

	consulClient, err := getConsulCient()
	if err != nil {
		logger.Println(err.Error())
		return
	}

	kv := consulClient.KV()

	if configuration.IsReset {
		removeStoredConfig(kv)
		loadConfigFromPath(kv)
	} else if !isConfigInitialized(kv) {
		loadConfigFromPath(kv)
	}
	// If 'IsReset' is unset and Consul already has been configured, do nothing.
}

// Print a banner.
func printBanner(path string) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Println(string(b))
}

// Load a json file that contains configuration info for Config-Seed applcation
// and keep the info in configuration variable.
func loadConfigurationFile(path string) error {
	// Read the configuration file
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	// Decode the configuration as JSON
	err = json.Unmarshal(contents, &configuration)
	if err != nil {
		return err
	}

	return nil
}

// Get handle of Consul client using the URL from configuration info.
// Before getting handle, it tries to receive a response from a Consul agent by simple health-check.
func getConsulCient() (*consulapi.Client, error) {
	consulUrl := configuration.ConsulProtocol + "://" + configuration.ConsulHost + ":" + strconv.Itoa(configuration.ConsulPort)

	// Check the connection to Consul
	fails := 0
	for fails < configuration.FailLimit {
		resp, err := httpGet(consulUrl + CONSUL_STATUS_PATH)
		if err != nil {
			logger.Println(err.Error())
			time.Sleep(time.Second * time.Duration(configuration.FailWaittime))
			fails++
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
	}
	if fails >= configuration.FailLimit {
		return nil, errors.New("Cannot get connection to Consul")
	}

	// Connect to the Consul Agent
	config := consulDefaultConfig()
	config.Address = consulUrl

	return consulNewClient(config)
}

// Remove all values in Consul K/V store, under the globalprefix which is presents in configuration file.
func removeStoredConfig(kv *consulapi.KV) {
	_, err := consulDeleteTree(kv, configuration.GlobalPrefix, nil)
	if err != nil {
		logger.Println(err.Error())
		return
	}
	logger.Println("All values under the globalPrefix(\"" + configuration.GlobalPrefix + "\") is removed.")
}

// Check if Consul has been configured by trying to get any key that starts with a globalprefix.
func isConfigInitialized(kv *consulapi.KV) bool {
	keys, _, err := consulKeys(kv, configuration.GlobalPrefix, "", nil)
	if err != nil {
		logger.Println(err.Error())
		return false
	}

	if len(keys) > 0 {
		logger.Printf("%s exists! The configuration data has been initialized.\n", configuration.GlobalPrefix)
		return true
	}
	logger.Printf("%s doesn't exist! Start importing configuration data.\n", configuration.GlobalPrefix)
	return false
}

// Load all config files and put the configuration info to Consul K/V store.
func loadConfigFromPath(kv *consulapi.KV) {
	err := filepath.Walk(configuration.ConfigPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories & unacceptable property extension
		if info.IsDir() || !isAcceptablePropertyExtensions(info.Name()) {
			return nil
		}

		dir, file := filepath.Split(path)
		configPath, err := filepath.Rel(".", configuration.ConfigPath)
		if err != nil {
			return err
		}

		dir = strings.TrimPrefix(dir, configPath+"/")
		logger.Println("found config file:", file, "in context", dir)

		props, err := readPropertyFile(path)
		if err != nil {
			return err
		}

		// Put config properties to Consul K/V store.
		prefix := configuration.GlobalPrefix + "/" + dir
		for k := range props {
			p := &consulapi.KVPair{Key: prefix + k, Value: []byte(props[k])}
			if _, err := consulPut(kv, p, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Println(err.Error())
		return
	}
}

// Check whether a filename extension of input file belongs to the list of acceptable property extensions
// which are present in configuration file.
func isAcceptablePropertyExtensions(file string) bool {
	for _, v := range configuration.AcceptablePropertyExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

// Load a property file(.yaml or .properties) and parse it to a map.
func readPropertyFile(filePath string) (ConfigProperties, error) {
	if isYamlExtension(filePath) {
		// Read .yaml/.yml file
		return readYamlFile(filePath)
	} else {
		// Read .properties file
		return readPropertiesFile(filePath)
	}
}

// Check whether a filename extension is yaml or not.
func isYamlExtension(file string) bool {
	for _, v := range configuration.YamlExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

// Parse a yaml file to a map.
func readYamlFile(filePath string) (ConfigProperties, error) {
	configProps := ConfigProperties{}

	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var body map[string]interface{}
	if err = yaml.Unmarshal(contents, &body); err != nil {
		return nil, err
	}

	for key, value := range body {
		configProps[key] = fmt.Sprint(value)
	}

	return configProps, nil
}

// Parse a properties file to a map.
func readPropertiesFile(filePath string) (ConfigProperties, error) {
	configProps := ConfigProperties{}

	props, err := properties.LoadFile(filePath, properties.UTF8)
	if err != nil {
		return nil, err
	}
	configProps = props.Map()

	return configProps, nil
}
