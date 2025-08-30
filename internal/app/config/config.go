package config

type Configuration struct {
	BindAddress string `toml:"bind_address"`
	LogLevel    string `toml:"log_level"`
	// tdb
	TarantooldbAddress string `toml:"tarantooldb_address"`
	Username           string `toml:"username"`
	Password           string `toml:"password"`
	Timeout            uint   `toml:"timeout"`
	// kafka
	KafkaAddress  string `toml:"kafka_address"`
	TopicName     string `toml:"topic_name"`
	ConsumerGroup string `toml:"consumer_group"`
}

func NewConfiguration() *Configuration {
	return &Configuration{
		BindAddress:        ":8080",
		LogLevel:           "debug",
		TarantooldbAddress: ":3301",
		Username:           "username",
		Password:           "password",
		Timeout:            3,
		KafkaAddress:       ":9092",
		TopicName:          "topic-1",
		ConsumerGroup:      "cg-1",
	}
}
