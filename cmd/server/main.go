package main

import (
	"context"
	"log"

	"domain-lite/internal/config"
	"domain-lite/internal/handler"
	"domain-lite/internal/router"
	"domain-lite/internal/store"
)

func main() {
	cfg := config.Load()
	db := store.Init(cfg.DBPath)
	store.SeedAdmin(db)
	r := router.New(cfg, db)

	// 后台定时同步 WHOIS 并发送临期提醒（不阻塞主服务）。
	domainH := handler.NewDomainHandler(cfg, db)
	go domainH.ScheduleSync(context.Background())

	addr := ":" + cfg.Port
	log.Printf("domain-lite listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
