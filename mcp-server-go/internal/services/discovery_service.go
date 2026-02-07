package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ServerInfo 服务器信息（与 Rust HUD 格式对齐）
type ServerInfo struct {
	ID          string  `json:"id"`
	Port        int     `json:"port"`
	PID         int     `json:"pid"`
	ParentPID   *int    `json:"parent_pid"`
	ProjectPath string  `json:"project_path"`
	ProjectName string  `json:"project_name,omitempty"`
	Heartbeat   float64 `json:"heartbeat"`
	Started     string  `json:"started"`
}

// DiscoveryData 服务发现数据
type DiscoveryData struct {
	Servers []ServerInfo `json:"servers"`
}

// DiscoveryService 服务发现与心跳注册
type DiscoveryService struct {
	projectRoot string
	serverID    string
	startedAt   string
	stopCh      chan struct{}
	mu          sync.Mutex
}

// normalizePath 标准化路径格式（统一使用正斜杠）
func normalizePath(path string) string {
	// 将 Windows 反斜杠转换为正斜杠
	return filepath.ToSlash(path)
}

// NewDiscoveryService 创建服务发现实例
func NewDiscoveryService(projectRoot string) *DiscoveryService {
	// 生成唯一 Server ID (取时间戳后8位十六进制)
	id := fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	return &DiscoveryService{
		projectRoot: projectRoot,
		serverID:    id,
		startedAt:   time.Now().Format("2006-01-02 15:04:05"),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动心跳注册（后台 Goroutine）
func (d *DiscoveryService) Start() {
	go func() {
		// 首次立即注册
		d.register()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.register()
			case <-d.stopCh:
				d.unregister()
				return
			}
		}
	}()
}

// Stop 停止心跳并注销
func (d *DiscoveryService) Stop() {
	close(d.stopCh)
}

// register 注册/更新当前服务器信息
func (d *DiscoveryService) register() {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := getDiscoveryFilePath()
	if path == "" {
		return
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)

	// 读取现有数据
	var data DiscoveryData
	if content, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(content, &data)
	}

	// 清理失效服务器（心跳超时 或 PID 不存在 或 路径无效）并按 PID 去重
	now := float64(time.Now().Unix())
	myPID := os.Getpid()
	pidMap := make(map[int]ServerInfo) // PID -> 最新记录
	for _, s := range data.Servers {
		// 跳过自己（相同 PID 的旧条目，后面会重新添加）
		if s.PID == myPID {
			continue
		}

		// 条件1：心跳超时（30秒）
		if now-s.Heartbeat >= 30 {
			continue
		}

		// 条件2：PID 不存在（进程已死亡）
		if !isProcessAlive(s.PID) {
			continue
		}

		// 条件3：项目路径无效（路径不存在或为空）
		if s.ProjectPath == "" || s.ProjectPath == "Unknown" {
			continue
		}
		if _, err := os.Stat(s.ProjectPath); err != nil {
			continue // 路径不存在，跳过
		}

		// PID 去重：只保留心跳最新的记录
		if existing, ok := pidMap[s.PID]; ok {
			if s.Heartbeat > existing.Heartbeat {
				pidMap[s.PID] = s
			}
		} else {
			pidMap[s.PID] = s
		}
	}

	// 转换回切片
	var alive []ServerInfo
	for _, s := range pidMap {
		alive = append(alive, s)
	}

	// 添加/更新自己
	projectName := filepath.Base(d.projectRoot)
	// 标准化路径格式，确保 HUD 显示一致
	normalizedPath := normalizePath(d.projectRoot)
	self := ServerInfo{
		ID:          d.serverID,
		Port:        0, // StdIO 模式无端口
		PID:         os.Getpid(),
		ParentPID:   nil,
		ProjectPath: normalizedPath,
		ProjectName: projectName,
		Heartbeat:   float64(time.Now().UnixNano()) / 1e9,
		Started:     d.startedAt,
	}
	alive = append(alive, self)
	data.Servers = alive

	// 写回文件
	content, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile(path, content, 0644)
}

// unregister 注销当前服务器
func (d *DiscoveryService) unregister() {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := getDiscoveryFilePath()
	if path == "" {
		return
	}

	var data DiscoveryData
	if content, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(content, &data)
	}

	// 移除自己
	var remaining []ServerInfo
	for _, s := range data.Servers {
		if s.ID != d.serverID {
			remaining = append(remaining, s)
		}
	}
	data.Servers = remaining

	content, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile(path, content, 0644)
}

// getDiscoveryFilePath 获取服务发现文件路径
func getDiscoveryFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mcp-cockpit", "servers.json")
}

// isProcessAlive 检查 PID 是否存活（Windows 兼容）
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Windows: 使用 tasklist 命令验证
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// 如果输出包含 "INFO: No tasks" 或不包含 PID，则进程不存在
	return strings.Contains(string(output), fmt.Sprintf("%d", pid)) && !strings.Contains(string(output), "INFO:")
}

// CleanupServersJSON 清理 servers.json 中的死亡进程（启动时调用）
func CleanupServersJSON() {
	path := getDiscoveryFilePath()
	if path == "" {
		return
	}

	// 读取现有数据
	var data DiscoveryData
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(content, &data); err != nil {
		return
	}

	// 过滤：只保留存活的 PID，并按 PID 去重
	pidMap := make(map[int]ServerInfo)
	for _, s := range data.Servers {
		if !isProcessAlive(s.PID) {
			fmt.Fprintf(os.Stderr, "[MCP-Go] 清理失效服务器: %s (PID %d 已死亡)\n", s.ProjectName, s.PID)
			continue
		}
		// PID 去重：只保留最新的
		if existing, ok := pidMap[s.PID]; ok {
			if s.Heartbeat > existing.Heartbeat {
				pidMap[s.PID] = s
			}
		} else {
			pidMap[s.PID] = s
		}
	}

	// 转换回切片
	var alive []ServerInfo
	for _, s := range pidMap {
		alive = append(alive, s)
	}
	data.Servers = alive

	// 写回文件
	newContent, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile(path, newContent, 0644)
}
