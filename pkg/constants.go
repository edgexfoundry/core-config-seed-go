package pkg

type ConfigurationStruct struct {
	DBType                              string
	MongoDatabaseName                   string
	MongoDBUserName                     string
	MongoDBPassword                     string
	MongoDBHost                         string
	MongoDBPort                         int
	MongoDBConnectTimeout               int
	ReadMaxLimit                        int
	Protocol                            string
	ServiceAddress                      string
	ServicePort                         int
	ServiceTimeout                      int
	AppOpenMsg                          string
	CheckInterval                       string
	ConsulProfilesActive                string
	ConsulHost                          string
	ConsulCheckAddress                  string
	ConsulPort                          int
	EnableRemoteLogging                 bool
	LoggingFile                         string
	LoggingRemoteURL                    string
	NotificationPostDeviceChanges       bool
	NotificationsSlug                   string
	NotificationContent                 string
	NotificationSender                  string
	NotificationDescription             string
	NotificationLabel                   string
	SupportNotificationsHost            string
	SupportNotificationsPort            int
	SupportNotificationsSubscriptionURL string
	SupportNotificationsTransmissionURL string
}

// Configuration data for the metadata service
var Configuration  = ConfigurationStruct{} // Needs to be initialized before use

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

var CoreConfiguration  = CoreConfig{}    // Needs to be initialized before use

// Map to cover key/value.
type ConfigProperties map[string]string

