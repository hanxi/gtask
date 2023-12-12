package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/hanxi/gtask/log"
)

type Config struct {
	StackBufLen int
	MsgQueueLen int
	CallTimeout int
	LogLevel    string
}

// 包级别的配置实例，拥有默认值
var C = &Config{
	StackBufLen: 4096, // 报错堆栈长度
	MsgQueueLen: 4096, // 消息接收队列长度
	CallTimeout: 1,    // Call超时（秒）
	LogLevel:    "debug",
}

func onConfigInit() {
	log.SetLevel(C.LogLevel)
}

func doLoad(configPath string) {
	file, fileErr := os.Open(configPath)
	if fileErr != nil {
		slog.Error("Error opening config file. Using default config", "file", fileErr, "C", C)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if decodeErr := decoder.Decode(C); decodeErr != nil {
		slog.Error("Error decoding config file. Using default config.", "file", decodeErr, "C", C)
	}
}

var once sync.Once

// Load 读取和解析配置文件，覆盖默认值
func Load(configPath string) error {
	var err error
	once.Do(func() {
		doLoad(configPath)
		onConfigInit()
	})
	return err
}
