package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/sub2api/dfswitch/internal/adapter"
	"github.com/sub2api/dfswitch/internal/client"
	"github.com/sub2api/dfswitch/internal/store"
)

// statusActive matches backend/internal/domain/constants.go: Key.Status
// when the key is usable. Phase 0 curl confirmed the literal "active".
const statusActive = "active"

type SyncService struct {
	cfg *store.Config

	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	lastSync  time.Time
	lastError string
}

func NewSyncService(cfg *store.Config) *SyncService {
	s := &SyncService{cfg: cfg}
	if cfg.SyncEnabled {
		s.Start()
	}
	return s
}

func (s *SyncService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})

	go s.loop()
	log.Println("[sync] 后台同步已启动")
}

func (s *SyncService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
	log.Println("[sync] 后台同步已停止")
}

func (s *SyncService) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *SyncService) LastSync() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastSync.IsZero() {
		return ""
	}
	return s.lastSync.Format(time.RFC3339)
}

func (s *SyncService) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

func (s *SyncService) loop() {
	s.doSync()

	interval := time.Duration(s.cfg.SyncInterval) * time.Minute
	if interval < time.Minute {
		interval = 30 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.doSync()
		}
	}
}

func (s *SyncService) doSync() {
	if s.cfg.GetAccessToken() == "" {
		s.setError("未登录")
		return
	}
	if s.cfg.GetServerURL() == "" {
		s.setError("未配置服务器地址")
		return
	}

	applied := s.cfg.GetAppliedTools()
	if len(applied) == 0 {
		s.setSuccess()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := client.New(s.cfg)
	keys, err := cli.ListKeys(ctx)
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			s.setError("登录已过期，后台同步暂停")
			// Stop the ticker so we don't keep hammering refresh every
			// interval. The user will restart sync after logging in.
			go s.Stop()
		} else {
			s.setError("获取 Key 列表失败: " + err.Error())
		}
		return
	}
	if len(keys) == 0 {
		s.setError("账号下无可用 Key")
		return
	}

	adapters := adapter.AllAdapters()
	adapterMap := make(map[string]adapter.Adapter, len(adapters))
	for _, a := range adapters {
		adapterMap[a.ID()] = a
	}

	serverURL := s.cfg.GetServerURL()
	var firstErr string

	for toolID, entry := range applied {
		a, ok := adapterMap[toolID]
		if !ok {
			continue
		}

		// Replay the exact key previously applied. If it's missing or no
		// longer active, surface an error rather than silently swapping
		// to another key — users need to see their key has been revoked.
		var target *client.Key
		for i := range keys {
			if keys[i].Key == entry.KeyValue {
				target = &keys[i]
				break
			}
		}
		if target == nil {
			msg := toolID + ": 之前使用的 Key 已不存在，请重新选择"
			log.Printf("[sync] %s\n", msg)
			if firstErr == "" {
				firstErr = msg
			}
			continue
		}
		if target.Status != statusActive {
			msg := toolID + ": Key 已被禁用，请重新选择"
			log.Printf("[sync] %s (status=%s)\n", msg, target.Status)
			if firstErr == "" {
				firstErr = msg
			}
			continue
		}

		platform := ""
		if target.Group != nil {
			platform = target.Group.Platform
		}
		if !adapter.IsCompatible(a.SupportedPlatforms(), platform) {
			continue
		}

		baseURL := entry.BaseURL
		if baseURL == "" {
			baseURL = adapter.GatewayURL(serverURL, platform)
		}

		if err := a.Apply(adapter.ApplyRequest{APIKey: target.Key, BaseURL: baseURL, Platform: platform}); err != nil {
			log.Printf("[sync] 同步 %s 失败: %v\n", toolID, err)
			if firstErr == "" {
				firstErr = toolID + ": " + err.Error()
			}
			continue
		}
		s.cfg.SetAppliedTool(toolID, target.Key, baseURL)
		log.Printf("[sync] 同步 %s 成功\n", toolID)
	}
	_ = s.cfg.Save()

	if firstErr != "" {
		s.setError(firstErr)
	} else {
		s.setSuccess()
	}
}

func (s *SyncService) setError(msg string) {
	s.mu.Lock()
	s.lastSync = time.Now()
	s.lastError = msg
	s.mu.Unlock()
}

func (s *SyncService) setSuccess() {
	s.mu.Lock()
	s.lastSync = time.Now()
	s.lastError = ""
	s.mu.Unlock()
}
