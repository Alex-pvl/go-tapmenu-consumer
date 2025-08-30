package main

import (
	"flag"

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

	db := store.New(configuration)
	consumer := kafka.NewConsumer(configuration)
	server := tapmenu.New(configuration, db, consumer)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
