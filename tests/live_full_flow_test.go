package tests

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/dashboard"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/fileagg"
	"pikpak-manager/internal/offline"
	"pikpak-manager/internal/scheduler"
	"pikpak-manager/internal/stream"
)

func setupLiveEnvironment(t *testing.T) (*sql.DB, *account.Service, *scheduler.AccountScheduler, *offline.Service, *fileagg.Service, *dashboard.Service, *stream.Handler) {
	tempDB := fmt.Sprintf("%s/live_test_%d.db", os.TempDir(), time.Now().UnixNano())
	cfg := &config.Config{
		DBPath:        tempDB,
		AppSecret:     "live-test-secret-32-characters!",
		AdminUsername: "admin",
		AdminPassword: "admin123456",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(tempDB)
		os.Remove(tempDB + "-wal")
		os.Remove(tempDB + "-shm")
	})

	accSvc := account.NewService(database, cfg.AppSecret)
	sched := scheduler.NewAccountScheduler(accSvc)
	offSvc := offline.NewService(database, accSvc, sched)
	fileSvc := fileagg.NewService(database, accSvc)
	dashSvc := dashboard.NewService(database)
	streamHandler := stream.NewHandler(fileSvc)

	return database, accSvc, sched, offSvc, fileSvc, dashSvc, streamHandler
}

func TestLiveFullSystemE2E(t *testing.T) {
	if os.Getenv("RUN_LIVE_TEST") == "" {
		t.Skip("Skipping live test. Set RUN_LIVE_TEST=1 to run.")
	}

	ctx := context.Background()
	_, accSvc, _, offSvc, fileSvc, dashSvc, streamHandler := setupLiveEnvironment(t)

	proxyURL := "socks5://127.0.0.1:10808"

	acc1User := os.Getenv("PIKPAK_ACC1_USER")
	if acc1User == "" {
		acc1User = "3541049@gmail.com"
	}
	acc1Pass := os.Getenv("PIKPAK_ACC1_PASS")
	if acc1Pass == "" {
		acc1Pass = "*rniq&CqBE5x8ew9"
	}
	acc2User := os.Getenv("PIKPAK_ACC2_USER")
	if acc2User == "" {
		acc2User = "8744102@gmail.com"
	}
	acc2Pass := os.Getenv("PIKPAK_ACC2_PASS")
	if acc2Pass == "" {
		acc2Pass = "hPap^s#XZ@N4KmAA"
	}

	// 1. 添加账号 1 与账号 2
	t.Log("===> Step 1: Adding both PikPak accounts with proxy...")
	acc1, err := accSvc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-1 (" + acc1User + ")",
		Username:  acc1User,
		Password:  acc1Pass,
		ProxyURL:  proxyURL,
		Priority:  20,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create account 1: %v", err)
	}

	acc2, err := accSvc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-2 (" + acc2User + ")",
		Username:  acc2User,
		Password:  acc2Pass,
		ProxyURL:  proxyURL,
		Priority:  10,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create account 2: %v", err)
	}

	// 2. 测试账号连通性与容量同步 (容量显示功能测试)
	t.Log("===> Step 2: Testing Account connectivity and Capacity sync...")
	testRes1, err := accSvc.TestAccount(ctx, acc1.ID)
	if err != nil || !testRes1.Success {
		t.Fatalf("Account 1 test failed: %v, error=%s", err, testRes1.Error)
	}
	t.Logf("Account 1 healthy! Used: %d bytes, Total: %d bytes, Latency: %d ms", testRes1.UsedSpace, testRes1.TotalSpace, testRes1.LatencyMs)

	testRes2, err := accSvc.TestAccount(ctx, acc2.ID)
	if err != nil || !testRes2.Success {
		t.Fatalf("Account 2 test failed: %v, error=%s", err, testRes2.Error)
	}
	t.Logf("Account 2 healthy! Used: %d bytes, Total: %d bytes, Latency: %d ms", testRes2.UsedSpace, testRes2.TotalSpace, testRes2.LatencyMs)

	// 验证 Dashboard 聚合容量统计
	stats, err := dashSvc.GetStats()
	if err != nil {
		t.Fatalf("GetDashboardStats failed: %v", err)
	}
	t.Logf("Dashboard Aggregated Stats: TotalAccounts=%d, HealthyAccounts=%d, UsedSpace=%d, TotalSpace=%d",
		stats.TotalAccounts, stats.HealthyAccounts, stats.UsedSpace, stats.TotalSpace)
	if stats.TotalAccounts != 2 || stats.HealthyAccounts != 2 {
		t.Errorf("Expected 2 healthy accounts, got total=%d healthy=%d", stats.TotalAccounts, stats.HealthyAccounts)
	}
	if stats.TotalSpace <= 0 || stats.UsedSpace <= 0 {
		t.Errorf("Expected positive storage stats, got used=%d total=%d", stats.UsedSpace, stats.TotalSpace)
	}

	// 3. 虚拟文件系统聚合与文件浏览测试
	t.Log("===> Step 3: Testing Unified Virtual File System...")
	rootFiles, err := fileSvc.ListFiles(ctx, "root", "name", "asc")
	if err != nil {
		t.Fatalf("Failed to list root files: %v", err)
	}
	t.Logf("Unified root contains %d files/folders:", len(rootFiles))
	var videoFile *fileagg.VirtualFile
	for _, f := range rootFiles {
		t.Logf("  * [%s] %s (Size: %d bytes, Account: %s, VirtualID: %s, IsVideo: %v)",
			f.Kind, f.Name, f.Size, f.AccountName, f.VirtualID, f.IsVideo)
		if f.IsVideo && strings.Contains(f.Name, "Tutorial") {
			v := f
			videoFile = &v
		}
	}

	// 4. 视频播放与 HTTP Range 206 串流测试
	t.Log("===> Step 4: Testing Video Playback & HTTP Range 206 Streaming...")
	if videoFile == nil {
		t.Fatal("Expected to find video file 'PikPak Tutorial.mkv' in root, but not found")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/media/info/:virtual_id", streamHandler.GetPlaybackInfo)
	router.GET("/api/media/stream/:virtual_id", streamHandler.ProxyStream)

	// 4.1 测试获取视频播放元信息
	infoReq, _ := http.NewRequest(http.MethodGet, "/api/media/info/"+videoFile.VirtualID, nil)
	infoW := httptest.NewRecorder()
	router.ServeHTTP(infoW, infoReq)
	if infoW.Code != http.StatusOK {
		t.Fatalf("GetPlaybackInfo returned HTTP %d: %s", infoW.Code, infoW.Body.String())
	}
	t.Logf("Playback Info response: %s", infoW.Body.String())

	// 4.2 测试 HTTP Range 206 局部内容串流 (bytes=0-1023)
	streamReq, _ := http.NewRequest(http.MethodGet, "/api/media/stream/"+videoFile.VirtualID, nil)
	streamReq.Header.Set("Range", "bytes=0-1023")
	streamW := httptest.NewRecorder()
	router.ServeHTTP(streamW, streamReq)

	if streamW.Code != http.StatusPartialContent {
		t.Fatalf("Expected HTTP 206 Partial Content, got HTTP %d. Body: %s", streamW.Code, streamW.Body.String())
	}
	contentRange := streamW.Header().Get("Content-Range")
	acceptRanges := streamW.Header().Get("Accept-Ranges")
	contentType := streamW.Header().Get("Content-Type")
	t.Logf("Range 206 Stream Headers: Content-Range=%s, Accept-Ranges=%s, Content-Type=%s",
		contentRange, acceptRanges, contentType)

	if !strings.HasPrefix(contentRange, "bytes 0-1023/") {
		t.Errorf("Unexpected Content-Range header: %s", contentRange)
	}
	if streamW.Body.Len() != 1024 {
		t.Errorf("Expected 1024 bytes body, got %d bytes", streamW.Body.Len())
	}
	t.Log("Video Range 206 chunk stream verified successfully!")

	// 5. 离线下载创建与调度测试
	t.Log("===> Step 5: Testing Offline Download submission on live PikPak...")
	// 使用快速且公开合法的测试资源 (Debian CD torrent)
	testURL := "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.7.0-amd64-netinst.iso.torrent"
	taskRes, err := offSvc.SubmitSingleLink(ctx, testURL, "debian-netinst-test.torrent", "")
	if err != nil {
		t.Fatalf("Failed to submit offline task: %v", err)
	}
	t.Logf("Offline task created successfully! Task ID: %s, Status: %s, Dispatched to: %s (Account ID %d)",
		taskRes.ID, taskRes.Status, taskRes.Account, taskRes.AccountID)

	// 验证在系统数据库中可查到该任务
	createdTask, err := offSvc.GetTask(ctx, taskRes.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	t.Logf("Task in DB: ID=%s, File=%s, Status=%s, PikPakTaskID=%s",
		createdTask.ID, createdTask.FileName, createdTask.Status, createdTask.PikPakTaskID)

	// 清理该测试离线任务
	_ = offSvc.DeleteTask(ctx, taskRes.ID)
	t.Log("Cleaned up test offline task.")

	// 6. 自动切换账号 (Auto Failover) 场景验证
	t.Log("===> Step 6: Testing Automatic Account Failover...")
	// 将账号 1 (当前高优先级 20) 标记为今日额度耗尽 QUOTA_EXHAUSTED
	_ = accSvc.UpdateStatus(acc1.ID, "QUOTA_EXHAUSTED", "task_daily_create_limit: test quota exhausted")
	t.Logf("Account 1 (Priority 20) marked as QUOTA_EXHAUSTED")

	// 提交新任务，验证调度器自动故障转移选择账号 2 (Priority 10)
	failoverTaskRes, err := offSvc.SubmitSingleLink(ctx, testURL, "failover-test.torrent", "")
	if err != nil {
		t.Fatalf("Failover task submission failed: %v", err)
	}
	t.Logf("Failover Task dispatched to: %s (Account ID %d)", failoverTaskRes.Account, failoverTaskRes.AccountID)

	if failoverTaskRes.AccountID != acc2.ID {
		t.Errorf("Expected task to failover to Account 2 (%d), but got account %d (%s)",
			acc2.ID, failoverTaskRes.AccountID, failoverTaskRes.Account)
	} else {
		t.Log("SUCCESS: Scheduler automatically failed over to Account 2!")
	}

	_ = offSvc.DeleteTask(ctx, failoverTaskRes.ID)

	// 恢复账号 1 额度
	_ = accSvc.ResetQuota(acc1.ID)
	t.Log("Reset Account 1 quota back to HEALTHY.")
	t.Log("===> ALL LIVE SYSTEM TESTS PASSED SUCCESSFULLY! <===")
}
