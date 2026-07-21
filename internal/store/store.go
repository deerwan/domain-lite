package store

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"domain-lite/internal/model"
)

// Init 打开（或创建）SQLite 数据库并执行自动迁移。
// 使用纯 Go 的 modernc/sqlite 驱动，无需 cgo，可静态编译进单二进制。
func Init(dbPath string) *gorm.DB {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create db dir: %v", err)
		}
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.DnsAccount{}, &model.Domain{}, &model.NotifySetting{}, &model.DnsRecordLog{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	return db
}

// SeedAdmin 在库中无用户时创建默认管理员（自用场景开箱即用）。
// 默认账号 admin / admin123，可通过环境变量 ADMIN_USERNAME / ADMIN_PASSWORD 覆盖。
func SeedAdmin(db *gorm.DB) {
	var cnt int64
	db.Model(&model.User{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	username := getenv("ADMIN_USERNAME", "admin")
	password := getenv("ADMIN_PASSWORD", "admin123")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := db.Create(&model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         "admin",
	}).Error; err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	log.Printf("seeded default admin user %q (set ADMIN_USERNAME/ADMIN_PASSWORD to override)", username)
}

// getenv 读取环境变量，缺失时返回默认值。
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
