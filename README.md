

core-config-seed-go
================================

This repository is for initializing the Configuration Management micro service.
It loads the default configuration from property or YAML files, and push values to the Consul Key/Value store.


## Prerequisites ##
- Remember, you must configure proxy for git accordingly if necessary.
  - Setting up proxy for git
```shell
$ git config --global http.proxy http://proxyuser:proxypwd@proxyserver.com:8080
```
- docker-ce
    - Version: 17.09
    - [How to install](https://docs.docker.com/engine/installation/linux/docker-ce/ubuntu/)

## How to build ##
This provides how to dockerize sources codes to create a Docker image.
```shell
$ docker build -t core-config-seed-go -f Dockerfile .
```

If it succeeds, you can see the built image as follows:
```shell
$ sudo docker images
REPOSITORY                   TAG        IMAGE ID        CREATED           SIZE
core-config-seed-go          latest     xxxxxxxxxxxx    XX seconds ago    XXX MB
```

## How to run ##
Required options to run Docker image
- port
    - 8400:8400
    - 8500:8500
    - 8600:8600

You can execute it with a Docker image as follows:
```shell
$ docker run -p 8400:8400 -p 8500:8500 -p 8600:8600 --name="edgex-core-config-seed-go" --hostname="edgex-core-config-seed-go" core-config-seed-go
```
After executed, you can use Consul Web UI(http://localhost:8500) for viewing services, nodes, health checks and their current status, and for reading and setting key/value data.

## Configuration Guidelines ##

The configuration of this tool is located in res/configuration.json.
There are several properties in it, and here are the default values and explanation:

    #The root path of the configuration files which would be loaded by this tool
    ConfigPath=./config

    #The global prefix namespace which will be created on the Consul Key/Value store
    GlobalPrefix=config

    #The communication protocol of the Consul server
    ConsulProtocol=http

    #The hostname of the Consul server
    ConsulHost=localhost

    #The communication port number of the Consul server
    ConsulPort=8500

    #If isReset=true, it will remove all the original values under the globalPrefix and import the configuration data
    #If isReset=false, it will check the globalPrefix exists or not, and it only imports configuration data when the globalPrefix doesn't exist.
    IsReset=false

    #The number for retry to connect to the Consul server when connection fails
    FailLimit=30

    #The seconds how long to wait for the next retry
    FailWaittime=3

## Configuration File Structure ##

In /config folder, there are some sample files for testing.<br>
The structure of the keys on the Consul server will be the same as the folders of the configPath, and the folder name must be the same as the microservice id registered on the Consul server.

For example, the files under /config/edgex-core-data folder will be loaded and create /{global_prefix}/edgex-core-data/{property_name} on the Consul server.
In addition, "edgex-core-data" is the micro service id of Core Data micro service.

However, you can use different profile name to categorize the usage on the same microservice. For instance,
"/config/edgex-core-data" contains the default configuration of Core Data Microservice.<br>
"/config/edgex-core-data,dev" contains the specific configuration for development time, and "dev" is the profile name.
"/config/edgex-core-data,test" contains the specific configuration for test time, and "test" is the profile name.
