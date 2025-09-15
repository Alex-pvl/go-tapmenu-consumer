package config

type Configuration struct {
	BindAddress    string `toml:"bind_address"`
	LogLevel       string `toml:"log_level"`
	FrontOriginUrl string `toml:"front_origin_url"`
	LocalOriginUrl string `toml:"local_origin_url"`
	// tarantool
	TarantoolAddress string `toml:"tarantool_address"`
	Username         string `toml:"username"`
	Password         string `toml:"password"`
	Timeout          uint   `toml:"timeout"`
	OrdersSpaceId    int    `toml:"orders_space_id"`
	WaitersSpaceId   int    `toml:"waiters_space_id"`
	// kafka
	KafkaAddress  string `toml:"kafka_address"`
	TopicName     string `toml:"topic_name"`
	ConsumerGroup string `toml:"consumer_group"`
}

func NewConfiguration() *Configuration {
	return &Configuration{
		BindAddress:      ":8080",
		LogLevel:         "debug",
		FrontOriginUrl:   "http://localhost:3000",
		LocalOriginUrl:   "http://localhost:3000",
		TarantoolAddress: ":3301",
		Username:         "username",
		Password:         "password",
		Timeout:          3,
		OrdersSpaceId:    513,
		WaitersSpaceId:   514,
		KafkaAddress:     ":9092",
		TopicName:        "topic-1",
		ConsumerGroup:    "cg-1",
	}
}
