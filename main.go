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
package main

import (
	"github.com/edgexfoundry/core-config-seed-go/pkg"
	"github.com/edgexfoundry/core-config-seed-go/pkg/config"
	"errors"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/fatih/structs"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/magiconair/properties"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var Version = "master"

const CONSUL_STATUS_PATH = "/v1/agent/self"

// Hook the functions in the other packages for the tests.
var (
	consulDefaultConfig = consulapi.DefaultConfig
	consulNewClient     = consulapi.NewClient
	consulDeleteTree    = (*consulapi.KV).DeleteTree
	consulPut           = (*consulapi.KV).Put
	consulKeys          = (*consulapi.KV).Keys
	httpGet             = http.Get
)

// Configuration struct used to parse the JSON configuration file.
type CoreConfig struct {
	ConfigPath                   string
	GlobalPrefix                 string
	ConsulProtocol               string
	ConsulHost                   string
	ConsulPort                   int
	IsReset                      bool
	FailLimit                    int
	FailWaitTime                 int
	AcceptablePropertyExtensions []string
	YamlExtensions               []string
	TomlExtensions               []string
}

// Map to cover key/value.
type ConfigProperties map[string]string

// Bootstrap config for consul et. al.
var coreconfig = CoreConfig{}

func main() {

	var useConsul bool
	var useProfile string

	flag.BoolVar(&useConsul, "consul", false, "Indicates the service should use consul.")
	flag.BoolVar(&useConsul, "c", false, "Indicates the service should use consul.")
	flag.StringVar(&useProfile, "profile", "", "Specify a profile other than default.")
	flag.StringVar(&useProfile, "p", "", "Specify a profile other than default.")
	flag.Parse()

	//Read init boot strap config.  NOTE may not be needed in the future at all.

	// Configuration data for the config-seed service.
	coreconfig := &CoreConfig{}

	err := config.LoadFromFile(useProfile, coreconfig)
	if err != nil {
		logBeforeTermination(err)
		return
	}

	consulClient, err := getConsulCient(*coreconfig)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	kv := consulClient.KV()

	if coreconfig.IsReset {
		removeStoredConfig(kv)
		loadConfigFromPath(*coreconfig, kv)
	} else if !isConfigInitialized(*coreconfig, kv) {
		loadConfigFromPath(*coreconfig, kv)
	}

	printBanner("./res/banner.txt")
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

func logBeforeTermination(err error) {
	fmt.Println(err.Error())
}

// Get handle of Consul client using the URL from configuration info.
// Before getting handle, it tries to receive a response from a Consul agent by simple health-check.
func getConsulCient(coreconfig CoreConfig) (*consulapi.Client, error) {

	consulUrl := coreconfig.ConsulProtocol + "://" + coreconfig.ConsulHost + ":" + strconv.Itoa(coreconfig.ConsulPort)

	// Check the connection to Consul
	fails := 0
	for fails < coreconfig.FailLimit {
		resp, err := httpGet(consulUrl + CONSUL_STATUS_PATH)
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(time.Second * time.Duration(coreconfig.FailWaitTime))
			fails++
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
	}
	if fails >= coreconfig.FailLimit {
		return nil, errors.New("Cannot get connection to Consul")
	}

	// Connect to the Consul Agent
	configTemp := consulDefaultConfig()
	configTemp.Address = consulUrl

	return consulNewClient(configTemp)
}

// Remove all values in Consul K/V store, under the globalprefix which is presents in configuration file.
func removeStoredConfig(kv *consulapi.KV) {
	_, err := consulDeleteTree(kv, coreconfig.GlobalPrefix, nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("All values under the globalPrefix(\"" + coreconfig.GlobalPrefix + "\") is removed.")
}

// Check if Consul has been configured by trying to get any key that starts with a globalprefix.
func isConfigInitialized(coreconfig CoreConfig, kv *consulapi.KV) bool {
	keys, _, err := consulKeys(kv, coreconfig.GlobalPrefix, "", nil)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	if len(keys) > 0 {
		fmt.Printf("%s exists! The configuration data has been initialized.\n", coreconfig.GlobalPrefix)
		return true
	}
	fmt.Printf("%s doesn't exist! Start importing configuration data.\n", coreconfig.GlobalPrefix)
	return false
}

// Load a property file(.yaml or .properties) and parse it to a map.
func readPropertyFile(coreconfig CoreConfig, filePath string) (ConfigProperties, error) {

	// prob should not be here
	configuration := &pkg.ConfigurationStruct{}

	if isTomlExtension(coreconfig, filePath) {
		// Read .toml
		return readTomlFile(filePath, configuration)
	} else if isYamlExtension(coreconfig, filePath) {
		// Read .yaml/.yml file
		return readYamlFile(filePath)
	} else {
		// Read .properties file
		return readPropertiesFile(filePath)
	}

}

// Load all config files and put the configuration info to Consul K/V store.
func loadConfigFromPath(coreconfig CoreConfig, kv *consulapi.KV) {
	err := filepath.Walk(coreconfig.ConfigPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories & unacceptable property extension
		if info.IsDir() || !isAcceptablePropertyExtensions(coreconfig, info.Name()) {
			return nil
		}

		dir, file := filepath.Split(path)
		configPath, err := filepath.Rel(".", coreconfig.ConfigPath)
		if err != nil {
			return err
		}

		dir = strings.TrimPrefix(dir, configPath+"/")
		fmt.Println("found config file:", file, "in context", dir)

		// Parse *.properties
		props, err := readPropertyFile(coreconfig, path)
		if err != nil {
			return err
		}

		// Put config properties to Consul K/V store.
		prefix := coreconfig.GlobalPrefix + "/" + dir
		for k := range props {
			p := &consulapi.KVPair{Key: prefix + k, Value: []byte(props[k])}
			if _, err := consulPut(kv, p, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func isAcceptablePropertyExtensions(coreconfig CoreConfig, file string) bool {
	for _, v := range coreconfig.AcceptablePropertyExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

// Check whether a filename extension is yaml or not.
func isYamlExtension(coreconfig CoreConfig, file string) bool {
	for _, v := range coreconfig.YamlExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

func isTomlExtension(coreconfig CoreConfig, file string) bool {
	for _, v := range coreconfig.TomlExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

func readTomlFile(filePath string, configuration interface{}) (ConfigProperties, error) {

	configProps := ConfigProperties{}

	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return configProps, fmt.Errorf("could not load configuration file (%s): %v", filePath, err.Error())
	}

	// Decode the configuration from TOML
	err = toml.Unmarshal(contents, configuration)
	if err != nil {
		return configProps, fmt.Errorf("unable to parse configuration file (%s): %v", filePath, err.Error())
	}

	m := structs.Map(configuration)

	form := make(map[string]string)

	for k, v := range m {
		switch v := v.(type) {
		case string:
			form[k] = v
		case int, int8, int16, int32, int64:
			iInt := v.(int)
			iInt, ok := v.(int)
			if !ok {
				// issues in cast
			}
			form[k] = strconv.Itoa(iInt)
		case bool:
			form[k] = strconv.FormatBool(v)
		}

	}

	return form, nil
}

// Parse a yaml file to a map.
func readYamlFile(filePath string) (ConfigProperties, error) {

	configProps := ConfigProperties{}

	contents, err := ioutil.ReadFile(filePath)

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(contents, configProps); err != nil {
		return nil, err
	}

	m := structs.Map(configProps)

	for key, value := range m {
		m[key] = fmt.Sprint(value)
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
