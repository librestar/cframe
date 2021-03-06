package main

import (
	"flag"
	"fmt"

	"github.com/ICKelin/cframe/pkg/database"
	log "github.com/ICKelin/cframe/pkg/logs"
)

func main() {
	flgConf := flag.String("c", "", "config file")
	flag.Parse()
	conf, err := ParseConfig(*flgConf)
	if err != nil {
		fmt.Println(err)
		return
	}

	log.Init(conf.Log.Path, conf.Log.Level, conf.Log.Days)
	// initial mongodb url
	database.NewModelManager(conf.MongoUrl, conf.DBName)

	s := NewServer(conf.ListenAddr)
	fmt.Println(s.ListenAndServe())
}
