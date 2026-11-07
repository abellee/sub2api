// mediaservicedev runs the real media administration APIs with a local SQLite database.
// It deliberately supports only local preview authentication and test callbacks.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type rejectBilling struct{}

func (rejectBilling) Settle(mediahub.SettleRequest) error {
	return fmt.Errorf("preview does not process real generation or billing")
}

func main() {
	listen := flag.String("listen", "127.0.0.1:18131", "loopback listen address")
	path := flag.String("settings", "data/media-service-preview/main.json", "local settings file")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		log.Fatal("preview must bind to a loopback IP")
	}
	defaults := handler.DefaultMediaHubSettings()
	db, err := sql.Open("sqlite", filepath.Join(filepath.Dir(*path), "download-records.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ddl, err := migrations.FS.ReadFile("250_media_generation_files.sql")
	if err != nil {
		log.Fatal(err)
	}
	if _, err = db.Exec(strings.ReplaceAll(strings.Split(string(ddl), "COMMENT ON")[0], "TIMESTAMPTZ", "TIMESTAMP")); err != nil {
		log.Fatal(err)
	}
	controller, err := handler.NewMediaHubController(*path, defaults, nil, mediahub.NewResultLinkStore(db))
	if err != nil {
		log.Fatal(err)
	}
	settings, err := mediahub.OpenSettings(*path, defaults)
	if err != nil {
		log.Fatal(err)
	}
	callback := mediahub.NewSettleServer(func() string { return settings.Active().CallbackSecret }, rejectBilling{}, time.Hour)
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	// Block browser requests from non-local pages. This preview never listens publicly.
	r.Use(func(c *gin.Context) {
		for _, raw := range []string{c.GetHeader("Origin"), c.GetHeader("Referer")} {
			if raw == "" {
				continue
			}
			u, e := url.Parse(raw)
			if e != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
				c.AbortWithStatus(403)
				return
			}
		}
		host := strings.Split(c.Request.Host, ":")[0]
		if host != "127.0.0.1" && host != "localhost" {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	})
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/setup/status", func(c *gin.Context) { response.Success(c, gin.H{"needs_setup": false, "step": "complete"}) })
	r.GET("/api/v1/settings/public", func(c *gin.Context) {
		response.Success(c, gin.H{"site_name": "TokenFree · 本地媒体测试", "site_subtitle": "Media Service Preview", "registration_enabled": false, "run_mode": "standard", "backend_mode_enabled": false, "setup_completed": true, "payment_enabled": false, "totp_enabled": false, "passkey_enabled": false})
	})
	r.POST("/internal/media/result", gin.WrapH(callback))
	r.POST("/internal/media/settle", gin.WrapH(callback))
	admin := r.Group("/api/v1/admin", func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer sub2api-local-admin-mock-token" {
			response.Error(c, 401, "Local preview login required")
			c.Abort()
			return
		}
		c.Next()
	})
	controller.RegisterAdmin(admin)
	// Optional surrounding shell UI; media configuration itself always uses the real handlers above.
	admin.GET("/user-menu", func(c *gin.Context) { response.Success(c, gin.H{"items": []string{}}) })
	r.NoRoute(func(c *gin.Context) { response.Error(c, 404, "This local preview only serves media configuration") })
	log.Printf("Local media administration preview: http://%s", *listen)
	srv := &http.Server{Addr: *listen, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
