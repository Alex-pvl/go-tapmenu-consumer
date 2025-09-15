package main

import (
	"flag"
	"github.com/sirupsen/logrus"

	"github.com/BurntSushi/toml"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/config"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/store"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/tapmenu-consumer"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/tapmenu-consumer/kafka"

	"log"
)

var (
	configPath string
)

func init() {
	flag.StringVar(&configPath, "config-path", "configs/tapmenu.toml", "path to config file")
}

func main() {
	flag.Parse()

	configuration := config.NewConfiguration()
	if _, err := toml.DecodeFile(configPath, configuration); err != nil {
		log.Fatal(err)
	}

	logger, err := configureLogger(configuration)
	if err != nil {
		log.Fatal(err)
	}

	db := store.New(configuration, logger)
	logger.Infof("connected to tarantool %s:***@%s", configuration.Username, configuration.TarantoolAddress)
	consumer := kafka.NewConsumer(configuration)
	logger.Infof("created Kafka consumer on %s; topic=%s; consumer-group=%s",
		configuration.KafkaAddress, configuration.TopicName, configuration.ConsumerGroup)
	server := tapmenu.New(configuration, db, consumer, logger)
	if err := server.Start(); err != nil {
		logger.Error(err)
	}
}

func configureLogger(configuration *config.Configuration) (*logrus.Logger, error) {
	level, err := logrus.ParseLevel(configuration.LogLevel)
	if err != nil {
		return nil, err
	}

	logger := logrus.New()
	logger.SetLevel(level)
	return logger, nil
}
