package handler

import (
	"net/http"
	"sync"
	"time"

	"domain-lite/internal/config"
	"domain-lite/internal/middleware"
	"domain-lite/internal/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler 处理登录/注册/用户信息。
type AuthHandler struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewAuthHandler(cfg *config.Config, db *gorm.DB) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 登录失败锁定：同一账号连续失败超过阈值后临时锁定，防暴力破解。
// 基于账号名（而非 IP）计数，避免反代后 IP 不准确导致的误判/绕过。
const (
	loginMaxFails   = 5
	loginLockWindow = 15 * time.Minute
)

var (
	loginFailsMu     sync.Mutex
	loginFailCounts  = make(map[string]int)
	loginLockedUntil = make(map[string]time.Time)
)

func loginCheckLocked(username string) (bool, time.Duration) {
	loginFailsMu.Lock()
	defer loginFailsMu.Unlock()
	if t, ok := loginLockedUntil[username]; ok {
		if time.Now().Before(t) {
			return true, t.Sub(time.Now())
		}
		delete(loginLockedUntil, username)
		delete(loginFailCounts, username)
	}
	return false, 0
}

func loginFail(username string) {
	loginFailsMu.Lock()
	defer loginFailsMu.Unlock()
	loginFailCounts[username]++
	if loginFailCounts[username] >= loginMaxFails {
		loginLockedUntil[username] = time.Now().Add(loginLockWindow)
		delete(loginFailCounts, username)
	}
}

func loginSuccess(username string) {
	loginFailsMu.Lock()
	defer loginFailsMu.Unlock()
	delete(loginFailCounts, username)
	delete(loginLockedUntil, username)
}

// Login 校验凭据并返回 JWT。
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	// 先检查是否处于锁定状态
	if locked, remain := loginCheckLocked(req.Username); locked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "too many failed login attempts, try again later: " + remain.Round(time.Minute).String(),
		})
		return
	}
	var user model.User
	err := h.db.Where("username = ?", req.Username).First(&user).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		// 用户不存在或密码错误都计入失败，避免账号枚举
		loginFail(req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid username or password"})
		return
	}
	loginSuccess(req.Username)
	token, err := middleware.GenerateToken(h.cfg, user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "token gen failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"token": token,
		"user":  gin.H{"id": user.ID, "username": user.Username, "role": user.Role},
	}})
}

// Register 注册新用户。
func (h *AuthHandler) Register(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid params"})
		return
	}
	var cnt int64
	h.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "username exists"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "hash failed"})
		return
	}
	user := model.User{Username: req.Username, PasswordHash: string(hash), Role: "admin"}
	h.db.Create(&user)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// Me 返回当前登录用户信息。
func (h *AuthHandler) Me(c *gin.Context) {
	uid, _ := c.Get("uid")
	var user model.User
	if err := h.db.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"id": user.ID, "username": user.Username, "role": user.Role,
	}})
}
