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
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/edgexfoundry/core-config-seed-go/internal/pkg"
	"github.com/edgexfoundry/core-config-seed-go/internal/pkg/config"
	"github.com/fatih/structs"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/magiconair/properties"
	"gopkg.in/yaml.v2"
)

var Version = "master"

const consulStatusPath = "/v1/agent/self"

// Hook the functions in the other packages for the tests.
var (
	consulDefaultConfig = consulapi.DefaultConfig
	consulNewClient     = consulapi.NewClient
	consulDeleteTree    = (*consulapi.KV).DeleteTree
	consulPut           = (*consulapi.KV).Put
	consulKeys          = (*consulapi.KV).Keys
	httpGet             = http.Get
)

func main() {

	var useConsul bool
	var useProfile string

	flag.BoolVar(&useConsul, "consul", false, "Indicates the service should use consul.")
	flag.BoolVar(&useConsul, "c", false, "Indicates the service should use consul.")
	flag.StringVar(&useProfile, "profile", "", "Specify a profile other than default.")
	flag.StringVar(&useProfile, "p", "", "Specify a profile other than default.")
	flag.Parse()

	// Configuration data for the config-seed service.
	coreConfig := &pkg.CoreConfig{}

	err := config.LoadFromFile(useProfile, coreConfig)
	if err != nil {
		logBeforeTermination(err)
		return
	}

	consulClient, err := getConsulClient(*coreConfig)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	kv := consulClient.KV()

	if coreConfig.IsReset {
		removeStoredConfig(kv)
		loadConfigFromPath(*coreConfig, kv)
	} else if !isConfigInitialized(*coreConfig, kv) {
		loadConfigFromPath(*coreConfig, kv)
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
func getConsulClient(coreConfig pkg.CoreConfig) (*consulapi.Client, error) {

	consulUrl := coreConfig.ConsulProtocol + "://" + coreConfig.ConsulHost + ":" + strconv.Itoa(coreConfig.ConsulPort)

	// Check the connection to Consul
	fails := 0
	for fails < coreConfig.FailLimit {
		resp, err := httpGet(consulUrl + consulStatusPath)
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(time.Second * time.Duration(coreConfig.FailWaitTime))
			fails++
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}
	}
	if fails >= coreConfig.FailLimit {
		return nil, errors.New("Cannot get connection to Consul")
	}

	// Connect to the Consul Agent
	configTemp := consulDefaultConfig()
	configTemp.Address = consulUrl

	return consulNewClient(configTemp)
}

// Remove all values in Consul K/V store, under the globalprefix which is presents in configuration file.
func removeStoredConfig(kv *consulapi.KV) {
	_, err := consulDeleteTree(kv, pkg.CoreConfiguration.GlobalPrefix, nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("All values under the globalPrefix(\"" + pkg.CoreConfiguration.GlobalPrefix + "\") is removed.")
}

// Check if Consul has been configured by trying to get any key that starts with a globalprefix.
func isConfigInitialized(coreConfig pkg.CoreConfig, kv *consulapi.KV) bool {
	keys, _, err := consulKeys(kv, coreConfig.GlobalPrefix, "", nil)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	if len(keys) > 0 {
		fmt.Printf("%s exists! The configuration data has been initialized.\n", coreConfig.GlobalPrefix)
		return true
	}
	fmt.Printf("%s doesn't exist! Start importing configuration data.\n", coreConfig.GlobalPrefix)
	return false
}

// Load a property file(.yaml or .properties) and parse it to a map.
func readPropertyFile(coreConfig pkg.CoreConfig, filePath string) (pkg.ConfigProperties, error) {

	if isTomlExtension(coreConfig, filePath) {
		// Read .toml
		return readTomlFile(filePath)
	} else if isYamlExtension(coreConfig, filePath) {
		// Read .yaml/.yml file
		return readYamlFile(filePath)
	} else {
		// Read .properties file
		return readPropertiesFile(filePath)
	}

}

// Load all config files and put the configuration info to Consul K/V store.
func loadConfigFromPath(coreConfig pkg.CoreConfig, kv *consulapi.KV) {
	err := filepath.Walk(coreConfig.ConfigPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories & unacceptable property extension
		if info.IsDir() || !isAcceptablePropertyExtensions(coreConfig, info.Name()) {
			return nil
		}

		dir, file := filepath.Split(path)
		configPath, err := filepath.Rel(".", coreConfig.ConfigPath)
		if err != nil {
			return err
		}

		dir = strings.TrimPrefix(dir, configPath+"/")
		fmt.Println("found config file:", file, "in context", dir)

		// Parse *.properties
		props, err := readPropertyFile(coreConfig, path)
		if err != nil {
			return err
		}

		// Put config properties to Consul K/V store.
		prefix := coreConfig.GlobalPrefix + "/" + dir
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

func isAcceptablePropertyExtensions(coreConfig pkg.CoreConfig, file string) bool {
	for _, v := range coreConfig.AcceptablePropertyExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

// Check whether a filename extension is yaml or not.
func isYamlExtension(coreConfig pkg.CoreConfig, file string) bool {
	for _, v := range coreConfig.YamlExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}

func isTomlExtension(coreConfig pkg.CoreConfig, file string) bool {
	for _, v := range coreConfig.TomlExtensions {
		if v == filepath.Ext(file) {
			return true
		}
	}
	return false
}


//This works for now because our TOML is simply key/value.
//Will not work once we go hierarchical
func readTomlFile(filePath string) (pkg.ConfigProperties, error) {
	configProps := pkg.ConfigProperties{}

	file, err := os.Open(filePath)
	if err != nil {
		return configProps, fmt.Errorf("could not load configuration file (%s): %v", filePath, err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			tokens := strings.Split(scanner.Text(), "=")
			configProps[strings.Trim(tokens[0], " '")] = strings.Trim(tokens[1], " '")
		}
	}
	return configProps, nil
}

// Parse a yaml file to a map.
func readYamlFile(filePath string) (pkg.ConfigProperties, error) {

	configProps := pkg.ConfigProperties{}

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
func readPropertiesFile(filePath string) (pkg.ConfigProperties, error) {

	configProps := pkg.ConfigProperties{}

	props, err := properties.LoadFile(filePath, properties.UTF8)
	if err != nil {
		return nil, err
	}
	configProps = props.Map()

	return configProps, nil
}
