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
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
)

type tearDown func(t *testing.T)

var (
	originNewClient = consulNewClient
	originPut       = consulPut
	originKeys      = consulKeys
	originHttpGet   = httpGet

	jsonFile, propertiesFile, yamlFile *os.File
	kvMap                              map[string]string
)

func clearKvMap() {
	for k := range kvMap {
		delete(kvMap, k)
	}
}

func mockKvPut(kv *consulapi.KV, p *consulapi.KVPair, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error) {
	kvMap[p.Key] = string(p.Value)
	return nil, nil
}

func mockKvKeys(kv *consulapi.KV, prefix string, separator string, q *consulapi.QueryOptions) ([]string, *consulapi.QueryMeta, error) {
	if kv == nil {
		return nil, nil, errors.New("Invalid consul kv")
	}
	var values []string
	for k := range kvMap {
		if strings.HasPrefix(k, prefix) {
			values = append(values, kvMap[k])
		}
	}
	return values, nil, nil
}

func mockNewClient(config *consulapi.Config) (*consulapi.Client, error) {
	return &consulapi.Client{}, nil
}

func mockHttpGet(url string) (*http.Response, error) {
	if strings.Compare(url, "http://localhost:8500"+CONSUL_STATUS_PATH) == 0 {
		return &http.Response{StatusCode: http.StatusOK}, nil
	} else {
		return nil, errors.New("Connection refused")
	}
}

func setUp(t *testing.T) (tearDown, error) {
	originNewClient = consulNewClient
	originPut = consulPut
	originKeys = consulKeys
	originHttpGet = httpGet

	consulNewClient = mockNewClient
	consulPut = mockKvPut
	consulKeys = mockKvKeys
	httpGet = mockHttpGet

	kvMap = make(map[string]string)

	// initialize configuration
	configuration.ConfigPath = "./"
	configuration.GlobalPrefix = "config"
	configuration.ConsulProtocol = "http"
	configuration.ConsulHost = "localhost"
	configuration.ConsulPort = 8500
	configuration.IsReset = false
	configuration.FailLimit = 3
	configuration.FailWaittime = 0
	configuration.AcceptablePropertyExtensions = []string{".yaml", ".yml", ".properties"}
	configuration.YamlExtensions = []string{".yaml", ".yml"}

	// create test files
	var err error = nil
	jsonFile, err = os.Create("test.json")
	if err != nil {
		goto exit
	}
	_, err = jsonFile.Write([]byte("{\"key\":\"value\"}"))
	if err != nil {
		goto exit
	}

	propertiesFile, err = os.Create("test.properties")
	if err != nil {
		goto exit
	}
	_, err = propertiesFile.Write([]byte("key=value"))
	if err != nil {
		goto exit
	}

	yamlFile, err = os.Create("test.yaml")
	if err != nil {
		goto exit
	}
	_, err = yamlFile.Write([]byte("key: value"))
	if err != nil {
		goto exit
	}

exit:
	return func(t *testing.T) {
		if jsonFile != nil {
			os.Remove(jsonFile.Name())
		}
		if propertiesFile != nil {
			os.Remove(propertiesFile.Name())
		}
		if yamlFile != nil {
			os.Remove(yamlFile.Name())
		}

		consulNewClient = originNewClient
		consulPut = originPut
		consulKeys = originKeys
		httpGet = originHttpGet

		clearKvMap()
	}, err
}

func TestLoadConfigurationFile(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	var invalidJsonFile *os.File
	invalidJsonFile, err = os.Create("invalid.json")
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}
	_, err = invalidJsonFile.Write([]byte("{\"This is an invalid json\"}"))
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	testCases := []struct {
		name        string
		path        string
		expectedErr string
	}{
		{"Success", jsonFile.Name(), ""},
		{"ExpectErrorWithInvalidFilePath", "invalid.file", "open invalid.file: no such file or directory"},
		{"ExpectErrorWithInvalidJsonFile", invalidJsonFile.Name(), "invalid character '}' after object key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadConfigurationFile(tc.path)
			if err != nil && err.Error() != tc.expectedErr {
				t.Error("Expected error:" + tc.expectedErr + ", Actual error:" + err.Error())
			}
		})
	}

	if invalidJsonFile != nil {
		os.Remove(invalidJsonFile.Name())
	}
}

func TestIsAcceptablePropertyExtensions(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	testCases := []struct {
		name           string
		file           *os.File
		expectedRetVal bool
	}{
		{"ExpectReturnTrueWithPropertiesFile", propertiesFile, true},
		{"ExpectReturnFalseWithJsonFile", jsonFile, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if ret := isAcceptablePropertyExtensions(tc.file.Name()); ret != tc.expectedRetVal {
				t.Error("File:" + tc.file.Name() + ", Expected retval:" +
					strconv.FormatBool(tc.expectedRetVal) + ", Actual retval:" + strconv.FormatBool(ret))
			}
		})
	}
}

func TestIsYamlExtensions(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	testCases := []struct {
		name           string
		file           *os.File
		expectedRetVal bool
	}{
		{"ExpectReturnTrueWithYamlFile", yamlFile, true},
		{"ExpectReturnFalseWithPropertiesFile", propertiesFile, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if ret := isYamlExtension(tc.file.Name()); ret != tc.expectedRetVal {
				t.Error("File:" + tc.file.Name() + ", Expected retval:" +
					strconv.FormatBool(tc.expectedRetVal) + ", Actual retval:" + strconv.FormatBool(ret))
			}
		})
	}
}

func TestReadPropertiesFile(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	var invalidPropertiesFile, invalidYamlFile *os.File
	invalidPropertiesFile, err = os.Create("invalid.properties")
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}
	_, err = invalidPropertiesFile.Write([]byte("This is an invalid properties"))
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	invalidYamlFile, err = os.Create("invalid.yaml")
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}
	_, err = invalidYamlFile.Write([]byte("This is an invalid yaml"))
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	testCases := []struct {
		name          string
		filePath      string
		isValidFile   bool
		key           string
		isKeyValid    bool
		expectedValue string
	}{
		{"PropertiesFile_Success", propertiesFile.Name(), true, "key", true, "value"},
		{"PropertiesFile_ExpectFailToOpenFile", "NoExist.properties", false, "key", false, "value"},
		{"PropertiesFile_ExpectFailToParse", invalidPropertiesFile.Name(), false, "key", false, "value"},
		{"PropertiesFile_ExpectValueNotExistWithInvalidKey", propertiesFile.Name(), true, "Invalid_key", false, "value"},
		{"YamlFile_Success", yamlFile.Name(), true, "key", true, "value"},
		{"YamlFile_ExpectFailToOpenFile", "NoExist.yaml", false, "key", false, "value"},
		{"YamlFile_ExpectFailToParse", invalidYamlFile.Name(), false, "key", false, "value"},
		{"YamlFile_ExpectValueNotExistWithInvalidKey", yamlFile.Name(), true, "Invalid_key", false, "value"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			props, err := readPropertyFile(tc.filePath)
			if err != nil {
				if tc.isValidFile {
					t.Error("readPropertyFile failed : " + err.Error())
				}
				return
			}

			val, exist := props[tc.key]
			if exist && val != tc.expectedValue {
				t.Error("Expected value:" + tc.expectedValue + ", Actual:" + val)
			} else if !exist && tc.isKeyValid {
				t.Error("Key(" + tc.key + ") doesn't exist.")
			}
		})
	}

	if invalidPropertiesFile != nil {
		os.Remove(invalidPropertiesFile.Name())
	}

	if invalidYamlFile != nil {
		os.Remove(invalidYamlFile.Name())
	}
}

func TestIsConfigInitialized(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	if isConfigInitialized(nil) {
		t.Error("Config should not be initialized with invalid consul kv.")
	}

	consulClient, _ := getConsulCient()
	if consulClient == nil {
		t.Fatal("consulClient is nil.")
	}
	kv := consulClient.KV()

	if isConfigInitialized(kv) {
		t.Error("Config should not be initialized by default.")
	}

	loadConfigFromPath(kv)

	if !isConfigInitialized(kv) {
		t.Error("Config should be initialized after load")
	}

	clearKvMap()
}

func TestGetConsulCient(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	testCases := []struct {
		name              string
		consulHost        string
		failLimit         int
		isSuccessExpected bool
	}{
		{"Success", configuration.ConsulHost, configuration.FailLimit, true},
		{"ExpectFailWithInvalidConsulHost", "Invalid_consulHost", configuration.FailLimit, false},
		{"ExpectFailWhenFailLimitExceeded", configuration.ConsulHost, 0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configuration.ConsulHost = tc.consulHost
			configuration.FailLimit = tc.failLimit

			consulClient, _ := getConsulCient()
			if tc.isSuccessExpected && consulClient == nil {
				t.Error("Consul not obtained.")
			} else if !tc.isSuccessExpected && consulClient != nil {
				t.Error("Consul obtained when fail limit exceeded.")
			}
		})
	}
}

func TestLoadConfigFromPath(t *testing.T) {
	tearDown, err := setUp(t)
	defer tearDown(t)
	if err != nil {
		t.Fatal("setUp failed : " + err.Error())
	}

	consulClient, err := getConsulCient()
	if consulClient == nil || err != nil {
		t.Fatal("getConsulCient failed.")
	}
	kv := consulClient.KV()

	testCases := []struct {
		name              string
		configPath        string
		key               string
		isSuccessExpected bool
	}{
		{"Success", configuration.ConfigPath, configuration.GlobalPrefix + "/key", true},
		{"ExpectFailWithInvalidConfigPath", "./Invalid_path", configuration.GlobalPrefix + "/key", false},
		{"ExpectValueNotExistWithInvalidKey", configuration.ConfigPath, "/Invalid_key", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configuration.ConfigPath = tc.configPath

			loadConfigFromPath(kv)

			_, isPresent := kvMap[tc.key]

			if tc.isSuccessExpected && !isPresent {
				t.Error(tc.key + " is not present")
			} else if !tc.isSuccessExpected && isPresent {
				t.Error(tc.key + " is present")
			}

			clearKvMap()
		})
	}
}

func TestPrintBanner(t *testing.T) {
	testCases := []struct {
		name           string
		bannerFilePath string
	}{
		{"Success", "../../res/banner.txt"},
		{"ExpectFailWithInvalidFilePath", "./Invalid_path.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			printBanner(tc.bannerFilePath)
		})
	}
}
