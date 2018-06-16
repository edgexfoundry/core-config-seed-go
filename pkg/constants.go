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
